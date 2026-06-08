package stages

import (
	"context"
	"strings"
	"testing"
)

// TestReview_ReturnsSystemPrompt confirms BuildReviewPrompt loads the
// embedded review template, substitutes {{.IssueID}} / {{.IssueTitle}} /
// {{.Branch}} / {{.Iteration}} / {{.WorkDir}}, and packages a non-empty
// system prompt with a non-zero turn cap and a non-empty tool set. The
// test is independent of any agent backend — BuildReviewPrompt is a pure
// prompt-builder.
func TestReview_ReturnsSystemPrompt(t *testing.T) {
	ref := TaskRef{
		IssueID:    "issue-14",
		IssueTitle: "Implement Review stage",
		Branch:     "feat/review-stage",
		Iteration:  1,
		WorkDir:    "/tmp/agentra-review",
	}
	out, err := BuildReviewPrompt(ref)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil {
		t.Fatal("BuildReviewPrompt returned nil prompt")
	}
	if out.SystemPrompt == "" {
		t.Error("expected non-empty system prompt")
	}
	if !strings.Contains(out.SystemPrompt, "issue-14") {
		t.Errorf("expected system prompt to contain substituted IssueID, got %q", out.SystemPrompt)
	}
	if !strings.Contains(out.SystemPrompt, "Implement Review stage") {
		t.Errorf("expected system prompt to contain substituted IssueTitle, got %q", out.SystemPrompt)
	}
	if !strings.Contains(out.SystemPrompt, "Review") {
		t.Errorf("expected system prompt to reference Review stage, got %q", out.SystemPrompt)
	}
	if out.UserPrompt == "" {
		t.Error("expected non-empty user prompt")
	}
	if !strings.Contains(out.UserPrompt, "feat/review-stage") {
		t.Errorf("expected user prompt to mention working branch, got %q", out.UserPrompt)
	}
	if !strings.Contains(out.UserPrompt, "Implement Review stage") {
		t.Errorf("expected user prompt to mention issue title, got %q", out.UserPrompt)
	}
	if out.MaxTurns <= 0 {
		t.Errorf("expected MaxTurns > 0, got %d", out.MaxTurns)
	}
	if len(out.Tools) == 0 {
		t.Error("expected non-empty Tools list for review stage")
	}
}

// TestReview_Registered confirms the Review executor is wired into the
// stages registry under "loop_review" and that it does NOT call
// backend.Execute (it returns a prompt, not a session). We pass a nil
// backend to prove the contract: the executor only touches the backend
// parameter to satisfy the signature.
func TestReview_Registered(t *testing.T) {
	e, err := Resolve("loop_review")
	if err != nil {
		t.Fatalf("loop_review should be registered: %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil executor")
	}
	// nil backend is intentional — Review must not dereference it.
	res, err := e(context.Background(), TaskRef{IssueID: "x", Branch: "b", Iteration: 1}, nilAgentBackend())
	if err != nil {
		t.Fatalf("executor: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.Output == "" {
		t.Error("expected Review result to carry the loaded system prompt in Output")
	}
}

// TestReviewPrompt_OutputShape is a guard against prompt drift. The
// coordinator's parseTaskResult decodes the agent's final text as JSON
// into loop.TaskResult, whose fields are tagged with the names
// review_approved, review_issues, pr_url, pr_number, and branch_name.
// If a future edit to prompts/review.md renames any of those fields, the
// coordinator will silently drop the review verdict. This test reads the
// embedded template directly and asserts the field names are still
// present.
func TestReviewPrompt_OutputShape(t *testing.T) {
	ref := TaskRef{
		IssueID:    "issue-14",
		IssueTitle: "Output shape guard",
		Branch:     "feat/review",
		Iteration:  1,
		WorkDir:    "/tmp/agentra",
	}
	prompt, err := loadPrompt("review", ref)
	if err != nil {
		t.Fatalf("loadPrompt(review): %v", err)
	}
	required := []string{
		"review_approved",
		"review_issues",
		"pr_url",
		"pr_number",
		"branch_name",
	}
	for _, field := range required {
		if !strings.Contains(prompt, field) {
			t.Errorf("review prompt is missing required JSON field name %q; coordinator's parseTaskResult will drop the verdict", field)
		}
	}
}

// TestReviewPrompt_ReadOnlyTools confirms the review stage's tool set is
// strictly read-only. The reviewer inspects the develop stage's diff and
// emits a verdict — it must not write files, commit, or push. This test
// catches drift if a future edit to toolsForStage accidentally grants
// the review stage mutating tools.
func TestReviewPrompt_ReadOnlyTools(t *testing.T) {
	tools := toolsForStage("loop_review")
	if len(tools) == 0 {
		t.Fatal("expected non-empty tool set for loop_review")
	}
	required := map[string]bool{
		"read_file": true,
		"git_diff":  true,
	}
	for name := range required {
		found := false
		for _, got := range tools {
			if got == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("loop_review tool set is missing required read-only tool %q; got %v", name, tools)
		}
	}
	mutating := []string{"write_file", "git_commit", "git_push", "create_branch", "github_pr_create", "run_command", "run_test"}
	for _, bad := range mutating {
		for _, got := range tools {
			if got == bad {
				t.Errorf("loop_review tool set must not include mutating tool %q; got %v", bad, tools)
			}
		}
	}
}
