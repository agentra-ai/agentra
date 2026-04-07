package cli

import "testing"

func TestResolveSiteURLFromEnv(t *testing.T) {
	t.Run("prefers NEXT_PUBLIC_SITE_URL", func(t *testing.T) {
		t.Setenv("NEXT_PUBLIC_SITE_URL", "http://web.example")
		t.Setenv("AGENTRA_APP_URL", "http://app.example")
		t.Setenv("FRONTEND_ORIGIN", "http://origin.example")

		if got := ResolveSiteURLFromEnv(); got != "http://web.example" {
			t.Fatalf("ResolveSiteURLFromEnv() = %q, want %q", got, "http://web.example")
		}
	})

	t.Run("falls back through app and origin", func(t *testing.T) {
		t.Setenv("NEXT_PUBLIC_SITE_URL", "")
		t.Setenv("AGENTRA_APP_URL", "")
		t.Setenv("FRONTEND_ORIGIN", "http://origin.example/")

		if got := ResolveSiteURLFromEnv(); got != "http://origin.example" {
			t.Fatalf("ResolveSiteURLFromEnv() = %q, want %q", got, "http://origin.example")
		}
	})
}

func TestResolveLocalDaemonBaseURL(t *testing.T) {
	t.Setenv("AGENTRA_LOCAL_BIND_HOST", "127.0.0.9")

	if got := ResolveLocalDaemonBaseURL("19514"); got != "http://127.0.0.9:19514" {
		t.Fatalf("ResolveLocalDaemonBaseURL() = %q, want %q", got, "http://127.0.0.9:19514")
	}
}
