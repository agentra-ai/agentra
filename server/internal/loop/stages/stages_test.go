package stages

import (
	"errors"
	"strings"
	"testing"

	"github.com/agentra-ai/agentra/server/pkg/agent"
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

func TestValidateAdapterForTaskType(t *testing.T) {
	claude, _ := agent.DescriptorFor(agent.ProviderClaude)
	codex, _ := agent.DescriptorFor(agent.ProviderCodex)

	if err := ValidateAdapterForTaskType(claude, "loop_plan"); err != nil {
		t.Fatalf("Claude loop_plan validation failed: %v", err)
	}
	if err := ValidateAdapterForTaskType(codex, "standard"); err != nil {
		t.Fatalf("Codex standard validation failed: %v", err)
	}
	err := ValidateAdapterForTaskType(codex, "loop_develop")
	if err == nil || !agent.IsUnsupportedCapability(err) || !strings.Contains(err.Error(), "max_turns") {
		t.Fatalf("Codex loop_develop validation = %v, want max_turns capability error", err)
	}
	err = ValidateAdapterForTaskType(claude, "loop_unknown")
	if err == nil || !strings.Contains(err.Error(), "unsupported loop task type") {
		t.Fatalf("unknown loop validation = %v", err)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
