package stages

import (
	"context"
	"strings"
	"testing"
)

// TestDevelop_ReturnsSystemPrompt confirms BuildDevelopPrompt loads the
// embedded develop template, substitutes {{.IssueID}} / {{.IssueTitle}} /
// {{.Branch}} / {{.Iteration}} / {{.WorkDir}}, and packages a non-empty
// system prompt with a non-zero turn cap and a non-empty tool set. The
// test is independent of any agent backend — BuildDevelopPrompt is a pure
// prompt-builder.
func TestDevelop_ReturnsSystemPrompt(t *testing.T) {
	ref := TaskRef{
		IssueID:    "issue-7",
		IssueTitle: "Implement Develop stage",
		Branch:     "feat/develop-stage",
		Iteration:  2,
		WorkDir:    "/tmp/agentra-dev",
	}
	out, err := BuildDevelopPrompt(ref)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil {
		t.Fatal("BuildDevelopPrompt returned nil prompt")
	}
	if out.SystemPrompt == "" {
		t.Error("expected non-empty system prompt")
	}
	if !strings.Contains(out.SystemPrompt, "issue-7") {
		t.Errorf("expected system prompt to contain substituted IssueID, got %q", out.SystemPrompt)
	}
	if !strings.Contains(out.SystemPrompt, "Implement Develop stage") {
		t.Errorf("expected system prompt to contain substituted IssueTitle, got %q", out.SystemPrompt)
	}
	if !strings.Contains(out.SystemPrompt, "Develop") {
		t.Errorf("expected system prompt to reference Develop stage, got %q", out.SystemPrompt)
	}
	if out.UserPrompt == "" {
		t.Error("expected non-empty user prompt")
	}
	if !strings.Contains(out.UserPrompt, "feat/develop-stage") {
		t.Errorf("expected user prompt to mention working branch, got %q", out.UserPrompt)
	}
	if !strings.Contains(out.UserPrompt, "Implement Develop stage") {
		t.Errorf("expected user prompt to mention issue title, got %q", out.UserPrompt)
	}
	if out.MaxTurns <= 0 {
		t.Errorf("expected MaxTurns > 0, got %d", out.MaxTurns)
	}
	if len(out.Tools) == 0 {
		t.Error("expected non-empty Tools list for develop stage")
	}
}

// TestDevelopPrompt_SubstitutesBranch confirms the develop template's
// {{.Branch}} placeholder is replaced. Branch substitution matters because
// the develop stage's prompt instructs the agent to check out a specific
// working branch, and an unsubstituted placeholder would either confuse
// the agent or break a downstream tool call.
func TestDevelopPrompt_SubstitutesBranch(t *testing.T) {
	ref := TaskRef{
		IssueID:    "issue-9",
		IssueTitle: "Branch substitution",
		Branch:     "feat/branch-sub",
		Iteration:  1,
		WorkDir:    "/tmp/agentra",
	}
	out, err := BuildDevelopPrompt(ref)
	if err != nil {
		t.Fatalf("BuildDevelopPrompt: %v", err)
	}
	if !strings.Contains(out.SystemPrompt, "feat/branch-sub") {
		t.Errorf("expected system prompt to contain substituted Branch, got %q", out.SystemPrompt)
	}
	if strings.Contains(out.SystemPrompt, "{{.Branch}}") {
		t.Errorf("expected literal {{.Branch}} placeholder to be substituted, got %q", out.SystemPrompt)
	}
}

// TestDevelop_Registered confirms the Develop executor is wired into the
// stages registry under "loop_develop" and that it does NOT call
// backend.Execute (it returns a prompt, not a session). We pass a nil
// backend to prove the contract: the executor only touches the backend
// parameter to satisfy the signature.
func TestDevelop_Registered(t *testing.T) {
	e, err := Resolve("loop_develop")
	if err != nil {
		t.Fatalf("loop_develop should be registered: %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil executor")
	}
	// nil backend is intentional — Develop must not dereference it.
	res, err := e(context.Background(), TaskRef{IssueID: "x", Branch: "b", Iteration: 1}, nilAgentBackend())
	if err != nil {
		t.Fatalf("executor: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.Output == "" {
		t.Error("expected Develop result to carry the loaded system prompt in Output")
	}
}
