package traces

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TraceService manages the lifecycle of execution traces, recording agent
// steps, tool calls, tokens, and cost into the execution_traces table.
type TraceService struct {
	pool *pgxpool.Pool
}

// NewTraceService creates a new TraceService backed by the given connection pool.
func NewTraceService(pool *pgxpool.Pool) *TraceService {
	return &TraceService{pool: pool}
}

// StartTrace creates a new execution trace in "running" state and returns it.
func (s *TraceService) StartTrace(ctx context.Context, taskID, agentID, issueID, provider, model string) (*ExecutionTrace, error) {
	id := uuid.New()

	row := s.pool.QueryRow(ctx, `
		INSERT INTO execution_traces (id, task_id, agent_id, issue_id, provider, model, status, start_time)
		VALUES ($1, $2, $3, $4, $5, $6, 'running', $7)
		RETURNING id, task_id, agent_id, issue_id, provider, model, steps, tools, tokens, cost, start_time, end_time, status, created_at, updated_at
	`,
		id,
		parseUUIDString(taskID),
		parseUUIDString(agentID),
		parseUUIDString(issueID),
		provider,
		model,
		time.Now(),
	)

	var dbRow ExecutionTraceDB
	if err := scanExecutionTrace(row, &dbRow); err != nil {
		return nil, fmt.Errorf("start trace: %w", err)
	}

	trace := dbToExecutionTrace(&dbRow)
	slog.Info("execution trace started", "trace_id", trace.ID, "task_id", trace.TaskID, "agent_id", trace.AgentID)
	return trace, nil
}

// RecordStep appends a step to the trace.
func (s *TraceService) RecordStep(ctx context.Context, traceID string, step TraceStep) error {
	stepJSON, err := json.Marshal(step)
	if err != nil {
		return fmt.Errorf("marshal step: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE execution_traces SET
		    steps = steps || $2::jsonb,
		    updated_at = NOW()
		WHERE id = $1
	`, parseUUIDString(traceID), stepJSON)
	if err != nil {
		return fmt.Errorf("record step: %w", err)
	}
	return nil
}

// RecordToolCall appends a tool call to the trace.
func (s *TraceService) RecordToolCall(ctx context.Context, traceID string, tc ToolCall) error {
	tcJSON, err := json.Marshal(tc)
	if err != nil {
		return fmt.Errorf("marshal tool call: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE execution_traces SET
		    tools = tools || $2::jsonb,
		    updated_at = NOW()
		WHERE id = $1
	`, parseUUIDString(traceID), tcJSON)
	if err != nil {
		return fmt.Errorf("record tool call: %w", err)
	}
	return nil
}

// UpdateTokens updates the token usage and cost for a trace.
func (s *TraceService) UpdateTokens(ctx context.Context, traceID string, tokens TokenUsage) error {
	tokensJSON, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("marshal tokens: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE execution_traces SET
		    tokens = $2,
		    cost = $3,
		    updated_at = NOW()
		WHERE id = $1
	`, parseUUIDString(traceID), tokensJSON, tokens.TotalCost)
	if err != nil {
		return fmt.Errorf("update tokens: %w", err)
	}
	return nil
}

// EndTrace marks a trace as completed, failed, or aborted.
func (s *TraceService) EndTrace(ctx context.Context, traceID, status string) error {
	row := s.pool.QueryRow(ctx, `
		UPDATE execution_traces SET
		    status = $2,
		    end_time = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'running'
		RETURNING id, status
	`, parseUUIDString(traceID), status)

	var id, newStatus string
	if err := row.Scan(&id, &newStatus); err != nil {
		return fmt.Errorf("end trace: %w", err)
	}

	slog.Info("execution trace ended", "trace_id", id, "status", newStatus)
	return nil
}

// GetTrace retrieves a single execution trace by ID, including all steps and tools.
func (s *TraceService) GetTrace(ctx context.Context, traceID string) (*ExecutionTrace, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, task_id, agent_id, issue_id, provider, model, steps, tools, tokens, cost, start_time, end_time, status, created_at, updated_at
		FROM execution_traces
		WHERE id = $1
	`, parseUUIDString(traceID))

	var dbRow ExecutionTraceDB
	if err := scanExecutionTrace(row, &dbRow); err != nil {
		return nil, fmt.Errorf("get trace: %w", err)
	}

	return dbToExecutionTrace(&dbRow), nil
}

// GetTraceByTask retrieves the most recent execution trace for a task.
func (s *TraceService) GetTraceByTask(ctx context.Context, taskID string) (*ExecutionTrace, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, task_id, agent_id, issue_id, provider, model, steps, tools, tokens, cost, start_time, end_time, status, created_at, updated_at
		FROM execution_traces
		WHERE task_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, parseUUIDString(taskID))

	var dbRow ExecutionTraceDB
	if err := scanExecutionTrace(row, &dbRow); err != nil {
		return nil, fmt.Errorf("get trace by task: %w", err)
	}

	return dbToExecutionTrace(&dbRow), nil
}

// ListTraces retrieves all execution traces for an issue, ordered by start time descending.
func (s *TraceService) ListTraces(ctx context.Context, issueID string) ([]*ExecutionTrace, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, task_id, agent_id, issue_id, provider, model, steps, tools, tokens, cost, start_time, end_time, status, created_at, updated_at
		FROM execution_traces
		WHERE issue_id = $1
		ORDER BY start_time DESC
	`, parseUUIDString(issueID))
	if err != nil {
		return nil, fmt.Errorf("list traces: %w", err)
	}
	defer rows.Close()

	var traces []*ExecutionTrace
	for rows.Next() {
		var dbRow ExecutionTraceDB
		if err := scanExecutionTraceRows(rows, &dbRow); err != nil {
			return nil, fmt.Errorf("scan trace row: %w", err)
		}
		traces = append(traces, dbToExecutionTrace(&dbRow))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return traces, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// pgxScanner abstracts pgx.Row and pgx.Rows for the shared scan logic.
type pgxScanner interface {
	Scan(dest ...any) error
}

// scanExecutionTrace scans a single pgx.Row into an ExecutionTraceDB.
func scanExecutionTrace(row pgx.Row, t *ExecutionTraceDB) error {
	return row.Scan(
		&t.ID,
		&t.TaskID,
		&t.AgentID,
		&t.IssueID,
		&t.Provider,
		&t.Model,
		&t.Steps,
		&t.Tools,
		&t.Tokens,
		&t.Cost,
		&t.StartTime,
		&t.EndTime,
		&t.Status,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
}

// scanExecutionTraceRows scans a row from a pgx.Rows iterator.
func scanExecutionTraceRows(rows pgx.Rows, t *ExecutionTraceDB) error {
	return rows.Scan(
		&t.ID,
		&t.TaskID,
		&t.AgentID,
		&t.IssueID,
		&t.Provider,
		&t.Model,
		&t.Steps,
		&t.Tools,
		&t.Tokens,
		&t.Cost,
		&t.StartTime,
		&t.EndTime,
		&t.Status,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
}

// dbToExecutionTrace converts a database row to an API ExecutionTrace.
func dbToExecutionTrace(db *ExecutionTraceDB) *ExecutionTrace {
	t := &ExecutionTrace{
		ID:       formatUUID(db.ID),
		TaskID:   formatUUID(db.TaskID),
		AgentID:  formatUUID(db.AgentID),
		IssueID:  formatUUID(db.IssueID),
		Provider: db.Provider,
		Model:    db.Model,
		Status:   db.Status,
	}

	if db.Steps != nil {
		json.Unmarshal(db.Steps, &t.Steps)
	}
	if t.Steps == nil {
		t.Steps = []TraceStep{}
	}

	if db.Tools != nil {
		json.Unmarshal(db.Tools, &t.Tools)
	}
	if t.Tools == nil {
		t.Tools = []ToolCall{}
	}

	if db.Tokens != nil {
		json.Unmarshal(db.Tokens, &t.Tokens)
	}

	if db.Cost.Valid {
		f, err := db.Cost.Float64Value()
		if err == nil {
			t.Cost = f.Float64
		}
	}

	if db.StartTime.Valid {
		t.StartTime = db.StartTime.Time
	}
	if db.EndTime.Valid {
		t.EndTime = db.EndTime.Time
	}

	return t
}

// formatUUID converts a pgtype.UUID to a standard UUID string representation.
func formatUUID(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	dst := make([]byte, 36)
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], b[10:16])
	return string(dst)
}

// parseUUIDString parses a UUID string into a pgtype.UUID.
func parseUUIDString(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{Valid: false}
	}
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{Valid: false}
	}
	return u
}
