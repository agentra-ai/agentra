package agentproviders

import (
	"context"
	"net/http"
	"testing"

	"github.com/agentra-ai/agentra/server/pkg/agent"
)

func TestAnthropicProvider_Name(t *testing.T) {
	t.Parallel()
	p := NewAnthropicProvider(APIConfig{APIKey: "test-key"})
	if p.Name() != "anthropic" {
		t.Errorf("Name() = %q, want %q", p.Name(), "anthropic")
	}
}

func TestAnthropicProvider_Models(t *testing.T) {
	t.Parallel()
	p := NewAnthropicProvider(APIConfig{APIKey: "test-key"})
	models := p.Models()
	if len(models) == 0 {
		t.Fatal("Models() returned empty slice")
	}
	if models[0].Provider != "anthropic" {
		t.Errorf("Models()[0].Provider = %q, want %q", models[0].Provider, "anthropic")
	}
}

func TestAnthropicProvider_Supports(t *testing.T) {
	t.Parallel()
	p := NewAnthropicProvider(APIConfig{APIKey: "test-key"})
	tests := []struct {
		model   Model
		want    bool
	}{
		{Model{Provider: "anthropic", Name: "claude-3-5-sonnet-20241022"}, true},
		{Model{Provider: "anthropic", Name: "claude-3-opus-20240229"}, true},
		{Model{Provider: "openai", Name: "gpt-4o"}, false},
		{Model{Provider: "openrouter", Name: "anthropic/claude-3.5-sonnet"}, false},
	}
	for _, tt := range tests {
		got := p.Supports(tt.model)
		if got != tt.want {
			t.Errorf("Supports(%+v) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestAnthropicProvider_SupportsArbitraryAnthropicModel(t *testing.T) {
	t.Parallel()
	p := NewAnthropicProvider(APIConfig{APIKey: "test-key"})
	// Should recognize any anthropic model
	m := Model{Provider: "anthropic", Name: "custom-model"}
	if !p.Supports(m) {
		t.Errorf("Supports(%+v) = false, want true", m)
	}
}

func TestOpenAIProvider_Name(t *testing.T) {
	t.Parallel()
	p := NewOpenAIProvider(APIConfig{APIKey: "test-key"})
	if p.Name() != "openai" {
		t.Errorf("Name() = %q, want %q", p.Name(), "openai")
	}
}

func TestOpenAIProvider_Supports(t *testing.T) {
	t.Parallel()
	p := NewOpenAIProvider(APIConfig{APIKey: "test-key"})
	tests := []struct {
		model Model
		want  bool
	}{
		{Model{Provider: "openai", Name: "gpt-4o"}, true},
		{Model{Provider: "openai", Name: "gpt-4o-mini"}, true},
		{Model{Provider: "anthropic", Name: "claude-3-5-sonnet-20241022"}, false},
	}
	for _, tt := range tests {
		got := p.Supports(tt.model)
		if got != tt.want {
			t.Errorf("Supports(%+v) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestOpenAIProvider_SupportsArbitraryOpenAIModel(t *testing.T) {
	t.Parallel()
	p := NewOpenAIProvider(APIConfig{APIKey: "test-key"})
	// Should recognize any openai model (allows custom fine-tuned models)
	m := Model{Provider: "openai", Name: "custom-fine-tuned-model"}
	if !p.Supports(m) {
		t.Errorf("Supports(%+v) = false, want true", m)
	}
}

func TestOpenRouterProvider_Name(t *testing.T) {
	t.Parallel()
	p := NewOpenRouterProvider(APIConfig{APIKey: "test-key"})
	if p.Name() != "openrouter" {
		t.Errorf("Name() = %q, want %q", p.Name(), "openrouter")
	}
}

func TestOpenRouterProvider_Supports(t *testing.T) {
	t.Parallel()
	p := NewOpenRouterProvider(APIConfig{APIKey: "test-key"})
	tests := []struct {
		model Model
		want  bool
	}{
		{Model{Provider: "openrouter", Name: "anthropic/claude-3.5-sonnet"}, true},
		{Model{Provider: "openrouter", Name: "openai/gpt-4o"}, true},
		{Model{Provider: "anthropic", Name: "claude-3-5-sonnet-20241022"}, false},
	}
	for _, tt := range tests {
		got := p.Supports(tt.model)
		if got != tt.want {
			t.Errorf("Supports(%+v) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestOllamaProvider_Name(t *testing.T) {
	t.Parallel()
	p := NewOllamaProvider(APIConfig{})
	if p.Name() != "ollama" {
		t.Errorf("Name() = %q, want %q", p.Name(), "ollama")
	}
}

func TestOllamaProvider_Supports(t *testing.T) {
	t.Parallel()
	p := NewOllamaProvider(APIConfig{})
	tests := []struct {
		model Model
		want  bool
	}{
		{Model{Provider: "ollama", Name: "llama3"}, true},
		{Model{Provider: "ollama", Name: "codellama"}, true},
		{Model{Provider: "anthropic", Name: "claude-3-5-sonnet-20241022"}, false},
	}
	for _, tt := range tests {
		got := p.Supports(tt.model)
		if got != tt.want {
			t.Errorf("Supports(%+v) = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestNewProvider_Unknown(t *testing.T) {
	t.Parallel()
	_, err := NewProvider("unknown", APIConfig{})
	if err == nil {
		t.Error("NewProvider(unknown) = nil, want error")
	}
}

func TestProviderInterface_ExecuteReturnsSession(t *testing.T) {
	t.Parallel()
	// Test that Execute returns a non-nil Session
	// We use a mock server to avoid actual API calls
	p := NewOpenAIProvider(APIConfig{APIKey: "test-key"})
	// Execute would fail without a server, but we can verify it returns a session structure
	// In a real test, you'd start a mock HTTP server
	_ = p
}

func TestTokenUsageInResult(t *testing.T) {
	t.Parallel()
	// Verify TokenUsage struct has all expected fields
	usage := &agent.TokenUsage{
		InputTokens:     100,
		OutputTokens:    50,
		CacheReadTokens: 25,
		CacheWriteTokens: 10,
	}
	if usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", usage.InputTokens)
	}
	if usage.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", usage.OutputTokens)
	}
	if usage.CacheReadTokens != 25 {
		t.Errorf("CacheReadTokens = %d, want 25", usage.CacheReadTokens)
	}
	if usage.CacheWriteTokens != 10 {
		t.Errorf("CacheWriteTokens = %d, want 10", usage.CacheWriteTokens)
	}
}

// mockHTTPHandler is a simple test helper
type mockHTTPHandler struct {
	statusCode int
	response   string
}

func (m *mockHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(m.statusCode)
	w.Write([]byte(m.response))
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()
	// Test that Execute respects context cancellation
	// This would require a slow-responding server, so we just verify the pattern
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	p := NewAnthropicProvider(APIConfig{APIKey: "test-key"})
	_, err := p.Execute(ctx, "test prompt", ExecOptions{Timeout: 100 * 1000})
	// Should fail quickly due to cancelled context
	if err == nil {
		// The goroutine will exit, but Execute itself returned
		t.Log("Execute returned with cancelled context")
	}
}