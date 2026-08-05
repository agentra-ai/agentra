package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// RunRef is the identity presented by a Runtime Adapter for an attempt-scoped
// callback. WorkItemID supplies locality for authorization and diagnostics;
// RunID prevents a delayed callback from mutating a newer attempt.
type RunRef struct {
	WorkItemID pgtype.UUID
	RunID      pgtype.UUID
}

// RunCompletion contains the authoritative data persisted when a Run and its
// Work Item complete. Projection-only effects (comments, metrics, realtime,
// and the denormalized execution Trace) are recovered from the durable event
// committed by this transition.
type RunCompletion struct {
	Result      []byte
	SessionID   string
	WorkDir     string
	DurationMs  int64
	TotalTokens int64
}

var (
	ErrStaleRun             = errors.New("run is not active for this work item")
	ErrLifecycleUnavailable = errors.New("run lifecycle store is unavailable")
)

type runLifecycleTxStarter interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// RunLifecycle is the unique authoritative write Module for attempt-scoped
// Work Item transitions. Its Interface hides row locking, dual-record state
// changes, active Run validation, and rollback from every Runtime Adapter.
type RunLifecycle struct {
	starter runLifecycleTxStarter
	queries *db.Queries
}

func NewRunLifecycle(starter runLifecycleTxStarter, queries *db.Queries) *RunLifecycle {
	return &RunLifecycle{starter: starter, queries: queries}
}

func (l *RunLifecycle) Start(ctx context.Context, ref RunRef) (db.AgentTaskQueue, error) {
	var task db.AgentTaskQueue
	err := l.withLockedRun(ctx, ref, []string{"dispatched"}, []string{"dispatched"}, func(q *db.Queries, lockedTask db.AgentTaskQueue, _ db.TaskRun) error {
		if _, err := q.SetTaskRunRunning(ctx, db.SetTaskRunRunningParams{
			ID:     ref.RunID,
			TaskID: ref.WorkItemID,
		}); err != nil {
			return fmt.Errorf("start run: %w", err)
		}
		updated, err := q.SetAgentTaskRunning(ctx, db.SetAgentTaskRunningParams{
			ID:          ref.WorkItemID,
			ActiveRunID: ref.RunID,
		})
		if err != nil {
			return fmt.Errorf("start work item: %w", err)
		}
		task = updated
		return l.appendEvent(ctx, q, LifecycleEventRunStarted, task, ref.RunID)
	})
	return task, err
}

func (l *RunLifecycle) AssertRunning(ctx context.Context, ref RunRef) (db.AgentTaskQueue, error) {
	if l == nil || l.queries == nil {
		return db.AgentTaskQueue{}, ErrLifecycleUnavailable
	}
	if !ref.WorkItemID.Valid || !ref.RunID.Valid {
		return db.AgentTaskQueue{}, ErrStaleRun
	}
	if _, err := l.queries.GetActiveTaskRun(ctx, db.GetActiveTaskRunParams{
		TaskID: ref.WorkItemID,
		ID:     ref.RunID,
	}); err != nil {
		return db.AgentTaskQueue{}, ErrStaleRun
	}
	task, err := l.queries.GetAgentTask(ctx, ref.WorkItemID)
	if err != nil || task.Status != "running" || task.ActiveRunID != ref.RunID {
		return db.AgentTaskQueue{}, ErrStaleRun
	}
	return task, nil
}

// Status returns the Work Item state only when ref is its active Run or the
// latest terminal Run. This lets a daemon observe cancellation of its own Run
// while preventing an older daemon from observing and continuing a newer Run.
func (l *RunLifecycle) Status(ctx context.Context, ref RunRef) (db.AgentTaskQueue, error) {
	if l == nil || l.queries == nil || !ref.WorkItemID.Valid || !ref.RunID.Valid {
		return db.AgentTaskQueue{}, ErrStaleRun
	}
	task, err := l.queries.GetAgentTask(ctx, ref.WorkItemID)
	if err != nil {
		return db.AgentTaskQueue{}, ErrStaleRun
	}
	if task.ActiveRunID.Valid {
		if task.ActiveRunID != ref.RunID {
			return db.AgentTaskQueue{}, ErrStaleRun
		}
		return task, nil
	}
	latest, err := l.queries.GetLatestTaskRun(ctx, ref.WorkItemID)
	if err != nil || latest.ID != ref.RunID {
		return db.AgentTaskQueue{}, ErrStaleRun
	}
	return task, nil
}

func (l *RunLifecycle) Checkpoint(ctx context.Context, ref RunRef, sessionID, workDir string) (db.AgentTaskQueue, error) {
	var task db.AgentTaskQueue
	err := l.withLockedRun(ctx, ref, []string{"running"}, []string{"running"}, func(q *db.Queries, _ db.AgentTaskQueue, _ db.TaskRun) error {
		session := pgtype.Text{String: sessionID, Valid: sessionID != ""}
		directory := pgtype.Text{String: workDir, Valid: workDir != ""}
		if _, err := q.CheckpointTaskRun(ctx, db.CheckpointTaskRunParams{
			ID:        ref.RunID,
			SessionID: session,
			WorkDir:   directory,
		}); err != nil {
			return fmt.Errorf("checkpoint run: %w", err)
		}
		updated, err := q.CheckpointAgentTaskSessionForRun(ctx, db.CheckpointAgentTaskSessionForRunParams{
			ID:          ref.WorkItemID,
			ActiveRunID: ref.RunID,
			SessionID:   session,
			WorkDir:     directory,
		})
		if err != nil {
			return fmt.Errorf("checkpoint work item: %w", err)
		}
		task = updated
		return l.appendEvent(ctx, q, LifecycleEventRunCheckpoint, task, ref.RunID)
	})
	return task, err
}

func (l *RunLifecycle) Complete(ctx context.Context, ref RunRef, completion RunCompletion) (db.AgentTaskQueue, error) {
	var task db.AgentTaskQueue
	err := l.withLockedRun(ctx, ref, []string{"running"}, []string{"running"}, func(q *db.Queries, _ db.AgentTaskQueue, _ db.TaskRun) error {
		session := pgtype.Text{String: completion.SessionID, Valid: completion.SessionID != ""}
		directory := pgtype.Text{String: completion.WorkDir, Valid: completion.WorkDir != ""}
		if _, err := q.CompleteTaskRun(ctx, db.CompleteTaskRunParams{
			ID:          ref.RunID,
			Status:      "completed",
			DurationMs:  pgtype.Int4{Int32: clampInt32(completion.DurationMs), Valid: true},
			ExitCode:    pgtype.Int4{Int32: 0, Valid: true},
			TotalSteps:  pgtype.Int4{Int32: 0, Valid: true},
			TotalTokens: pgtype.Int4{Int32: clampInt32(completion.TotalTokens), Valid: true},
			TotalCost:   pgtype.Numeric{},
			Output:      pgtype.Text{String: string(completion.Result), Valid: len(completion.Result) > 0},
			Error:       pgtype.Text{},
			SessionID:   session,
			WorkDir:     directory,
		}); err != nil {
			return fmt.Errorf("complete run: %w", err)
		}
		updated, err := q.CompleteAgentTaskForRun(ctx, db.CompleteAgentTaskForRunParams{
			ID:          ref.WorkItemID,
			ActiveRunID: ref.RunID,
			Result:      completion.Result,
			SessionID:   session,
			WorkDir:     directory,
		})
		if err != nil {
			return fmt.Errorf("complete work item: %w", err)
		}
		task = updated
		return l.appendEvent(ctx, q, LifecycleEventRunCompleted, task, ref.RunID)
	})
	return task, err
}

func (l *RunLifecycle) Fail(ctx context.Context, ref RunRef, errMsg string) (db.AgentTaskQueue, error) {
	var task db.AgentTaskQueue
	err := l.withLockedRun(ctx, ref, []string{"dispatched", "running"}, []string{"dispatched", "running"}, func(q *db.Queries, _ db.AgentTaskQueue, run db.TaskRun) error {
		duration := int64(0)
		if run.StartedAt.Valid {
			duration = time.Since(run.StartedAt.Time).Milliseconds()
		}
		if _, err := q.CompleteTaskRun(ctx, db.CompleteTaskRunParams{
			ID:          ref.RunID,
			Status:      "failed",
			DurationMs:  pgtype.Int4{Int32: clampInt32(duration), Valid: true},
			ExitCode:    pgtype.Int4{},
			TotalSteps:  pgtype.Int4{Int32: run.TotalSteps.Int32, Valid: run.TotalSteps.Valid},
			TotalTokens: pgtype.Int4{Int32: run.TotalTokens.Int32, Valid: run.TotalTokens.Valid},
			TotalCost:   run.TotalCost,
			Output:      run.Output,
			Error:       pgtype.Text{String: errMsg, Valid: errMsg != ""},
			SessionID:   run.SessionID,
			WorkDir:     run.WorkDir,
		}); err != nil {
			return fmt.Errorf("fail run: %w", err)
		}
		updated, err := q.FailAgentTaskForRun(ctx, db.FailAgentTaskForRunParams{
			ID:          ref.WorkItemID,
			ActiveRunID: ref.RunID,
			Error:       pgtype.Text{String: errMsg, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("fail work item: %w", err)
		}
		task = updated
		return l.appendEvent(ctx, q, LifecycleEventRunFailed, task, ref.RunID)
	})
	return task, err
}

// RetryActive closes the current dispatched/running Run before requeueing its
// Work Item. A new claim allocates a different Run identity.
func (l *RunLifecycle) RetryActive(ctx context.Context, ref RunRef, errMsg string) (db.AgentTaskQueue, bool, error) {
	var task db.AgentTaskQueue
	err := l.withLockedRun(ctx, ref, []string{"dispatched", "running"}, []string{"dispatched", "running"}, func(q *db.Queries, _ db.AgentTaskQueue, run db.TaskRun) error {
		if _, err := q.CompleteTaskRun(ctx, db.CompleteTaskRunParams{
			ID:          ref.RunID,
			Status:      "failed",
			DurationMs:  pgtype.Int4{},
			ExitCode:    pgtype.Int4{},
			TotalSteps:  run.TotalSteps,
			TotalTokens: run.TotalTokens,
			TotalCost:   run.TotalCost,
			Output:      run.Output,
			Error:       pgtype.Text{String: errMsg, Valid: errMsg != ""},
			SessionID:   run.SessionID,
			WorkDir:     run.WorkDir,
		}); err != nil {
			return fmt.Errorf("close run for retry: %w", err)
		}
		updated, err := q.RetryAgentTask(ctx, ref.WorkItemID)
		if err != nil {
			return fmt.Errorf("retry work item: %w", err)
		}
		task = updated
		return l.appendEvent(ctx, q, LifecycleEventRunRetry, task, ref.RunID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.AgentTaskQueue{}, false, nil
	}
	return task, err == nil, err
}

func (l *RunLifecycle) Cancel(ctx context.Context, workItemID pgtype.UUID) (db.AgentTaskQueue, pgtype.UUID, error) {
	if l == nil || l.starter == nil || l.queries == nil || !workItemID.Valid {
		return db.AgentTaskQueue{}, pgtype.UUID{}, ErrLifecycleUnavailable
	}
	tx, err := l.starter.Begin(ctx)
	if err != nil {
		return db.AgentTaskQueue{}, pgtype.UUID{}, fmt.Errorf("begin cancel transition: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := l.queries.WithTx(tx)
	locked, err := q.GetAgentTaskForUpdate(ctx, workItemID)
	if err != nil {
		return db.AgentTaskQueue{}, pgtype.UUID{}, err
	}
	runID := locked.ActiveRunID
	if runID.Valid {
		run, err := q.GetTaskRunForUpdate(ctx, runID)
		if err != nil || run.TaskID != workItemID || locked.Status != run.Status ||
			!containsStatus([]string{"dispatched", "running"}, run.Status) {
			return db.AgentTaskQueue{}, pgtype.UUID{}, ErrStaleRun
		}
		if _, err := q.CompleteTaskRun(ctx, db.CompleteTaskRunParams{
			ID:          run.ID,
			Status:      "cancelled",
			DurationMs:  pgtype.Int4{},
			ExitCode:    pgtype.Int4{},
			TotalSteps:  run.TotalSteps,
			TotalTokens: run.TotalTokens,
			TotalCost:   run.TotalCost,
			Output:      run.Output,
			Error:       run.Error,
			SessionID:   run.SessionID,
			WorkDir:     run.WorkDir,
		}); err != nil {
			return db.AgentTaskQueue{}, pgtype.UUID{}, fmt.Errorf("cancel run: %w", err)
		}
	} else if containsStatus([]string{"dispatched", "running"}, locked.Status) {
		return db.AgentTaskQueue{}, pgtype.UUID{}, ErrStaleRun
	}
	task, err := q.CancelAgentTask(ctx, workItemID)
	if err != nil {
		return db.AgentTaskQueue{}, pgtype.UUID{}, fmt.Errorf("cancel work item: %w", err)
	}
	if err := l.appendEvent(ctx, q, LifecycleEventRunCancelled, task, runID); err != nil {
		return db.AgentTaskQueue{}, pgtype.UUID{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.AgentTaskQueue{}, pgtype.UUID{}, fmt.Errorf("commit cancel transition: %w", err)
	}
	return task, runID, nil
}

// CancelForIssue atomically cancels every active Work Item for an Issue and
// appends one durable lifecycle fact per affected item. The query is one
// database statement, so callers never observe a partially emitted batch.
func (l *RunLifecycle) CancelForIssue(ctx context.Context, issueID pgtype.UUID) (int, error) {
	if l == nil || l.queries == nil || !issueID.Valid {
		return 0, ErrLifecycleUnavailable
	}
	events, err := l.queries.CancelAgentTasksByIssueLifecycle(ctx, issueID)
	if err != nil {
		return 0, fmt.Errorf("cancel issue work items: %w", err)
	}
	return len(events), nil
}

// CancelForAgent is the archive-time counterpart to CancelForIssue.
func (l *RunLifecycle) CancelForAgent(ctx context.Context, agentID pgtype.UUID) (int, error) {
	if l == nil || l.queries == nil || !agentID.Valid {
		return 0, ErrLifecycleUnavailable
	}
	events, err := l.queries.CancelAgentTasksByAgentLifecycle(ctx, agentID)
	if err != nil {
		return 0, fmt.Errorf("cancel agent work items: %w", err)
	}
	return len(events), nil
}

// FailStaleTasks closes timed-out active Runs, fails their Work Items, and
// emits the matching facts atomically for the background sweeper.
func (l *RunLifecycle) FailStaleTasks(ctx context.Context, dispatchTimeout, runningTimeout float64) (int, error) {
	if l == nil || l.queries == nil {
		return 0, ErrLifecycleUnavailable
	}
	events, err := l.queries.FailStaleTasksLifecycle(ctx, db.FailStaleTasksLifecycleParams{
		DispatchTimeoutSecs: dispatchTimeout,
		RunningTimeoutSecs:  runningTimeout,
	})
	if err != nil {
		return 0, fmt.Errorf("fail stale work items: %w", err)
	}
	return len(events), nil
}

// FailTasksForOfflineRuntimes closes every active Run owned by a runtime that
// the heartbeat sweeper has marked offline.
func (l *RunLifecycle) FailTasksForOfflineRuntimes(ctx context.Context) (int, error) {
	if l == nil || l.queries == nil {
		return 0, ErrLifecycleUnavailable
	}
	events, err := l.queries.FailTasksForOfflineRuntimesLifecycle(ctx)
	if err != nil {
		return 0, fmt.Errorf("fail offline runtime work items: %w", err)
	}
	return len(events), nil
}

// RecoverTasksForRuntime requeues retryable Work Items after a daemon restart
// and emits facts bound to the exact Run that was interrupted. A Work Item
// already failed by the offline sweep does not manufacture a second Run
// failure when its retry budget is exhausted.
func (l *RunLifecycle) RecoverTasksForRuntime(ctx context.Context, runtimeID pgtype.UUID) (requeued, failed int, err error) {
	if l == nil || l.queries == nil || !runtimeID.Valid {
		return 0, 0, ErrLifecycleUnavailable
	}
	rows, err := l.queries.RecoverTasksForRuntimeLifecycle(ctx, runtimeID)
	if err != nil {
		return 0, 0, fmt.Errorf("recover runtime work items: %w", err)
	}
	for _, row := range rows {
		switch row.Status {
		case "queued":
			requeued++
		case "failed":
			failed++
		}
	}
	return requeued, failed, nil
}

func (l *RunLifecycle) withLockedRun(
	ctx context.Context,
	ref RunRef,
	taskStatuses []string,
	runStatuses []string,
	apply func(*db.Queries, db.AgentTaskQueue, db.TaskRun) error,
) error {
	if l == nil || l.starter == nil || l.queries == nil {
		return ErrLifecycleUnavailable
	}
	if !ref.WorkItemID.Valid || !ref.RunID.Valid {
		return ErrStaleRun
	}
	tx, err := l.starter.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin lifecycle transition: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := l.queries.WithTx(tx)
	task, err := q.GetAgentTaskForUpdate(ctx, ref.WorkItemID)
	if err != nil {
		return ErrStaleRun
	}
	run, err := q.GetTaskRunForUpdate(ctx, ref.RunID)
	if err != nil || run.TaskID != ref.WorkItemID || task.ActiveRunID != ref.RunID ||
		task.Status != run.Status || !containsStatus(taskStatuses, task.Status) || !containsStatus(runStatuses, run.Status) {
		return ErrStaleRun
	}
	if err := apply(q, task, run); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit lifecycle transition: %w", err)
	}
	return nil
}

func containsStatus(statuses []string, status string) bool {
	for _, candidate := range statuses {
		if candidate == status {
			return true
		}
	}
	return false
}

func clampInt32(value int64) int32 {
	if value > int64(^uint32(0)>>1) {
		return int32(^uint32(0) >> 1)
	}
	if value < 0 {
		return 0
	}
	return int32(value)
}
