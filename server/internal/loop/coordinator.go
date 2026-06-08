// Package loop implements the Agentic Engineering Loop: a state machine that
// drives an issue from open to mergeable PR by chaining Plan → Develop →
// Review → Fix stages implemented as agent_task_queue rows.
//
// The Coordinator in this file is the brain: it watches task:completed events
// and advances loops through their stages. decideNextStage is a pure function
// — no I/O, no goroutines — so the state machine is trivially testable. The
// I/O lives in processTaskCompleted and applyDecision.
package loop

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agentra-ai/agentra/server/internal/events"
	"github.com/agentra-ai/agentra/server/internal/util"
	dbpkg "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

const (
	actionCreateTask = "create_task"
	actionComplete   = "complete"
	actionFail       = "fail"
	actionNoop       = "noop"

	taskTypePlan    = "loop_plan"
	taskTypeDevelop = "loop_develop"
	taskTypeReview  = "loop_review"
	taskTypeFix     = "loop_fix"
)

// loopRestoreTimeout is the maximum age of a running loop (measured from
// started_at) before RestoreOnStartup considers it abandoned and marks it
// failed. Paused loops are never timed out — pausing is an explicit operator
// action, not a stuck state.
const loopRestoreTimeout = 30 * time.Minute

// TaskResult is the parsed result of a completed agent task. Most fields
// are stage-specific: PR info comes from develop/review output, review
// verdict comes from review output.
type TaskResult struct {
	ReviewApproved *bool  `json:"review_approved,omitempty"`
	ReviewIssues   string `json:"review_issues,omitempty"`
	PRURL          string `json:"pr_url,omitempty"`
	PRNumber       *int   `json:"pr_number,omitempty"`
	BranchName     string `json:"branch_name,omitempty"`
}

// Decision is the output of decideNextStage. action drives the high-level
// branch in applyDecision; the remaining fields carry the data needed to
// execute that branch.
type Decision struct {
	action        string
	taskType      string
	prURL         string
	prNumber      int
	branchName    string
	reason        FailureReason
	iterationBump int
}

// Coordinator drives loops forward by reacting to task:completed events.
// The pure decideNextStage function holds the state machine; everything
// else is I/O.
type Coordinator struct {
	queries *dbpkg.Queries
	bus     *events.Bus
	store   *Store
}

func NewCoordinator(q *dbpkg.Queries, bus *events.Bus) *Coordinator {
	return &Coordinator{queries: q, bus: bus, store: NewStore(q)}
}

// StartLoop transitions a freshly created (status=pending, no stage) loop to
// status=running with current_stage=plan and enqueues the first loop_plan
// task. It is the production entry point that runs synchronously from the
// CreateLoop HTTP handler — the rest of the state machine is event-driven
// (HandleTaskCompleted / HandleTaskFailed), but the plan stage has no
// preceding task to fire on, so the handler must kick it off explicitly.
//
// Policy: StartLoop is intentionally NOT idempotent. It refuses to re-start
// a loop that is not in 'pending' status, so callers cannot accidentally
// create a second loop_plan task for a loop that is already running.
//
// Errors are returned to the caller. The Coordinator's own event handlers
// (HandleTaskCompleted / HandleTaskFailed) swallow errors and log them
// because they run on a goroutine; StartLoop runs in a request goroutine
// and the caller (the HTTP handler) wants to know if the first task
// failed to enqueue.
func (c *Coordinator) StartLoop(ctx context.Context, loopID string) error {
	l, err := c.store.GetLoop(ctx, loopID)
	if err != nil {
		return fmt.Errorf("load loop: %w", err)
	}
	if l.Status != StatusPending {
		return fmt.Errorf("loop %s: StartLoop requires status=pending, got %q", l.ID, l.Status)
	}

	// createTaskForStage handles CreateAgentTask (including the agent->runtime
	// lookup for the NOT NULL FK) and the UpdateStatus(Status: running,
	// CurrentStage: plan) in one go. It leaves started_at unset, so we
	// stamp it in a second UpdateStatus that echoes the values createTaskForStage
	// just wrote (status=running, current_stage=plan). UpdateStatus is a
	// full-row write on the state-machine columns, so we cannot pass nil
	// for either of those — the SQL would write an empty string for status
	// (CHECK violation) and NULL for current_stage (clobbering the stage
	// we just set).
	plan := StagePlan
	if err := c.createTaskForStage(ctx, l, Decision{
		action:   actionCreateTask,
		taskType: taskTypePlan,
	}); err != nil {
		return fmt.Errorf("enqueue plan task: %w", err)
	}

	// Stamp started_at. The SQL has COALESCE(started_at, $started_at) so this
	// is a one-way set on the first transition to running.
	now := nowUTC()
	running := StatusRunning
	if _, err := c.store.UpdateStatus(ctx, l.ID, UpdateStatusInput{
		Status:       &running,
		CurrentStage: &plan,
		StartedAt:    &now,
	}); err != nil {
		return fmt.Errorf("stamp started_at: %w", err)
	}

	slog.Info("loop coordinator: started loop",
		"loop_id", l.ID, "issue_id", l.IssueID, "workspace_id", l.WorkspaceID)
	return nil
}

// decideNextStage is a pure function. Given a loop's current state and the
// parsed result of the most recently completed task, it returns the next
// action the coordinator should take. No I/O, no goroutines, no globals.
func (c *Coordinator) decideNextStage(l *Loop, lastResult *TaskResult) Decision {
	switch l.Status {
	case StatusPaused, StatusCancelled, StatusDone, StatusFailed:
		return Decision{action: actionNoop}
	}
	if l.CurrentStage == nil {
		return Decision{action: actionFail, reason: FailureUnrecoverable}
	}

	switch *l.CurrentStage {
	case StagePlan:
		return Decision{action: actionCreateTask, taskType: taskTypeDevelop}
	case StageDevelop:
		return Decision{action: actionCreateTask, taskType: taskTypeReview}
	case StageReview:
		if lastResult != nil && lastResult.ReviewApproved != nil && *lastResult.ReviewApproved {
			d := Decision{action: actionComplete}
			d.prURL = lastResult.PRURL
			d.branchName = lastResult.BranchName
			if lastResult.PRNumber != nil {
				d.prNumber = *lastResult.PRNumber
			}
			return d
		}
		if l.Iteration >= l.MaxIterations {
			return Decision{action: actionFail, reason: FailureMaxIterations}
		}
		return Decision{action: actionCreateTask, taskType: taskTypeFix, iterationBump: 1}
	case StageFix:
		return Decision{action: actionCreateTask, taskType: taskTypeReview}
	}

	return Decision{action: actionNoop}
}

// HandleTaskCompleted is the events.Handler entry point. The bus publisher
// is synchronous, so this method returns immediately and the I/O happens on
// a goroutine. Failures inside the goroutine are logged (not returned).
func (c *Coordinator) HandleTaskCompleted(e events.Event) {
	go c.processTaskCompleted(context.Background(), e)
}

// HandleTaskFailed handles task:failed events. The bus publisher is
// synchronous, so this method returns immediately and the I/O happens on
// a goroutine. Failures inside the goroutine are logged (not returned).
func (c *Coordinator) HandleTaskFailed(e events.Event) {
	go c.processTaskFailed(context.Background(), e)
}

func (c *Coordinator) processTaskFailed(ctx context.Context, e events.Event) {
	taskID, ok := eventTaskID(e)
	if !ok {
		return
	}
	task, err := c.queries.GetAgentTask(ctx, util.ParseUUID(taskID))
	if err != nil {
		return
	}
	if !task.LoopID.Valid {
		return
	}
	l, err := c.store.GetLoop(ctx, util.UUIDToString(task.LoopID))
	if err != nil {
		return
	}
	if l.Status != StatusRunning {
		return
	}

	// Classify the error. The event payload may include an "error" string;
	// absent that, the default is unrecoverable so the operator sees a clear
	// "unclassified" signal rather than a misleading specific reason.
	reason := FailureUnrecoverable
	if msg, ok := e.Payload.(map[string]any); ok {
		if errMsg, ok := msg["error"].(string); ok && errMsg != "" {
			reason = classifyError(errMsg)
		}
	}

	now := nowUTC()
	failed := StatusFailed
	if _, err := c.store.UpdateStatus(ctx, l.ID, UpdateStatusInput{
		Status:        &failed,
		FailureReason: ptrString(string(reason)),
		CompletedAt:   &now,
	}); err != nil {
		slog.Error("loop coordinator: mark loop failed",
			"loop_id", l.ID, "task_id", task.ID, "err", err)
	}
}

// classifyError maps an error message from a failed task to a FailureReason.
// Conservative: when in doubt, return FailureUnrecoverable so the operator
// sees a clear "unclassified" signal rather than a misleading specific
// reason.
func classifyError(msg string) FailureReason {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "context"):
		return FailureContextExceeded
	case strings.Contains(lower, "filter"), strings.Contains(lower, "blocked"):
		return FailureContentFilter
	case strings.Contains(lower, "timeout"):
		return FailureStageTimeout
	case strings.Contains(lower, "pr"):
		return FailurePRCreateFailed
	}
	return FailureUnrecoverable
}

func ptrString(s string) *string { return &s }

// RestoreOnStartup re-arms loops that were running/paused when the server
// last stopped. For each one we either re-enqueue the current stage's task
// (when the in-flight task was lost mid-restart), mark the loop as
// failed/timeout (when the loop has been running too long with no progress),
// or skip it (when the loop is paused or a task is already in flight).
//
// Errors restoring a single loop are logged and skipped so one bad loop
// does not abort the whole restore. Paused loops are intentionally left
// alone — pausing is an explicit operator action, not a stuck state.
func (c *Coordinator) RestoreOnStartup(ctx context.Context) {
	loops, err := c.store.LoadActive(ctx)
	if err != nil {
		slog.Warn("loop coordinator: restore active loops failed", "err", err)
		return
	}
	slog.Info("loop coordinator: restoring active loops", "count", len(loops))

	for _, l := range loops {
		if err := c.restoreOne(ctx, l); err != nil {
			slog.Error("loop coordinator: restore loop failed",
				"loop_id", l.ID, "status", l.Status, "err", err)
		}
	}
}

// restoreOne applies the RestoreOnStartup policy to a single loop. Pulled
// out of the loop in RestoreOnStartup so per-loop error handling is uniform.
func (c *Coordinator) restoreOne(ctx context.Context, l *Loop) error {
	if l.CurrentStage == nil {
		slog.Warn("loop coordinator: skipping active loop with no current stage",
			"loop_id", l.ID, "status", l.Status)
		return nil
	}
	if l.Status == StatusPaused {
		// Paused loops are intentionally stopped; the operator will resume.
		return nil
	}
	if l.Status != StatusRunning {
		// LoadActive already filters to running/paused, but defend against
		// drift (e.g. a hook changed the status since the load).
		return nil
	}

	taskType := taskTypeForStage(*l.CurrentStage)
	if taskType == "" {
		slog.Warn("loop coordinator: unknown current stage, skipping",
			"loop_id", l.ID, "stage", *l.CurrentStage)
		return nil
	}

	has, err := c.queries.HasInFlightTaskForLoopStage(ctx, dbpkg.HasInFlightTaskForLoopStageParams{
		LoopID:   util.ParseUUID(l.ID),
		TaskType: taskType,
	})
	if err != nil {
		return fmt.Errorf("check in-flight task: %w", err)
	}
	if has {
		// A task is already in flight; the daemon will report its outcome
		// and the coordinator's normal handler will advance the loop.
		return nil
	}

	// No work in flight. If the loop has been running for too long with no
	// progress, declare it abandoned and move on. Paused loops are exempted
	// by the early-return above.
	if l.StartedAt != nil && time.Since(*l.StartedAt) > loopRestoreTimeout {
		now := nowUTC()
		reason := string(FailureLoopTimeout)
		if _, err := c.store.UpdateStatus(ctx, l.ID, UpdateStatusInput{
			Status:        ptrStatus(StatusFailed),
			FailureReason: &reason,
			CompletedAt:   &now,
		}); err != nil {
			return fmt.Errorf("mark loop timed out: %w", err)
		}
		slog.Info("loop coordinator: timed out loop on restore",
			"loop_id", l.ID, "started_at", l.StartedAt,
			"elapsed", time.Since(*l.StartedAt).Round(time.Second))
		return nil
	}

	// Re-enqueue the current stage's task. createTaskForStage also writes
	// the next stage back to the loop row, which is a no-op for a fresh
	// stage re-arm (it sets current_stage to the same value).
	if err := c.createTaskForStage(ctx, l, Decision{
		action:   actionCreateTask,
		taskType: taskType,
	}); err != nil {
		return fmt.Errorf("re-enqueue stage task: %w", err)
	}
	slog.Info("loop coordinator: restored loop by re-enqueuing task",
		"loop_id", l.ID, "task_type", taskType)
	return nil
}

func (c *Coordinator) processTaskCompleted(ctx context.Context, e events.Event) {
	taskID, ok := eventTaskID(e)
	if !ok {
		return
	}
	task, err := c.queries.GetAgentTask(ctx, util.ParseUUID(taskID))
	if err != nil {
		return
	}
	if !task.LoopID.Valid {
		return
	}
	loop, err := c.store.GetLoop(ctx, util.UUIDToString(task.LoopID))
	if err != nil {
		return
	}

	result := latestTaskResult(ctx, c.queries, task.ID)
	if err := c.applyDecision(ctx, loop, c.decideNextStage(loop, result)); err != nil {
		slog.Error("loop coordinator: apply decision failed",
			"loop_id", loop.ID,
			"task_id", task.ID,
			"err", err)
	}
}

// applyDecision performs the I/O implied by a Decision.
func (c *Coordinator) applyDecision(ctx context.Context, l *Loop, d Decision) error {
	switch d.action {
	case actionCreateTask:
		return c.createTaskForStage(ctx, l, d)
	case actionComplete:
		now := nowUTC()
		if _, err := c.store.SetPR(ctx, l.ID, d.prURL, d.prNumber, d.branchName); err != nil {
			return fmt.Errorf("set pr: %w", err)
		}
		if _, err := c.store.UpdateStatus(ctx, l.ID, UpdateStatusInput{
			Status:      ptrStatus(StatusDone),
			CompletedAt: &now,
		}); err != nil {
			return fmt.Errorf("update status to done: %w", err)
		}
		return nil
	case actionFail:
		now := nowUTC()
		reason := string(d.reason)
		if _, err := c.store.UpdateStatus(ctx, l.ID, UpdateStatusInput{
			Status:        ptrStatus(StatusFailed),
			FailureReason: &reason,
			CompletedAt:   &now,
		}); err != nil {
			return fmt.Errorf("update status to failed: %w", err)
		}
		return nil
	default:
		return nil
	}
}

func (c *Coordinator) createTaskForStage(ctx context.Context, l *Loop, d Decision) error {
	agentID := pgtype.UUID{}
	if l.AgentID != nil {
		agentID = util.ParseUUID(*l.AgentID)
	}
	// agent_task_queue.runtime_id is NOT NULL (added in migration 004). Look it
	// up from the agent row when we have one; otherwise the loop was created
	// without an agent and CreateAgentTask will fail with a NOT NULL violation.
	var runtimeID pgtype.UUID
	if l.AgentID != nil {
		agent, err := c.queries.GetAgent(ctx, agentID)
		if err != nil {
			return fmt.Errorf("lookup agent for runtime_id: %w", err)
		}
		runtimeID = agent.RuntimeID
	}
	if _, err := c.queries.CreateAgentTask(ctx, dbpkg.CreateAgentTaskParams{
		AgentID:   agentID,
		RuntimeID: runtimeID,
		IssueID:   util.ParseUUID(l.IssueID),
		Priority:  1,
		TaskType:  d.taskType,
		LoopID:    util.ParseUUID(l.ID),
	}); err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	next := stageFromString(d.taskType)
	newIter := l.Iteration + d.iterationBump
	if _, err := c.store.UpdateStatus(ctx, l.ID, UpdateStatusInput{
		Status:       ptrStatus(StatusRunning),
		CurrentStage: next,
		Iteration:    &newIter,
	}); err != nil {
		return fmt.Errorf("update loop stage: %w", err)
	}
	return nil
}

func ptrStatus(s Status) *Status { return &s }
