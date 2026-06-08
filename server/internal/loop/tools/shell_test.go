package tools

import (
	"context"
	"testing"
	"time"
)

// TestRunCommand_Echo confirms stdout is captured into Content and a
// successful command does not set Error.
func TestRunCommand_Echo(t *testing.T) {
	dir := t.TempDir()
	t1 := &RunCommandTool{WorkDir: dir, DefaultTimeout: 10 * time.Second}
	res, err := t1.Execute(context.Background(), map[string]any{"cmd": "echo hi"})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit %d, stderr=%s", res.ExitCode, res.Stderr)
	}
	if res.Content != "hi\n" {
		t.Errorf("got %q, want %q", res.Content, "hi\n")
	}
	if res.Error != "" {
		t.Errorf("unexpected tool error: %s", res.Error)
	}
}

// TestRunCommand_NonZeroExitNotToolError confirms a failed command
// (exit code 1 from `false`) is reported via ExitCode, not via Error.
// The LLM is expected to read stderr and decide what to do.
func TestRunCommand_NonZeroExitNotToolError(t *testing.T) {
	dir := t.TempDir()
	t1 := &RunCommandTool{WorkDir: dir, DefaultTimeout: 10 * time.Second}
	res, _ := t1.Execute(context.Background(), map[string]any{"cmd": "false"})
	if res.ExitCode == 0 {
		t.Error("expected non-zero exit code")
	}
	if res.Error != "" {
		t.Errorf("non-zero exit should not set Error, got %q", res.Error)
	}
}
