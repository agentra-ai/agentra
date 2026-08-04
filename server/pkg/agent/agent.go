// Package agent provides a unified interface for executing prompts via
// coding agents (Claude Code, Codex, OpenCode). It mirrors the happy-cli AgentBackend
// pattern, translated to idiomatic Go.
package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/agentra-ai/agentra/server/pkg/agent/types"
)

// Re-export shared types for backward compatibility.
type (
	Session     = types.Session
	MessageType = types.MessageType
	Message     = types.Message
	ExecOptions = types.ExecOptions
	Result      = types.Result
	TokenUsage  = types.TokenUsage
)

// Re-export constants.
const (
	MessageText       = types.MessageText
	MessageThinking   = types.MessageThinking
	MessageToolUse    = types.MessageToolUse
	MessageToolResult = types.MessageToolResult
	MessageStatus     = types.MessageStatus
	MessageSession    = types.MessageSession
	MessageError      = types.MessageError
	MessageLog        = types.MessageLog
)

// Backend is the Runtime Adapter v1 interface for coding-agent CLIs.
type Backend interface {
	// Descriptor returns the complete, machine-readable capability contract.
	Descriptor() AdapterDescriptor
	// Discover resolves the executable and reports its installed version.
	Discover(ctx context.Context) (Discovery, error)
	// Models lists provider models or returns UnsupportedCapabilityError.
	Models(ctx context.Context) ([]Model, error)
	// Execute runs a prompt and returns a Session for streaming results.
	// The caller should read from Session.Messages (optional) and wait on
	// Session.Result for the final outcome.
	Execute(ctx context.Context, prompt string, opts types.ExecOptions) (*types.Session, error)
}

// Config configures a Backend instance.
type Config struct {
	ExecutablePath string            // path to CLI binary (claude, codex, or opencode)
	Env            map[string]string // extra environment variables
	Logger         *slog.Logger
}

// New creates a Backend for the given agent type.
// Supported types: "claude", "codex", "opencode".
func New(agentType string, cfg Config) (Backend, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	switch agentType {
	case "claude":
		return &claudeBackend{cfg: cfg}, nil
	case "codex":
		return &codexBackend{cfg: cfg}, nil
	case "opencode":
		return &opencodeBackend{cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("unknown agent type: %q (supported: claude, codex, opencode)", agentType)
	}
}

// DetectVersion runs the agent CLI with --version and returns the output.
func DetectVersion(ctx context.Context, executablePath string) (string, error) {
	return detectCLIVersion(ctx, executablePath)
}
