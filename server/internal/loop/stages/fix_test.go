package stages

import (
	"context"
	"strings"
	"testing"
)

// TestFix_ReturnsSystemPrompt confirms BuildFixPrompt loads the embedded
// fix template, substitutes {{.IssueID}} / {{.IssueTitle}} / {{.Branch}} /
// {{.Iteration}} / {{.WorkDir}}, and packages a non-empty system prompt
// with a non-zero turn cap and a non-empty tool set. The test is
// independent of any agent backend — BuildFixPrompt is a pure
// prompt-builder.
func TestFix_ReturnsSystemPrompt(t *testing.T) {
	ref := TaskRef{
		IssueID:    "issue-15",
		IssueTitle: "Implement Fix stage",
		Branch:     "feat/fix-stage",
		Iteration:  2,
		WorkDir:    "/tmp/agentra-fix",
	}
	out, err := BuildFixPrompt(ref)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil {
		t.Fatal("BuildFixPrompt returned nil prompt")
	}
	if out.SystemPrompt == "" {
		t.Error("expected non-empty system prompt")
	}
	if !strings.Contains(out.SystemPrompt, "issue-15") {
		t.Errorf("expected system prompt to contain substituted IssueID, got %q", out.SystemPrompt)
	}
	if !strings.Contains(out.SystemPrompt, "Implement Fix stage") {
		t.Errorf("expected system prompt to contain substituted IssueTitle, got %q", out.SystemPrompt)
	}
	if !strings.Contains(out.SystemPrompt, "Fix") {
		t.Errorf("expected system prompt to reference Fix stage, got %q", out.SystemPrompt)
	}
	if out.UserPrompt == "" {
		t.Error("expected non-empty user prompt")
	}
	if !strings.Contains(out.UserPrompt, "feat/fix-stage") {
		t.Errorf("expected user prompt to mention working branch, got %q", out.UserPrompt)
	}
	if out.MaxTurns <= 0 {
		t.Errorf("expected MaxTurns > 0, got %d", out.MaxTurns)
	}
	if len(out.Tools) == 0 {
		t.Error("expected non-empty Tools list for fix stage")
	}
}

// TestFix_Registered confirms the Fix executor is wired into the stages
// registry under "loop_fix" and that it does NOT call backend.Execute (it
// returns a prompt, not a session). We pass a nil backend to prove the
// contract: the executor only touches the backend parameter to satisfy
// the signature.
func TestFix_Registered(t *testing.T) {
	e, err := Resolve("loop_fix")
	if err != nil {
		t.Fatalf("loop_fix should be registered: %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil executor")
	}
	// nil backend is intentional — Fix must not dereference it.
	res, err := e(context.Background(), TaskRef{IssueID: "x", Branch: "b", Iteration: 1}, nilAgentBackend())
	if err != nil {
		t.Fatalf("executor: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.Output == "" {
		t.Error("expected Fix result to carry the loaded system prompt in Output")
	}
}

// TestFixPrompt_DoesNotCreateBranch is a regression guard. The Fix stage
// runs after Develop has already pushed the branch and opened the PR; Fix
// only iterates on the existing branch and updates the existing PR. The
// tool set must therefore NOT include create_branch or
// github_pr_create. If a future edit to toolsForStage accidentally grants
// those tools to loop_fix, this test catches it.
func TestFixPrompt_DoesNotCreateBranch(t *testing.T) {
	tools := toolsForStage("loop_fix")
	if len(tools) == 0 {
		t.Fatal("expected non-empty tool set for loop_fix")
	}
	forbidden := []string{"create_branch", "github_pr_create"}
	for _, bad := range forbidden {
		for _, got := range tools {
			if got == bad {
				t.Errorf("loop_fix tool set must not include %q (Fix reuses the branch/PR from Develop); got %v", bad, tools)
			}
		}
	}
}

// TestFixPrompt_IterationInUserPrompt confirms the loop's Iteration value
// is threaded into the user prompt the Fix executor builds. The iteration
// number is what the loop coordinator bumps on each Review rejection
// (see TestDecideNextStage_ReviewRejectedToFix) and is the model's only
// signal that this is a follow-up iteration, not the first fix attempt.
// If a future refactor drops {{.Iteration}} from the user prompt, this
// test catches it.
func TestFixPrompt_IterationInUserPrompt(t *testing.T) {
	ref := TaskRef{
		IssueID:    "issue-15",
		IssueTitle: "Iteration threading",
		Branch:     "feat/fix-iter",
		Iteration:  2,
		WorkDir:    "/tmp/agentra",
	}
	out, err := BuildFixPrompt(ref)
	if err != nil {
		t.Fatalf("BuildFixPrompt: %v", err)
	}
	if !strings.Contains(out.UserPrompt, "2") {
		t.Errorf("expected user prompt to mention iteration 2, got %q", out.UserPrompt)
	}
}
