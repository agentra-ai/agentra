package traces

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
	DurationMs  int64         `json:"duration_ms"`
	ToolUsage   map[string]int `json:"tool_usage"`
	KeyActions  []string      `json:"key_actions"`
}
