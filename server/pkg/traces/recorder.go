package traces

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TraceRecorder struct {
	pool   *pgxpool.Pool
	taskID uuid.UUID
	steps  []TraceStep
	runID  uuid.UUID
	mu     sync.Mutex
}

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

func (r *TraceRecorder) flush(ctx context.Context) error {
	// Implementation: batch write steps
	_, err := r.pool.Exec(ctx, `
		INSERT INTO trace_steps (task_run_id, step_number, timestamp, action, tool, input_text, output_text, tokens_used, duration_ms, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, r.runID, r.steps[0].StepNumber, r.steps[0].Timestamp, r.steps[0].Action, r.steps[0].Tool, r.steps[0].InputText, r.steps[0].OutputText, r.steps[0].TokensUsed, r.steps[0].DurationMs, r.steps[0].Metadata)
	r.steps = r.steps[:0]
	return err
}
