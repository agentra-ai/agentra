package cli

import (
	"errors"
	"testing"
)

func TestResolveReportedCLIVersion(t *testing.T) {
	origLookPath := lookPathVersionBinary
	origExecutable := resolveExecutablePath
	origEvalSymlink := evalSymlinkPath
	origRunVersion := runVersionBinary
	t.Cleanup(func() {
		lookPathVersionBinary = origLookPath
		resolveExecutablePath = origExecutable
		evalSymlinkPath = origEvalSymlink
		runVersionBinary = origRunVersion
	})

	lookPathVersionBinary = func(_ string) (string, error) {
		return "/usr/local/bin/agentra", nil
	}
	resolveExecutablePath = func() (string, error) {
		return "/tmp/agentra-dev", nil
	}
	evalSymlinkPath = func(path string) (string, error) {
		return path, nil
	}

	t.Run("keeps embedded release version", func(t *testing.T) {
		runVersionBinary = func(_ string) ([]byte, error) {
			return []byte("agentra 0.0.2 (commit: abc123)\n"), nil
		}

		if got := ResolveReportedCLIVersion("0.0.3"); got != "0.0.3" {
			t.Fatalf("ResolveReportedCLIVersion() = %q, want %q", got, "0.0.3")
		}
	})

	t.Run("falls back from dev to installed version", func(t *testing.T) {
		runVersionBinary = func(_ string) ([]byte, error) {
			return []byte("agentra 0.0.2 (commit: abc123)\n"), nil
		}

		if got := ResolveReportedCLIVersion("dev"); got != "0.0.2" {
			t.Fatalf("ResolveReportedCLIVersion() = %q, want %q", got, "0.0.2")
		}
	})

	t.Run("keeps dev when installed version cannot be resolved", func(t *testing.T) {
		runVersionBinary = func(_ string) ([]byte, error) {
			return nil, errors.New("boom")
		}

		if got := ResolveReportedCLIVersion("dev"); got != "dev" {
			t.Fatalf("ResolveReportedCLIVersion() = %q, want %q", got, "dev")
		}
	})
}
