package traces

import (
	"context"
	"sync"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewTraceRecorder(pool *pgxpool.Pool, taskID, runID uuid.UUID) *TraceRecorder {
	return &TraceRecorder{
		pool:   pool,
		taskID: taskID,
		runID:  runID,
		steps:  []TraceStep{},
	}
}

func (r *TraceRecorder) RecordStep(ctx context.Context, step *TraceStep) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	step.TaskRunID = r.runID.String()
	r.steps = append(r.steps, *step)
	if len(r.steps) >= 10 {
		return r.flush(ctx)
	}
	return nil
}