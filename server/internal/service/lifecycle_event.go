package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

const lifecycleEventVersion = 1

const (
	LifecycleEventRunStarted    = "run.started"
	LifecycleEventRunCheckpoint = "run.checkpointed"
	LifecycleEventRunCompleted  = "run.completed"
	LifecycleEventRunFailed     = "run.failed"
	LifecycleEventRunRetry      = "run.retry_scheduled"
	LifecycleEventRunCancelled  = "run.cancelled"
)

type lifecycleEventPayload struct {
	Status string `json:"status"`
}

// appendEvent adds the durable fact consumed by projection Adapters. Because
// callers pass transaction-scoped Queries, an append failure rolls back the
// authoritative Lifecycle Transition instead of creating a projection gap.
func (l *RunLifecycle) appendEvent(
	ctx context.Context,
	q *db.Queries,
	eventType string,
	task db.AgentTaskQueue,
	runID pgtype.UUID,
) error {
	payload, err := json.Marshal(lifecycleEventPayload{Status: task.Status})
	if err != nil {
		return fmt.Errorf("marshal lifecycle event: %w", err)
	}
	if _, err := q.AppendLifecycleOutboxEvent(ctx, db.AppendLifecycleOutboxEventParams{
		WorkItemID:   task.ID,
		RunID:        runID,
		EventType:    eventType,
		EventVersion: lifecycleEventVersion,
		Payload:      payload,
	}); err != nil {
		return fmt.Errorf("append %s lifecycle event: %w", eventType, err)
	}
	return nil
}
