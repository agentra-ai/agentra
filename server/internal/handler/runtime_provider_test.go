package handler

import (
	"strings"
	"testing"

	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

func TestValidateLocalRuntimeProvider(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{"claude", "codex", "opencode"} {
		provider := provider
		t.Run(provider, func(t *testing.T) {
			t.Parallel()
			got, err := validateLocalRuntimeProvider("  " + strings.ToUpper(provider) + "  ")
			if err != nil {
				t.Fatalf("validateLocalRuntimeProvider() error: %v", err)
			}
			if got != provider {
				t.Fatalf("provider = %q, want %q", got, provider)
			}
		})
	}

	for _, provider := range []string{"", "unknown", "anthropic"} {
		if _, err := validateLocalRuntimeProvider(provider); err == nil {
			t.Errorf("validateLocalRuntimeProvider(%q) succeeded, want error", provider)
		}
	}
}

func TestCanonicalAgentProvider(t *testing.T) {
	t.Parallel()

	local := db.AgentRuntime{RuntimeMode: "local", Provider: "Claude"}
	provider, err := canonicalAgentProvider(local, "")
	if err != nil {
		t.Fatalf("canonicalAgentProvider(local) error: %v", err)
	}
	if provider != "claude" {
		t.Fatalf("local provider = %q, want claude", provider)
	}
	if _, err := canonicalAgentProvider(local, "codex"); err == nil {
		t.Fatal("mismatched requested provider succeeded")
	}

	cloud := db.AgentRuntime{RuntimeMode: "cloud", Provider: "Anthropic"}
	provider, err = canonicalAgentProvider(cloud, "anthropic")
	if err != nil {
		t.Fatalf("canonicalAgentProvider(cloud) error: %v", err)
	}
	if provider != "anthropic" {
		t.Fatalf("cloud provider = %q, want anthropic", provider)
	}
}

func TestValidateLoopRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		runtime db.AgentRuntime
		wantErr bool
	}{
		{name: "claude", runtime: db.AgentRuntime{RuntimeMode: "local", Provider: "claude"}},
		{name: "codex", runtime: db.AgentRuntime{RuntimeMode: "local", Provider: "codex"}, wantErr: true},
		{name: "opencode", runtime: db.AgentRuntime{RuntimeMode: "local", Provider: "opencode"}, wantErr: true},
		{name: "cloud", runtime: db.AgentRuntime{RuntimeMode: "cloud", Provider: "anthropic"}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateLoopRuntime(tt.runtime)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateLoopRuntime() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "max_turns") {
				t.Fatalf("error = %q, want explicit max_turns capability", err)
			}
		})
	}
}
