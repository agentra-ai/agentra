// Package tools implements the agent-callable tools for loop stages.
// Each tool is a pure function: (args JSON) → (Result, error). The agent
// runtime calls Execute with parsed args; we return a Result with content,
// stderr, exit code, and an optional Error field.
package tools

import "context"

// Tool is the contract for a loop-stage tool. The agent runtime calls
// Execute with args parsed from the LLM's tool_use payload.
type Tool interface {
	Name() string
	Description() string
	Schema() map[string]any // JSON schema for tool_use protocol
	Execute(ctx context.Context, args map[string]any) (Result, error)
}

// Result is what Execute returns. Content is the primary success output.
// Error is non-empty only for tool-level errors (bad args, path traversal,
// timeout). A non-zero exit code from a shell command is NOT a tool error
// — it goes in ExitCode, and the LLM sees the stderr in Stderr.
type Result struct {
	Content  string `json:"content"`
	Error    string `json:"error,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
}

// Registry maps tool name → implementation. Populated by init() in each
// file. Note: the registered instances have empty WorkDir and are only
// useful for Schema/Description lookup. For actual execution, construct
// a fresh instance with the task's WorkDir.
var Registry = map[string]Tool{}

// Register adds a tool to the registry, keyed by Name(). Called from
// init() in each tool file.
func Register(t Tool) { Registry[t.Name()] = t }

// Get retrieves a tool by name. The (Tool, bool) return mirrors map
// lookup so callers can distinguish "unknown tool" from "nil tool".
func Get(name string) (Tool, bool) {
	t, ok := Registry[name]
	return t, ok
}
