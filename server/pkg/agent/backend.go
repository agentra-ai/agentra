package agent

import (
	"context"
	"fmt"

	"github.com/agentra-ai/agentra/server/pkg/agent/types"
	agentproviders "github.com/agentra-ai/agentra/server/pkg/agent/providers"
)

// ExtendedBackend is a Backend that exposes metadata about its provider.
type ExtendedBackend interface {
	Backend
	ProviderType() ProviderType
	Model() string
	Capabilities() *Capabilities
}

// ProviderType identifies the type of backend.
type ProviderType string

const (
	ProviderClaude     ProviderType = "claude"
	ProviderCodex      ProviderType = "codex"
	ProviderOpenCode   ProviderType = "opencode"
	ProviderOpenAI     ProviderType = "openai"
	ProviderAnthropic  ProviderType = "anthropic"
	ProviderGemini     ProviderType = "gemini"
	ProviderOllama     ProviderType = "ollama"
	ProviderOpenRouter ProviderType = "openrouter"
)

// Capabilities describes what a backend supports.
type Capabilities struct {
	Streaming     bool
	ContextWindow int
	Tools         bool
	MultiModal    bool
}

// BackendFacade provides a unified interface across all provider types.
type BackendFacade struct {
	cliBackends  map[ProviderType]Backend
	apiProviders map[ProviderType]agentproviders.Provider
	defaultType  ProviderType
}

func NewBackendFacade(defaultProvider ProviderType) *BackendFacade {
	return &BackendFacade{
		cliBackends:  make(map[ProviderType]Backend),
		apiProviders: make(map[ProviderType]agentproviders.Provider),
		defaultType:  defaultProvider,
	}
}

// RegisterCLIBackend registers a CLI-based backend (Claude Code, Codex, OpenCode).
func (f *BackendFacade) RegisterCLIBackend(p ProviderType, b Backend) {
	f.cliBackends[p] = b
}

// RegisterAPIProvider registers an API-based provider (OpenAI, Anthropic, Ollama).
func (f *BackendFacade) RegisterAPIProvider(p ProviderType, provider agentproviders.Provider) {
	f.apiProviders[p] = provider
}

// Execute delegates to the appropriate backend.
func (f *BackendFacade) Execute(ctx context.Context, p ProviderType, prompt string, opts types.ExecOptions) (*types.Session, error) {
	if backend, ok := f.cliBackends[p]; ok {
		return backend.Execute(ctx, prompt, opts)
	}
	if provider, ok := f.apiProviders[p]; ok {
		return provider.Execute(ctx, prompt, opts)
	}
	// Fallback to default
	if backend, ok := f.cliBackends[f.defaultType]; ok {
		return backend.Execute(ctx, prompt, opts)
	}
	return nil, fmt.Errorf("no backend available for provider %s", p)
}
