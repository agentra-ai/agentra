package traces

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskRun struct {
	ID          string `json:"id"`
	TaskID      string `json:"task_id"`
	AgentID     string `json:"agent_id"`
	Status      string `json:"status"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at,omitempty"`
	DurationMs  int    `json:"duration_ms"`
	ExitCode    int    `json:"exit_code"`
	TotalSteps  int    `json:"total_steps"`
	TotalTokens int    `json:"total_tokens"`
	TotalCost   float64 `json:"total_cost"`
	Output      string `json:"output"`
	Error       string `json:"error,omitempty"`
	CreatedAt   string `json:"created_at"`
}

type TraceStep struct {
	ID          string         `json:"id"`
	TaskRunID   string         `json:"task_run_id"`
	StepNumber  int            `json:"step_number"`
	Timestamp   string         `json:"timestamp"`
	Action      string         `json:"action"`
	Tool        string         `json:"tool,omitempty"`
	InputText   string         `json:"input_text,omitempty"`
	OutputText  string         `json:"output_text,omitempty"`
	TokensUsed  int            `json:"tokens_used"`
	DurationMs  int            `json:"duration_ms"`
	Metadata    map[string]any `json:"metadata"`
}

type TaskRunSummary struct {
	TotalSteps  int            `json:"total_steps"`
	TotalTokens int            `json:"total_tokens"`
	TotalCost   float64       `json:"total_cost"`
	Duration    int64         `json:"duration_ms"`
	ToolUsage   map[string]int `json:"tool_usage"`
	KeyActions  []string      `json:"key_actions"`
}

type TraceRecorder struct {
	pool   *pgxpool.Pool
	taskID uuid.UUID
	steps  []TraceStep
	runID  uuid.UUID
	mu     sync.Mutex
}

func NewTraceRecorder(pool *pgxpool.Pool, taskID, runID uuid.UUID) *TraceRecorder {
	return &TraceRecorder{pool: pool, taskID: taskID, runID: runID, steps: []TraceStep{}}
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
