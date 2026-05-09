// Package agentproviders provides API-based LLM provider backends.
package agentproviders

import (
	"context"
	"fmt"
	"time"

	"github.com/agentra-ai/agentra/server/pkg/agent"
)

// Model describes a supported model for a Provider.
type Model struct {
	Provider string // e.g., "anthropic", "openai", "openrouter", "ollama"
	Name     string // e.g., "claude-3-5-sonnet-20241022"
}

// Provider is the interface for API-based LLM backends.
// It is separate from the CLI-based Backend interface and supports
// providers that communicate over HTTP (Anthropic, OpenAI, OpenRouter, Ollama).
type Provider interface {
	// Name returns the provider identifier (e.g., "anthropic", "openai").
	Name() string
	// Models returns the list of models supported by this provider.
	Models() []Model
	// Execute runs a prompt and returns a Session for streaming results.
	Execute(ctx context.Context, prompt string, opts ExecOptions) (*agent.Session, error)
	// StreamExecute runs a prompt and streams results via SSE.
	StreamExecute(ctx context.Context, prompt string, opts ExecOptions) (*agent.Session, error)
	// Supports returns true if this provider can handle the given model.
	Supports(model Model) bool
}

// ExecOptions configures a single API provider execution.
type ExecOptions struct {
	Cwd          string
	Model        string // Model name (e.g., "claude-3-5-sonnet-20241022")
	SystemPrompt string
	MaxTurns     int
	Timeout      time.Duration
}

// APIConfig holds configuration for an API-based provider.
type APIConfig struct {
	APIKey     string            // the API key / token
	Endpoint   string            // base URL (optional, defaults to official API)
	Extra      map[string]string // extra headers or config
	WorkspaceID string          // workspace ID, used for key encryption passphrase
}

// NewProvider creates a Provider for the given type.
// Supported types: "anthropic", "openai", "openrouter", "ollama".
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