package agent

import (
	"strings"
	"testing"
)

// ── Claude ──

func TestBuildClaudeArgsNoToolsByDefault(t *testing.T) {
	t.Parallel()

	opts := ExecOptions{
		Cwd:   "/tmp/work",
		Model: "claude-sonnet-4-5",
	}
	args := buildClaudeArgs(opts, "/usr/bin/claude")

	for _, a := range args {
		if a == "--allowedTools" {
			t.Fatalf("expected no --allowedTools flag when Tools is empty, got args=%v", args)
		}
	}
}

func TestBuildClaudeArgsWithTools(t *testing.T) {
	t.Parallel()

	opts := ExecOptions{
		Cwd:   "/tmp/work",
		Model: "claude-sonnet-4-5",
		Tools: []string{"Read", "Glob", "Grep"},
	}
	args := buildClaudeArgs(opts, "/usr/bin/claude")

	// Find the --allowedTools flag and verify its value is comma-joined.
	found := false
	for i, a := range args {
		if a == "--allowedTools" {
			found = true
			if i+1 >= len(args) {
				t.Fatalf("--allowedTools present but no value, args=%v", args)
			}
			if got := args[i+1]; got != "Read,Glob,Grep" {
				t.Fatalf("expected --allowedTools Read,Glob,Grep, got %q", got)
			}
		}
	}
	if !found {
		t.Fatalf("expected --allowedTools flag, got args=%v", args)
	}
}

func TestBuildClaudeArgsWithSingleTool(t *testing.T) {
	t.Parallel()

	opts := ExecOptions{
		Tools: []string{"read_file"},
	}
	args := buildClaudeArgs(opts, "/usr/bin/claude")

	for i, a := range args {
		if a == "--allowedTools" {
			if i+1 >= len(args) {
				t.Fatalf("--allowedTools present but no value")
			}
			if got := args[i+1]; got != "read_file" {
				t.Fatalf("expected --allowedTools read_file, got %q", got)
			}
			return
		}
	}
	t.Fatalf("expected --allowedTools flag, got args=%v", args)
}

func TestBuildClaudeArgsPreservesOtherFlags(t *testing.T) {
	t.Parallel()

	opts := ExecOptions{
		Model:        "claude-sonnet-4-5",
		SystemPrompt: "you are a planner",
		MaxTurns:     5,
		Tools:        []string{"Read"},
	}
	args := buildClaudeArgs(opts, "/usr/bin/claude")

	joined := strings.Join(args, " ")
	for _, want := range []string{"--model claude-sonnet-4-5", "--max-turns 5", "--allowedTools Read", "-p"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected args to contain %q, got %q", want, joined)
		}
	}
}

// ── Codex ──

func TestBuildCodexTurnParamsNoToolsByDefault(t *testing.T) {
	t.Parallel()

	opts := ExecOptions{
		Cwd:   "/tmp/work",
		Model: "gpt-5",
	}
	params := buildCodexTurnParams(opts, "thread-123", "do the thing")

	// If a "tools" key exists in the params at all, it should be nil or absent
	// when opts.Tools is empty.
	if v, ok := params["tools"]; ok && v != nil {
		t.Fatalf("expected no tools restriction, got params[\"tools\"]=%v", v)
	}
}

func TestBuildCodexTurnParamsToolsUnsupported(t *testing.T) {
	t.Parallel()

	// Codex's JSON-RPC API does not currently expose a per-turn tool
	// restriction field. The plumbing is in place — when codex supports it,
	// the only change needed is inside buildCodexTurnParams. For now the
	// helper ignores opts.Tools and does not add any flag/param.
	opts := ExecOptions{
		Tools: []string{"read_file", "search_code"},
	}
	params := buildCodexTurnParams(opts, "thread-123", "do the thing")

	// Sanity: threadId and input are present (the function still does its job).
	if params["threadId"] != "thread-123" {
		t.Fatalf("expected threadId=thread-123, got %v", params["threadId"])
	}
	input, ok := params["input"].([]map[string]any)
	if !ok || len(input) == 0 {
		t.Fatalf("expected non-empty input, got %v", params["input"])
	}

	// No tools-related key should be set.
	for _, key := range []string{"tools", "allowedTools", "allowed_tools"} {
		if v, ok := params[key]; ok && v != nil {
			t.Fatalf("expected no %q key in codex params (API doesn't support it), got %v", key, v)
		}
	}
}

// ── Opencode ──

func TestBuildOpencodeArgsNoToolsByDefault(t *testing.T) {
	t.Parallel()

	opts := ExecOptions{
		Cwd:   "/tmp/work",
		Model: "anthropic/claude-sonnet-4-5",
	}
	args := buildOpencodeArgs(opts, "/usr/bin/opencode")

	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--allowedTools") || strings.Contains(joined, "--tools") {
		t.Fatalf("expected no tools flag when Tools is empty, got %q", joined)
	}
}

func TestBuildOpencodeArgsToolsUnsupported(t *testing.T) {
	t.Parallel()

	// Opencode's `run` command does not currently expose a per-invocation
	// tool restriction flag (verified against `opencode run --help`). The
	// plumbing is in place — when opencode supports it, the only change
	// needed is inside buildOpencodeArgs. For now the helper ignores
	// opts.Tools and does not add any flag.
	opts := ExecOptions{
		Tools: []string{"read_file", "search_code"},
	}
	args := buildOpencodeArgs(opts, "/usr/bin/opencode")

	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--allowedTools") || strings.Contains(joined, "--tools") {
		t.Fatalf("expected no tools flag (opencode does not support it), got %q", joined)
	}
}
