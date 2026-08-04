package traces

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ExecutionTrace is a structured record of an agent task execution.
// It collects steps, tool calls, token usage, and cost into a single
// denormalized document that can be queried by task or issue.
type ExecutionTrace struct {
	ID        string      `json:"id"`
	RunID     string      `json:"run_id"`
	TaskID    string      `json:"task_id"`
	AgentID   string      `json:"agent_id"`
	IssueID   string      `json:"issue_id"`
	Provider  string      `json:"provider"`
	Model     string      `json:"model"`
	Steps     []TraceStep `json:"steps"`
	Tools     []ToolCall  `json:"tools"`
	Tokens    TokenUsage  `json:"tokens"`
	Cost      float64     `json:"cost"`
	StartTime time.Time   `json:"start_time"`
	EndTime   time.Time   `json:"end_time"`
	Status    string      `json:"status"` // running, completed, failed, aborted
}

// TraceStep is a single step in an execution trace (system, user, assistant, or tool).
type TraceStep struct {
	Step      int        `json:"step"`
	Type      string     `json:"type"` // system, user, assistant, tool
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Error     *string    `json:"error,omitempty"`
}

// ToolCall records a single tool invocation during agent execution.
type ToolCall struct {
	Tool       string         `json:"tool"`
	Input      map[string]any `json:"input"`
	Output     string         `json:"output"`
	DurationMs int64          `json:"duration_ms"`
}

// TokenUsage holds token consumption and cost for a trace.
type TokenUsage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalCost    float64 `json:"total_cost"`
}

// ExecutionTraceDB is the database row representation of an execution_trace.
type ExecutionTraceDB struct {
	ID        pgtype.UUID        `json:"id"`
	RunID     pgtype.UUID        `json:"run_id"`
	TaskID    pgtype.UUID        `json:"task_id"`
	AgentID   pgtype.UUID        `json:"agent_id"`
	IssueID   pgtype.UUID        `json:"issue_id"`
	Provider  string             `json:"provider"`
	Model     string             `json:"model"`
	Steps     []byte             `json:"steps"`
	Tools     []byte             `json:"tools"`
	Tokens    []byte             `json:"tokens"`
	Cost      pgtype.Numeric     `json:"cost"`
	StartTime pgtype.Timestamptz `json:"start_time"`
	EndTime   pgtype.Timestamptz `json:"end_time"`
	Status    string             `json:"status"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
}
