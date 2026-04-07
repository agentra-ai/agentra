package main

import (
	"strings"
	"testing"
)

func TestShouldReplaceLegacyDaemon(t *testing.T) {
	t.Run("replaces running legacy daemon", func(t *testing.T) {
		health := map[string]any{
			"status":     "running",
			"server_url": "https://server.multica.orb.local",
		}

		if !shouldReplaceLegacyDaemon(health) {
			t.Fatal("shouldReplaceLegacyDaemon() = false, want true")
		}
	})

	t.Run("does not replace matching agentra daemon", func(t *testing.T) {
		health := map[string]any{
			"status":     "running",
			"server_url": "http://server.agentra.orb.local",
		}

		if shouldReplaceLegacyDaemon(health) {
			t.Fatal("shouldReplaceLegacyDaemon() = true, want false")
		}
	})
}

func TestDaemonStartConflictError(t *testing.T) {
	t.Run("ignores matching daemon target", func(t *testing.T) {
		health := map[string]any{
			"status":     "running",
			"pid":        float64(42),
			"server_url": "http://server.agentra.orb.local",
		}

		if err := daemonStartConflictError("", health, "http://server.agentra.orb.local"); err != nil {
			t.Fatalf("daemonStartConflictError() error = %v, want nil", err)
		}
	})

	t.Run("reports conflicting daemon target", func(t *testing.T) {
		health := map[string]any{
			"status":     "running",
			"pid":        float64(32196),
			"server_url": "https://server.other.example",
		}

		err := daemonStartConflictError("", health, "http://server.agentra.orb.local")
		if err == nil {
			t.Fatal("daemonStartConflictError() = nil, want error")
		}

		msg := err.Error()
		for _, part := range []string{
			"https://server.other.example",
			"http://server.agentra.orb.local",
			"32196",
		} {
			if !strings.Contains(msg, part) {
				t.Fatalf("daemonStartConflictError() = %q, want substring %q", msg, part)
			}
		}
	})
}

func TestDaemonPIDFromHealth(t *testing.T) {
	t.Run("prefers positive pid", func(t *testing.T) {
		health := map[string]any{"pid": float64(59595)}

		pid, ok := daemonPIDFromHealth(health)
		if !ok {
			t.Fatal("daemonPIDFromHealth() ok = false, want true")
		}
		if pid != 59595 {
			t.Fatalf("daemonPIDFromHealth() pid = %d, want 59595", pid)
		}
	})

	t.Run("rejects negative pid", func(t *testing.T) {
		health := map[string]any{"pid": float64(-1)}

		if pid, ok := daemonPIDFromHealth(health); ok || pid != 0 {
			t.Fatalf("daemonPIDFromHealth() = (%d, %v), want (0, false)", pid, ok)
		}
	})
}
