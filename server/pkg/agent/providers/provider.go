// Package agentproviders provides API-based LLM provider backends.
package agentproviders

import (
	"context"
	"fmt"

	"github.com/agentra-ai/agentra/server/pkg/agent/types"
)

// Re-export for convenience.
type (
	Session     = types.Session
	Message     = types.Message
	MessageType = types.MessageType
	Result      = types.Result
	TokenUsage  = types.TokenUsage
	ExecOptions = types.ExecOptions
)

// Re-export constants.
const (
	MessageText       = types.MessageText
	MessageThinking   = types.MessageThinking
	MessageToolUse    = types.MessageToolUse
	MessageToolResult = types.MessageToolResult
	MessageStatus     = types.MessageStatus
	MessageError      = types.MessageError
	MessageLog        = types.MessageLog
)

// Model describes a supported model for a Provider.
type Model struct {
	Provider string // e.g., "anthropic", "openai", "openrouter", "ollama"
	Name     string // e.g., "claude-3-5-sonnet-20241022"
}

// Provider is the interface for API-based LLM backends.
type Provider interface {
	Name() string
	Models() []Model
	Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error)
	StreamExecute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error)
	Supports(model Model) bool
}

// APIConfig holds configuration for an API-based provider.
type APIConfig struct {
	APIKey      string            // the API key / token
	Endpoint    string            // base URL (optional, defaults to official API)
	Extra       map[string]string // extra headers or config
	WorkspaceID string            // workspace ID, used for key encryption passphrase
}

// NewProvider creates a Provider for the given type.
func NewProvider(providerType string, cfg APIConfig) (Provider, error) {
	switch providerType {
	case "anthropic":
		return NewAnthropicProvider(cfg), nil
	case "openai":
		return NewOpenAIProvider(cfg), nil
	case "openrouter":
		return NewOpenRouterProvider(cfg), nil
	case "ollama":
		return NewOllamaProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %q", providerType)
	}
}