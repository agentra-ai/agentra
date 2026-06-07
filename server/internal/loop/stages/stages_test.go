package stages

import (
	"errors"
	"strings"
	"testing"
)

// TestRegistryUnknown confirms that Resolve returns an error wrapping
// ErrUnknownStage for a task type no executor has been registered against.
func TestRegistryUnknown(t *testing.T) {
	_, err := Resolve("nonexistent")
	if err == nil {
		t.Fatal("Resolve(\"nonexistent\") = nil error, want error")
	}
	if !errors.Is(err, ErrUnknownStage) {
		t.Fatalf("Resolve(\"nonexistent\") err = %v, want errors.Is ErrUnknownStage", err)
	}
}

// TestLoadPrompt confirms that the embedded prompt templates load and
// substitute the {{.IssueID}} variable. With embed.FS the test is
// independent of the current working directory.
func TestLoadPrompt(t *testing.T) {
	ref := TaskRef{
		ID:         "task-abc",
		IssueID:    "issue-42",
		IssueTitle: "Add stages package",
		Branch:     "feat/stages",
		Iteration:  3,
		WorkDir:    "/tmp/agentra",
	}
	p, err := loadPrompt("plan", ref)
	if err != nil {
		t.Fatalf("loadPrompt(\"plan\", ref) returned err: %v", err)
	}
	if p == "" {
		t.Fatal("loadPrompt(\"plan\", ref) returned empty string, want non-empty")
	}
	if !strings.Contains(p, ref.IssueID) {
		t.Errorf("loaded prompt missing %q after substitution; got first 200 chars: %q",
			ref.IssueID, truncate(p, 200))
	}
	// Sanity: the unsubstituted variable should be gone.
	if strings.Contains(p, "{{.IssueID}}") {
		t.Errorf("loaded prompt still contains literal {{.IssueID}}")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
