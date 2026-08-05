// Package loop implements the Agentic Engineering Loop: a state machine that
// drives an issue from open to mergeable PR by chaining Plan → Develop →
// Review → Fix stages implemented as agent_task_queue rows.
//
// The Coordinator in this file owns the state-machine decisions. Durable Run
// lifecycle facts enter through LifecycleProjector, which applies each
// decision and its next Work Item in one transaction. decideNextStage remains
// pure — no I/O or goroutines — so the policy is independently testable.
package loop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

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

var errInvalidLoopConfiguration = errors.New("invalid loop configuration")

type resolvedStageAgent struct {
	agentID   pgtype.UUID
	runtimeID pgtype.UUID
}

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

// Coordinator drives loops forward from durable lifecycle facts. The pure
// decideNextStage function holds the state machine; everything else is I/O.
type Coordinator struct {
	queries *dbpkg.Queries
	store   *Store
	starter lifecycleTxStarter
}

func NewCoordinator(q *dbpkg.Queries, starter lifecycleTxStarter) *Coordinator {
	return &Coordinator{queries: q, store: NewStore(q), starter: starter}
}

// StartLoop transitions a freshly created (status=pending, no stage) loop to
// status=running with current_stage=plan and enqueues the first loop_plan
// task. It is the production entry point that runs synchronously from the
// CreateLoop HTTP handler. Subsequent stages advance through the durable
// LifecycleProjector, while the plan stage has no preceding Run event.
//
// Policy: StartLoop is intentionally NOT idempotent. It refuses to re-start
// a loop that is not in 'pending' status, so callers cannot accidentally
// create a second loop_plan task for a loop that is already running.
//
// Errors are returned because StartLoop runs in the request goroutine and the
// handler must know whether the first task was enqueued.
func (c *Coordinator) StartLoop(ctx context.Context, loopID string) error {
	if c.starter == nil {
		return fmt.Errorf("start loop: transaction store is unavailable")
	}
	tx, err := c.starter.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin start loop: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := c.queries.WithTx(tx)
	row, err := q.GetLoopForUpdate(ctx, util.ParseUUID(loopID))
	if err != nil {
		return fmt.Errorf("lock loop: %w", err)
	}
	l, err := rowToLoop(row)
	if err != nil {
		return fmt.Errorf("decode loop: %w", err)
	}
	if l.Status != StatusPending {
		return fmt.Errorf("loop %s: StartLoop requires status=pending, got %q", l.ID, l.Status)
	}
	txCoordinator := &Coordinator{queries: q, store: NewStore(q), starter: c.starter}

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
	if err := txCoordinator.createTaskForStage(ctx, l, Decision{
		action:   actionCreateTask,
		taskType: taskTypePlan,
	}); err != nil {
		return fmt.Errorf("enqueue plan task: %w", err)
	}

	// Stamp started_at. The SQL has COALESCE(started_at, $started_at) so this
	// is a one-way set on the first transition to running.
	now := nowUTC()
	running := StatusRunning
	if _, err := txCoordinator.store.UpdateStatus(ctx, l.ID, UpdateStatusInput{
		Status:       &running,
		CurrentStage: &plan,
		StartedAt:    &now,
	}); err != nil {
		return fmt.Errorf("stamp started_at: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit start loop: %w", err)
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
	if l.Status == StatusPaused {
		// Paused loops are intentionally stopped; the operator will resume.
		return nil
	}
	if l.Status != StatusRunning {
		// LoadActive already filters to running/paused, but defend against
		// drift (e.g. a hook changed the status since the load).
		return nil
	}
	if l.CurrentStage == nil {
		return c.failInvalidConfiguration(ctx, l, "running loop has no current stage")
	}

	taskType := taskTypeForStage(*l.CurrentStage)
	if taskType == "" {
		return c.failInvalidConfiguration(ctx, l,
			fmt.Sprintf("running loop has unknown current stage %q", *l.CurrentStage))
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
	pendingTerminal, err := c.queries.HasPendingEngineeringLoopLifecycleEvent(ctx, dbpkg.HasPendingEngineeringLoopLifecycleEventParams{
		LoopID: util.ParseUUID(l.ID), TaskType: taskType,
	})
	if err != nil {
		return fmt.Errorf("check pending lifecycle event: %w", err)
	}
	if pendingTerminal {
		// The durable consumer will advance this stage. Re-enqueueing here would
		// duplicate work after a crash between Run completion and projection.
		return nil
	}

	resolvedAgent, err := c.resolveStageAgent(ctx, l, taskType)
	if err != nil {
		if errors.Is(err, errInvalidLoopConfiguration) {
			return c.failInvalidConfiguration(ctx, l, err.Error())
		}
		return fmt.Errorf("resolve stage agent: %w", err)
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
	if err := c.createTaskForStageWithAgent(ctx, l, Decision{
		action:   actionCreateTask,
		taskType: taskType,
	}, resolvedAgent); err != nil {
		return fmt.Errorf("re-enqueue stage task: %w", err)
	}
	slog.Info("loop coordinator: restored loop by re-enqueuing task",
		"loop_id", l.ID, "task_type", taskType)
	return nil
}

// applyDecision performs the I/O implied by a Decision.
func (c *Coordinator) applyDecision(ctx context.Context, l *Loop, d Decision) error {
	switch d.action {
	case actionCreateTask:
		if err := c.createTaskForStage(ctx, l, d); err != nil {
			if errors.Is(err, errInvalidLoopConfiguration) {
				return c.failInvalidConfiguration(ctx, l, err.Error())
			}
			return err
		}
		return nil
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
	resolvedAgent, err := c.resolveStageAgent(ctx, l, d.taskType)
	if err != nil {
		return fmt.Errorf("resolve stage agent: %w", err)
	}
	return c.createTaskForStageWithAgent(ctx, l, d, resolvedAgent)
}

func (c *Coordinator) resolveStageAgent(ctx context.Context, l *Loop, taskType string) (resolvedStageAgent, error) {
	stage := stageFromString(taskType)
	if stage == nil {
		return resolvedStageAgent{}, fmt.Errorf("%w: unknown task type %q", errInvalidLoopConfiguration, taskType)
	}

	configuredAgentID := l.StageAgent(*stage)
	if configuredAgentID == nil || strings.TrimSpace(*configuredAgentID) == "" {
		return resolvedStageAgent{}, fmt.Errorf("%w: no agent configured for stage %q", errInvalidLoopConfiguration, *stage)
	}

	agentID := util.ParseUUID(strings.TrimSpace(*configuredAgentID))
	if !agentID.Valid {
		return resolvedStageAgent{}, fmt.Errorf("%w: invalid agent id for stage %q", errInvalidLoopConfiguration, *stage)
	}

	agent, err := c.queries.GetAgent(ctx, agentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return resolvedStageAgent{}, fmt.Errorf("%w: agent for stage %q does not exist", errInvalidLoopConfiguration, *stage)
		}
		return resolvedStageAgent{}, fmt.Errorf("lookup agent for stage %q: %w", *stage, err)
	}
	if util.UUIDToString(agent.WorkspaceID) != l.WorkspaceID {
		return resolvedStageAgent{}, fmt.Errorf("%w: agent for stage %q belongs to another workspace", errInvalidLoopConfiguration, *stage)
	}
	if agent.ArchivedAt.Valid {
		return resolvedStageAgent{}, fmt.Errorf("%w: agent for stage %q is archived", errInvalidLoopConfiguration, *stage)
	}
	if !agent.RuntimeID.Valid {
		return resolvedStageAgent{}, fmt.Errorf("%w: agent for stage %q has no runtime", errInvalidLoopConfiguration, *stage)
	}

	return resolvedStageAgent{agentID: agentID, runtimeID: agent.RuntimeID}, nil
}

func (c *Coordinator) createTaskForStageWithAgent(ctx context.Context, l *Loop, d Decision, agent resolvedStageAgent) error {
	if _, err := c.queries.CreateAgentTask(ctx, dbpkg.CreateAgentTaskParams{
		AgentID:   agent.agentID,
		RuntimeID: agent.runtimeID,
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

func (c *Coordinator) failInvalidConfiguration(ctx context.Context, l *Loop, detail string) error {
	now := nowUTC()
	reason := string(FailureInvalidConfig)
	if _, err := c.store.UpdateStatus(ctx, l.ID, UpdateStatusInput{
		Status:        ptrStatus(StatusFailed),
		CurrentStage:  l.CurrentStage,
		Iteration:     &l.Iteration,
		FailureReason: &reason,
		CompletedAt:   &now,
	}); err != nil {
		return fmt.Errorf("mark invalid loop failed: %w", err)
	}
	slog.Warn("loop coordinator: failed loop with invalid configuration",
		"loop_id", l.ID, "detail", detail)
	return nil
}

func ptrStatus(s Status) *Status { return &s }
