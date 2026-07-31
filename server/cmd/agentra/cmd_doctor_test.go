package main

import (
	"path/filepath"
	"testing"

	"github.com/agentra-ai/agentra/server/internal/doctor"
)

func TestDoctorCommandRegistered(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"doctor"})
	if err != nil {
		t.Fatalf("find doctor command: %v", err)
	}
	if command != doctorCmd {
		t.Fatalf("found %q, want doctor", command.Name())
	}
	for _, name := range []string{"output", "timeout", "repo", "skip-repo-remote"} {
		if command.Flags().Lookup(name) == nil {
			t.Fatalf("doctor flag %q is not registered", name)
		}
	}
}

func TestDoctorWorkspacesRootForProfile(t *testing.T) {
	t.Setenv("AGENTRA_WORKSPACES_ROOT", "")
	home := t.TempDir()
	t.Setenv("HOME", home)

	root, err := doctorWorkspacesRoot("staging")
	if err != nil {
		t.Fatalf("doctorWorkspacesRoot: %v", err)
	}
	want := filepath.Join(home, "agentra_workspaces_staging")
	if root != want {
		t.Fatalf("root = %q, want %q", root, want)
	}
}

func TestDoctorStatusLabel(t *testing.T) {
	tests := map[doctor.Status]string{
		doctor.StatusPass:    "PASS",
		doctor.StatusWarning: "WARN",
		doctor.StatusFail:    "FAIL",
		doctor.StatusSkipped: "SKIP",
	}
	for status, want := range tests {
		if got := doctorStatusLabel(status); got != want {
			t.Fatalf("doctorStatusLabel(%q) = %q, want %q", status, got, want)
		}
	}
}
