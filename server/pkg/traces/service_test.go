package traces

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestFormatUUID(t *testing.T) {
	// Test valid UUID
	var u pgtype.UUID
	if err := u.Scan("550e8400-e29b-41d4-a716-446655440000"); err != nil {
		t.Fatalf("failed to scan UUID: %v", err)
	}

	result := formatUUID(u)
	if result != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("expected valid UUID format, got %q", result)
	}

	// Test invalid UUID
	empty := formatUUID(pgtype.UUID{Valid: false})
	if empty != "" {
		t.Errorf("expected empty string for invalid UUID, got %q", empty)
	}
}

func TestParseUUIDString(t *testing.T) {
	// Valid UUID
	u := parseUUIDString("550e8400-e29b-41d4-a716-446655440000")
	if !u.Valid {
		t.Error("expected valid UUID")
	}

	// Empty string
	u = parseUUIDString("")
	if u.Valid {
		t.Error("expected invalid UUID for empty string")
	}

	// Invalid string
	u = parseUUIDString("not-a-uuid")
	if u.Valid {
		t.Error("expected invalid UUID for bad string")
	}
}

func TestDBToExecutionTrace(t *testing.T) {
	now := time.Now().UTC()

	db := &ExecutionTraceDB{
		ID:        mustParseUUID("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"),
		TaskID:    mustParseUUID("11111111-2222-4333-8444-555555555555"),
		AgentID:   mustParseUUID("22222222-3333-4444-8555-666666666666"),
		IssueID:   mustParseUUID("33333333-4444-5555-8666-777777777777"),
		Provider:  "claude",
		Model:     "claude-sonnet-4-20250514",
		Status:    "running",
		StartTime: pgtype.Timestamptz{Time: now, Valid: true},
	}

	trace := dbToExecutionTrace(db)
	if trace.ID != "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee" {
		t.Errorf("unexpected ID: %s", trace.ID)
	}
	if trace.Provider != "claude" {
		t.Errorf("unexpected provider: %s", trace.Provider)
	}
	if trace.Model != "claude-sonnet-4-20250514" {
		t.Errorf("unexpected model: %s", trace.Model)
	}
	if trace.Status != "running" {
		t.Errorf("unexpected status: %s", trace.Status)
	}
	if len(trace.Steps) != 0 {
		t.Errorf("expected empty steps, got %d", len(trace.Steps))
	}
	if len(trace.Tools) != 0 {
		t.Errorf("expected empty tools, got %d", len(trace.Tools))
	}
}

func TestDBToExecutionTraceWithData(t *testing.T) {
	db := &ExecutionTraceDB{
		ID:     mustParseUUID("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"),
		TaskID: mustParseUUID("11111111-2222-4333-8444-555555555555"),
		Status: "completed",
	}

	steps := []TraceStep{
		{Step: 1, Type: "assistant", Content: "I will fix the bug"},
		{Step: 2, Type: "tool", Content: "Running tests"},
	}
	stepsJSON, _ := json.Marshal(steps)
	db.Steps = stepsJSON

	tools := []ToolCall{
		{Tool: "read_file", Input: map[string]any{"path": "main.go"}, DurationMs: 150},
		{Tool: "run_tests", Output: "All tests passed", DurationMs: 3200},
	}
	toolsJSON, _ := json.Marshal(tools)
	db.Tools = toolsJSON

	tokens := TokenUsage{InputTokens: 1500, OutputTokens: 300, TotalCost: 0.05}
	tokensJSON, _ := json.Marshal(tokens)
	db.Tokens = tokensJSON

	trace := dbToExecutionTrace(db)

	if len(trace.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(trace.Steps))
	}
	if trace.Steps[0].Content != "I will fix the bug" {
		t.Errorf("unexpected step content: %s", trace.Steps[0].Content)
	}
	if len(trace.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(trace.Tools))
	}
	if trace.Tools[0].Tool != "read_file" {
		t.Errorf("unexpected tool name: %s", trace.Tools[0].Tool)
	}
	if trace.Tools[0].DurationMs != 150 {
		t.Errorf("unexpected tool duration: %d", trace.Tools[0].DurationMs)
	}
	if trace.Tokens.InputTokens != 1500 {
		t.Errorf("unexpected input tokens: %d", trace.Tokens.InputTokens)
	}
	if trace.Tokens.TotalCost != 0.05 {
		t.Errorf("unexpected cost: %f", trace.Tokens.TotalCost)
	}
}

func TestTraceStepJSON(t *testing.T) {
	errStr := "something went wrong"
	step := TraceStep{
		Step:    3,
		Type:    "tool",
		Content: "executing command",
		ToolCalls: []ToolCall{
			{Tool: "bash", Input: map[string]any{"command": "ls"}, DurationMs: 100},
		},
		Error: &errStr,
	}

	data, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded TraceStep
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Step != 3 {
		t.Errorf("unexpected step: %d", decoded.Step)
	}
	if decoded.Type != "tool" {
		t.Errorf("unexpected type: %s", decoded.Type)
	}
	if decoded.Error == nil || *decoded.Error != "something went wrong" {
		t.Errorf("unexpected error: %v", decoded.Error)
	}
	if len(decoded.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(decoded.ToolCalls))
	}
}

func TestTokenUsageJSON(t *testing.T) {
	tu := TokenUsage{
		InputTokens:  4096,
		OutputTokens: 1024,
		TotalCost:    0.15,
	}

	data, err := json.Marshal(tu)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded TokenUsage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.InputTokens != 4096 {
		t.Errorf("unexpected input tokens: %d", decoded.InputTokens)
	}
	if decoded.TotalCost != 0.15 {
		t.Errorf("unexpected cost: %f", decoded.TotalCost)
	}
}

func TestToolCallJSON(t *testing.T) {
	tc := ToolCall{
		Tool:       "write_file",
		Input:      map[string]any{"path": "/tmp/test.go", "content": "package main"},
		Output:     "file written successfully",
		DurationMs: 250,
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ToolCall
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Tool != "write_file" {
		t.Errorf("unexpected tool: %s", decoded.Tool)
	}
	if decoded.DurationMs != 250 {
		t.Errorf("unexpected duration: %d", decoded.DurationMs)
	}
}

func TestExecutionTraceJSONRoundtrip(t *testing.T) {
	original := &ExecutionTrace{
		ID:       "trace-001",
		TaskID:   "task-001",
		AgentID:  "agent-001",
		IssueID:  "issue-001",
		Provider: "claude",
		Model:    "claude-sonnet-4-20250514",
		Steps: []TraceStep{
			{Step: 1, Type: "assistant", Content: "Analyzing the issue..."},
		},
		Tools: []ToolCall{
			{Tool: "read_file", Input: map[string]any{"path": "main.go"}, DurationMs: 100},
		},
		Tokens:    TokenUsage{InputTokens: 500, OutputTokens: 200, TotalCost: 0.02},
		Cost:      0.02,
		StartTime: time.Now().UTC().Truncate(time.Second),
		Status:    "running",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ExecutionTrace
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: %s vs %s", decoded.ID, original.ID)
	}
	if decoded.Provider != original.Provider {
		t.Errorf("provider mismatch: %s vs %s", decoded.Provider, original.Provider)
	}
	if len(decoded.Steps) != len(original.Steps) {
		t.Errorf("steps count mismatch: %d vs %d", len(decoded.Steps), len(original.Steps))
	}
	if len(decoded.Tools) != len(original.Tools) {
		t.Errorf("tools count mismatch: %d vs %d", len(decoded.Tools), len(original.Tools))
	}
	if decoded.Tokens.TotalCost != original.Tokens.TotalCost {
		t.Errorf("cost mismatch: %f vs %f", decoded.Tokens.TotalCost, original.Tokens.TotalCost)
	}
}

func mustParseUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		panic(err)
	}
	return u
}
