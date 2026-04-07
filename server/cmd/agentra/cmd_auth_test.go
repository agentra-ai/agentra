package main

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/agentra-ai/agentra/server/internal/cli"
)

// testCmd returns a minimal cobra.Command with the --profile persistent flag
// registered, matching the rootCmd setup used in production.
func testCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.PersistentFlags().String("profile", "", "")
	return cmd
}

func TestResolveAppURL(t *testing.T) {
	cmd := testCmd()

	t.Run("prefers NEXT_PUBLIC_SITE_URL", func(t *testing.T) {
		t.Setenv("NEXT_PUBLIC_SITE_URL", "http://env.example")
		t.Setenv("AGENTRA_APP_URL", "http://localhost:14000")
		t.Setenv("FRONTEND_ORIGIN", "http://localhost:13000")

		if got := resolveAppURL(cmd); got != "http://env.example" {
			t.Fatalf("resolveAppURL() = %q, want %q", got, "http://env.example")
		}
	})

	t.Run("prefers AGENTRA_APP_URL", func(t *testing.T) {
		t.Setenv("NEXT_PUBLIC_SITE_URL", "")
		t.Setenv("AGENTRA_APP_URL", "http://localhost:14000")
		t.Setenv("FRONTEND_ORIGIN", "http://localhost:13000")

		if got := resolveAppURL(cmd); got != "http://localhost:14000" {
			t.Fatalf("resolveAppURL() = %q, want %q", got, "http://localhost:14000")
		}
	})

	t.Run("falls back to FRONTEND_ORIGIN", func(t *testing.T) {
		t.Setenv("NEXT_PUBLIC_SITE_URL", "")
		t.Setenv("AGENTRA_APP_URL", "")
		t.Setenv("FRONTEND_ORIGIN", "http://localhost:13026")

		if got := resolveAppURL(cmd); got != "http://localhost:13026" {
			t.Fatalf("resolveAppURL() = %q, want %q", got, "http://localhost:13026")
		}
	})

	t.Run("returns empty when nothing is configured", func(t *testing.T) {
		t.Setenv("NEXT_PUBLIC_SITE_URL", "")
		t.Setenv("AGENTRA_APP_URL", "")
		t.Setenv("FRONTEND_ORIGIN", "")
		t.Setenv("HOME", t.TempDir()) // avoid reading real config

		if got := resolveAppURL(cmd); got != "" {
			t.Fatalf("resolveAppURL() = %q, want empty string", got)
		}
	})

	t.Run("ignores legacy production config", func(t *testing.T) {
		t.Setenv("NEXT_PUBLIC_SITE_URL", "")
		t.Setenv("AGENTRA_APP_URL", "")
		t.Setenv("FRONTEND_ORIGIN", "")
		t.Setenv("HOME", t.TempDir())

		if err := cli.SaveCLIConfigForProfile(cli.CLIConfig{
			AppURL: "https://agentra.ai",
		}, ""); err != nil {
			t.Fatalf("SaveCLIConfigForProfile() error = %v", err)
		}

		if got := resolveAppURL(cmd); got != "" {
			t.Fatalf("resolveAppURL() = %q, want empty string", got)
		}
	})
}

func TestResolveServerURL(t *testing.T) {
	cmd := testCmd()

	t.Run("returns empty when nothing is configured", func(t *testing.T) {
		t.Setenv("AGENTRA_SERVER_URL", "")
		t.Setenv("HOME", t.TempDir())

		if got := resolveServerURL(cmd); got != "" {
			t.Fatalf("resolveServerURL() = %q, want empty string", got)
		}
	})

	t.Run("ignores legacy production config", func(t *testing.T) {
		t.Setenv("AGENTRA_SERVER_URL", "")
		t.Setenv("HOME", t.TempDir())

		if err := cli.SaveCLIConfigForProfile(cli.CLIConfig{
			ServerURL: "https://api.agentra.ai",
		}, ""); err != nil {
			t.Fatalf("SaveCLIConfigForProfile() error = %v", err)
		}

		if got := resolveServerURL(cmd); got != "" {
			t.Fatalf("resolveServerURL() = %q, want empty string", got)
		}
	})
}

func TestNormalizeAPIBaseURL(t *testing.T) {
	t.Run("converts websocket base URL", func(t *testing.T) {
		if got := normalizeAPIBaseURL("ws://localhost:18106/ws"); got != "http://localhost:18106" {
			t.Fatalf("normalizeAPIBaseURL() = %q, want %q", got, "http://localhost:18106")
		}
	})

	t.Run("keeps http base URL", func(t *testing.T) {
		if got := normalizeAPIBaseURL("http://localhost:8080"); got != "http://localhost:8080" {
			t.Fatalf("normalizeAPIBaseURL() = %q, want %q", got, "http://localhost:8080")
		}
	})

	t.Run("falls back to raw value for invalid URL", func(t *testing.T) {
		if got := normalizeAPIBaseURL("://bad-url"); got != "://bad-url" {
			t.Fatalf("normalizeAPIBaseURL() = %q, want %q", got, "://bad-url")
		}
	})
}
