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

// RestoreOnStartup loads any loops that were running/paused when the server
// last stopped, so the coordinator can re-subscribe to their state. Currently
// just logs the count; a follow-up task will re-enqueue stale stage tasks.
func (c *Coordinator) RestoreOnStartup(ctx context.Context) {
	loops, err := c.store.LoadActive(ctx)
	if err != nil {
		slog.Warn("loop coordinator: restore active loops failed", "err", err)
		return
	}
	slog.Info("loop coordinator: restored active loops", "count", len(loops))
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
