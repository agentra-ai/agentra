package loop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

const (
	engineeringLoopPollInterval = 250 * time.Millisecond
	runCompletedEvent           = "run.completed"
	runFailedEvent              = "run.failed"
)

type lifecycleTxStarter interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// LifecycleProjector is the durable Engineering Loop Adapter. It owns an
// independent consumer receipt and applies the event, next Work Item, and Loop
// transition in one transaction, giving this Seam substantially more Depth
// than the former process-local Bus goroutine.
type LifecycleProjector struct {
	starter     lifecycleTxStarter
	queries     *db.Queries
	coordinator *Coordinator
}

func NewLifecycleProjector(starter lifecycleTxStarter, queries *db.Queries, coordinator *Coordinator) *LifecycleProjector {
	return &LifecycleProjector{starter: starter, queries: queries, coordinator: coordinator}
}

func (p *LifecycleProjector) Run(ctx context.Context) {
	if p == nil || p.starter == nil || p.queries == nil || p.coordinator == nil {
		return
	}
	p.Drain(ctx)
	ticker := time.NewTicker(engineeringLoopPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.Drain(ctx)
		}
	}
}

// Drain applies every currently available event. Startup calls this before
// RestoreOnStartup so a committed terminal event wins over stage re-arming.
func (p *LifecycleProjector) Drain(ctx context.Context) {
	for {
		processed, err := p.ProcessNext(ctx)
		if err != nil {
			slog.Warn("engineering loop lifecycle projection failed", "error", err)
			return
		}
		if !processed {
			return
		}
	}
}

// ProcessNext holds the outbox row lock until the Loop mutation and consumer
// receipt commit. A crash rolls all three back; concurrent workers use
// SKIP LOCKED and cannot create a duplicate stage Work Item.
func (p *LifecycleProjector) ProcessNext(ctx context.Context) (bool, error) {
	if p == nil || p.starter == nil || p.queries == nil || p.coordinator == nil {
		return false, fmt.Errorf("engineering loop projector is unavailable")
	}
	tx, err := p.starter.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin engineering loop projection: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	q := p.queries.WithTx(tx)
	event, err := q.ClaimEngineeringLoopLifecycleEvent(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim engineering loop event: %w", err)
	}
	if err := p.apply(ctx, q, event); err != nil {
		_ = tx.Rollback(ctx)
		failureErr := p.queries.RecordEngineeringLoopLifecycleFailure(ctx, db.RecordEngineeringLoopLifecycleFailureParams{
			EventID: event.ID, LastError: pgtype.Text{String: err.Error(), Valid: true},
		})
		if failureErr != nil {
			return true, fmt.Errorf("apply engineering loop event: %v; record failure: %w", err, failureErr)
		}
		return true, err
	}
	if err := q.RecordEngineeringLoopLifecycleReceipt(ctx, event.ID); err != nil {
		return true, fmt.Errorf("record engineering loop receipt: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("commit engineering loop projection: %w", err)
	}
	return true, nil
}

func (p *LifecycleProjector) apply(ctx context.Context, q *db.Queries, event db.LifecycleOutbox) error {
	if event.EventVersion != 1 {
		return fmt.Errorf("unsupported lifecycle event version %d", event.EventVersion)
	}
	if !event.RunID.Valid {
		return fmt.Errorf("terminal lifecycle event has no run id")
	}
	task, err := q.GetAgentTask(ctx, event.WorkItemID)
	if err != nil {
		return fmt.Errorf("load loop work item: %w", err)
	}
	if !task.LoopID.Valid {
		return nil
	}
	loopRow, err := q.GetLoopForUpdate(ctx, task.LoopID)
	if err != nil {
		return fmt.Errorf("lock engineering loop: %w", err)
	}
	l, err := rowToLoop(loopRow)
	if err != nil {
		return fmt.Errorf("decode engineering loop: %w", err)
	}
	if l.Status != StatusRunning || l.CurrentStage == nil || task.TaskType != taskTypeForStage(*l.CurrentStage) {
		// A prior event already advanced or terminally closed the Loop. Recording
		// this event's receipt is the idempotent no-op.
		return nil
	}
	run, err := q.GetTaskRun(ctx, event.RunID)
	if err != nil {
		return fmt.Errorf("load exact terminal run: %w", err)
	}
	if run.TaskID != task.ID {
		return fmt.Errorf("terminal run does not belong to work item")
	}

	txCoordinator := &Coordinator{queries: q, store: NewStore(q)}
	switch event.EventType {
	case runCompletedEvent:
		var result *TaskResult
		if run.Output.Valid {
			result = parseTaskResult([]byte(run.Output.String))
		}
		if err := txCoordinator.applyDecision(ctx, l, txCoordinator.decideNextStage(l, result)); err != nil {
			return fmt.Errorf("apply completed stage decision: %w", err)
		}
		return nil
	case runFailedEvent:
		reason := FailureUnrecoverable
		if run.Error.Valid && run.Error.String != "" {
			reason = classifyError(run.Error.String)
		}
		if err := txCoordinator.applyDecision(ctx, l, Decision{action: actionFail, reason: reason}); err != nil {
			return fmt.Errorf("apply failed stage decision: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported engineering loop event %q", event.EventType)
	}
}
