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
	MessageSession    MessageType = "session"
	MessageError      MessageType = "error"
	MessageLog        MessageType = "log"
)

// Message is a unified event emitted by an agent during execution.
type Message struct {
	Type      MessageType
	Content   string         // text content (Text, Error, Log)
	Tool      string         // tool name (ToolUse, ToolResult)
	CallID    string         // tool call ID (ToolUse, ToolResult)
	Input     map[string]any // tool input (ToolUse)
	Output    string         // tool output (ToolResult)
	Status    string         // agent status string (Status)
	SessionID string         // resumable provider session ID (Session)
	Level     string         // log level (Log)
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
	// set. A non-empty value is accepted only when the adapter declares the
	// tool_restrictions capability; unsupported adapters reject it before launch.
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
	// Artifacts contains provider-declared outputs. CLI adapters that cannot
	// identify artifacts reliably leave this empty and declare the capability
	// unsupported instead of guessing from the worktree.
	Artifacts []Artifact
}

// TokenUsage holds token consumption metrics from an API provider.
type TokenUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens,omitempty"`
	CacheReadTokens       int64 `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens      int64 `json:"cache_write_tokens,omitempty"`
}

// Artifact is a provider-declared execution output. Path is workspace-local;
// URI is used for remote artifacts. At least one locator must be present when
// an adapter declares artifact support.
type Artifact struct {
	Kind      string `json:"kind"`
	Path      string `json:"path,omitempty"`
	URI       string `json:"uri,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
}
