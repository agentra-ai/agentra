package daemon

import (
	"testing"
)

// TestLoadConfig_ServerURLPriority covers the host-mode override chain for
// AGENTRA_SERVER_URL: AGENTRA_CLI_SERVER_URL beats AGENTRA_SERVER_URL when
// both are set, and Overrides.ServerURL beats both.
func TestLoadConfig_ServerURLPriority(t *testing.T) {
	t.Run("AGENTRA_CLI_SERVER_URL wins over AGENTRA_SERVER_URL", func(t *testing.T) {
		t.Setenv("AGENTRA_CLI_SERVER_URL", "http://cli.example")
		t.Setenv("AGENTRA_SERVER_URL", "ws://compose-internal:8080/ws")
		cfg, err := LoadConfig(Overrides{})
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.ServerBaseURL != "http://cli.example" {
			t.Fatalf("ServerBaseURL = %q, want %q", cfg.ServerBaseURL, "http://cli.example")
		}
	})

	t.Run("falls back to AGENTRA_SERVER_URL when CLI override is empty", func(t *testing.T) {
		t.Setenv("AGENTRA_CLI_SERVER_URL", "")
		t.Setenv("AGENTRA_SERVER_URL", "http://server.example")
		cfg, err := LoadConfig(Overrides{})
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.ServerBaseURL != "http://server.example" {
			t.Fatalf("ServerBaseURL = %q, want %q", cfg.ServerBaseURL, "http://server.example")
		}
	})

	t.Run("Overrides.ServerURL beats both env vars", func(t *testing.T) {
		t.Setenv("AGENTRA_CLI_SERVER_URL", "http://cli.example")
		t.Setenv("AGENTRA_SERVER_URL", "http://server.example")
		cfg, err := LoadConfig(Overrides{ServerURL: "http://flag.example"})
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if cfg.ServerBaseURL != "http://flag.example" {
			t.Fatalf("ServerBaseURL = %q, want %q", cfg.ServerBaseURL, "http://flag.example")
		}
	})

	t.Run("errors when neither env var is set and no override", func(t *testing.T) {
		t.Setenv("AGENTRA_CLI_SERVER_URL", "")
		t.Setenv("AGENTRA_SERVER_URL", "")
		if _, err := LoadConfig(Overrides{}); err == nil {
			t.Fatal("LoadConfig: expected error, got nil")
		}
	})
}
