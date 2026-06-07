package stages

import (
	"context"
	"strings"
	"testing"

	"github.com/agentra-ai/agentra/server/pkg/agent"
)

// TestPlan_ReturnsSystemPrompt confirms BuildPlanPrompt loads the embedded
// plan template, substitutes {{.IssueID}}, and produces a non-empty system
// prompt that references the Plan stage. The test is independent of any
// agent backend — BuildPlanPrompt is a pure prompt-builder.
func TestPlan_ReturnsSystemPrompt(t *testing.T) {
	out, err := BuildPlanPrompt(TaskRef{IssueID: "issue-1", IssueTitle: "Implement auth"})
	if err != nil {
		t.Fatal(err)
	}
	if out == nil {
		t.Fatal("BuildPlanPrompt returned nil prompt")
	}
	if out.SystemPrompt == "" {
		t.Error("expected non-empty system prompt")
	}
	if !strings.Contains(out.SystemPrompt, "issue-1") {
		t.Errorf("expected system prompt to contain substituted IssueID, got %q", out.SystemPrompt)
	}
	if !strings.Contains(out.SystemPrompt, "Plan") {
		t.Errorf("expected system prompt to reference Plan stage, got %q", out.SystemPrompt)
	}
	if !strings.Contains(out.SystemPrompt, "Implement auth") {
		t.Errorf("expected system prompt to contain substituted IssueTitle, got %q", out.SystemPrompt)
	}
}

// TestPlan_Registered confirms the Plan executor is wired into the
// stages registry under "loop_plan" and that it does NOT call
// backend.Execute (it returns a prompt, not a session). We pass a nil
// backend to prove the contract: the executor only touches the backend
// parameter to satisfy the signature.
func TestPlan_Registered(t *testing.T) {
	e, err := Resolve("loop_plan")
	if err != nil {
		t.Fatalf("loop_plan should be registered: %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil executor")
	}
	// nil backend is intentional — Plan must not dereference it.
	res, err := e(context.Background(), TaskRef{IssueID: "x"}, nilAgentBackend())
	if err != nil {
		t.Fatalf("executor: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.Output == "" {
		t.Error("expected Plan result to carry the loaded system prompt in Output")
	}
}

// nilAgentBackend returns a nil-typed agent.Backend for tests that
// verify the executor never invokes it. Calling any method on the
// returned value panics; the Plan executor must not.
func nilAgentBackend() agent.Backend {
	return nil
}
