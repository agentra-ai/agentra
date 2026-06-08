// Package types holds the shared domain types used by both the agent package
// and the providers package, avoiding import cycles.
package types

import "time"

// Session represents a running agent execution.
type Session struct {
	// Messages streams events as the agent works. The channel is closed
	// when the agent finishes (before Result is sent).
	Messages <-chan Message
	// Result receives exactly one value — the final outcome — then closes.
	Result <-chan Result
}

// MessageType identifies the kind of Message.
type MessageType string

const (
	MessageText       MessageType = "text"
	MessageThinking   MessageType = "thinking"
	MessageToolUse    MessageType = "tool-use"
	MessageToolResult MessageType = "tool-result"
	MessageStatus     MessageType = "status"
	MessageError      MessageType = "error"
	MessageLog        MessageType = "log"
)

// Message is a unified event emitted by an agent during execution.
type Message struct {
	Type    MessageType
	Content string         // text content (Text, Error, Log)
	Tool    string         // tool name (ToolUse, ToolResult)
	CallID  string         // tool call ID (ToolUse, ToolResult)
	Input   map[string]any // tool input (ToolUse)
	Output  string         // tool output (ToolResult)
	Status  string         // agent status string (Status)
	Level   string         // log level (Log)
}

// ExecOptions configures a single execution.
type ExecOptions struct {
	Cwd             string
	Model           string
	SystemPrompt    string
	MaxTurns        int
	Timeout         time.Duration
	ResumeSessionID string // if non-empty, resume a previous agent session

	// Tools restricts the agent's tool set to the given list. When empty (the
	// default for non-loop tasks), the agent CLI uses its full default tool
	// set. When non-empty, behavior depends on the provider:
	//   - Claude: passed as --allowedTools (comma-joined)
	//   - Codex:  included in the turn/start JSON-RPC params (provider-specific)
	//   - Opencode: included in the run command flags (provider-specific)
	Tools []string
}

// Result is the final outcome after an agent session completes.
type Result struct {
	Status     string // "completed", "failed", "aborted", "timeout"
	Output     string // accumulated text output
	Error      string // error message if failed
	DurationMs int64
	SessionID  string

	// TokenUsage is populated by API-based providers after execution.
	TokenUsage *TokenUsage
}

// TokenUsage holds token consumption metrics from an API provider.
type TokenUsage struct {
	InputTokens      int64 `json:"input_tokens"`
	OutputTokens     int64 `json:"output_tokens"`
	CacheReadTokens  int64 `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int64 `json:"cache_write_tokens,omitempty"`
}
