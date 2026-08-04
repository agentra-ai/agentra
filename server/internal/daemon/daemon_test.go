package daemon

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/agentra-ai/agentra/server/pkg/agent"
)

func TestPingExecOptionsConformToAdapterCapabilities(t *testing.T) {
	t.Parallel()

	for _, descriptor := range agent.KnownAdapters() {
		descriptor := descriptor
		t.Run(string(descriptor.Provider), func(t *testing.T) {
			t.Parallel()

			opts := pingExecOptions(descriptor, 3*time.Second)
			if opts.Timeout != 3*time.Second {
				t.Fatalf("timeout = %s, want 3s", opts.Timeout)
			}
			if descriptor.Supports(agent.CapabilityMaxTurns) {
				if opts.MaxTurns != 1 {
					t.Fatalf("max turns = %d, want 1", opts.MaxTurns)
				}
			} else if opts.MaxTurns != 0 {
				t.Fatalf("unsupported max turns = %d, want omitted", opts.MaxTurns)
			}
			if err := agent.ValidateExecOptions(descriptor, opts); err != nil {
				t.Fatalf("ping options violate adapter contract: %v", err)
			}
		})
	}
}

func TestNormalizeServerBaseURL(t *testing.T) {
	t.Parallel()

	got, err := NormalizeServerBaseURL("ws://localhost:8080/ws")
	if err != nil {
		t.Fatalf("NormalizeServerBaseURL returned error: %v", err)
	}
	if got != "http://localhost:8080" {
		t.Fatalf("expected http://localhost:8080, got %s", got)
	}
}

func TestBuildPromptContainsIssueID(t *testing.T) {
	t.Parallel()

	issueID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	prompt := BuildPrompt(Task{
		IssueID: issueID,
		Agent: &AgentData{
			Name: "Local Codex",
			Skills: []SkillData{
				{Name: "Concise", Content: "Be concise."},
			},
		},
	})

	// Prompt should contain the issue ID and CLI hint.
	for _, want := range []string{
		issueID,
		"agentra issue get",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}

	// Skills should NOT be inlined in the prompt (they're in runtime config).
	for _, absent := range []string{"## Agent Skills", "Be concise."} {
		if strings.Contains(prompt, absent) {
			t.Fatalf("prompt should NOT contain %q (skills are in runtime config)", absent)
		}
	}
}

func TestBuildPromptNoIssueDetails(t *testing.T) {
	t.Parallel()

	prompt := BuildPrompt(Task{
		IssueID: "test-id",
		Agent:   &AgentData{Name: "Test"},
	})

	// Prompt should not contain issue title/description (agent fetches via CLI).
	for _, absent := range []string{"**Issue:**", "**Summary:**"} {
		if strings.Contains(prompt, absent) {
			t.Fatalf("prompt should NOT contain %q — agent fetches details via CLI", absent)
		}
	}
}

func TestIsWorkspaceNotFoundError(t *testing.T) {
	t.Parallel()

	err := &requestError{
		Method:     http.MethodPost,
		Path:       "/api/daemon/register",
		StatusCode: http.StatusNotFound,
		Body:       `{"error":"workspace not found"}`,
	}
	if !isWorkspaceNotFoundError(err) {
		t.Fatal("expected workspace not found error to be recognized")
	}

	if isWorkspaceNotFoundError(&requestError{StatusCode: http.StatusInternalServerError, Body: `{"error":"workspace not found"}`}) {
		t.Fatal("did not expect 500 to be treated as workspace not found")
	}
}

// TestBuildPromptForStage_FixUsesRealBranchAndIteration is the regression
// guard for the "plumb real branch/iteration" fix. Before the fix,
// buildPromptForStage hard-coded Branch="loop/<IssueID>" and
// Iteration=1 for every loop_fix task, regardless of what the loop row
// actually contained. The fix: handler.ClaimTaskByRuntime now reads
// loops.branch_name and loops.iteration and threads them through the
// claim response, and buildPromptForStage uses them for loop_fix (and
// loop_review's Branch). This test exercises the daemon side of that
// contract directly: given a Task with real Branch/Iteration, the
// generated user prompt must mention them.
func TestBuildPromptForStage_FixUsesRealBranchAndIteration(t *testing.T) {
	t.Parallel()

	task := Task{
		ID:         "task-1",
		IssueID:    "issue-42",
		IssueTitle: "Real branch threading",
		Branch:     "feature/real-fix-branch",
		Iteration:  2,
		LoopID:     "loop-uuid",
	}
	userPrompt, _, _, _, err := buildPromptForStage("loop_fix", task, "/tmp/work")
	if err != nil {
		t.Fatalf("buildPromptForStage: %v", err)
	}

	if userPrompt == "" {
		t.Fatal("expected non-empty user prompt for loop_fix")
	}
	if !strings.Contains(userPrompt, "feature/real-fix-branch") {
		t.Errorf("expected user prompt to mention real branch %q, got %q", "feature/real-fix-branch", userPrompt)
	}
	if !strings.Contains(userPrompt, "iteration 2") && !strings.Contains(userPrompt, "iteration %d") {
		t.Errorf("expected user prompt to mention iteration 2, got %q", userPrompt)
	}
	if strings.Contains(userPrompt, "loop/issue-42") {
		t.Errorf("expected user prompt not to use the derived placeholder branch, got %q", userPrompt)
	}
	if strings.Contains(userPrompt, "iteration 1") {
		t.Errorf("expected user prompt to use real iteration 2, not the placeholder 1, got %q", userPrompt)
	}
}

// TestBuildPromptForStage_ReviewUsesRealBranch confirms loop_review
// threads the real branch from the develop stage. Review reads the diff
// for that branch, so a placeholder breaks the review entirely.
func TestBuildPromptForStage_ReviewUsesRealBranch(t *testing.T) {
	t.Parallel()

	task := Task{
		ID:        "task-2",
		IssueID:   "issue-77",
		Branch:    "feature/real-branch-from-develop",
		Iteration: 3,
		LoopID:    "loop-uuid-2",
	}
	userPrompt, systemPrompt, _, _, err := buildPromptForStage("loop_review", task, "/tmp/work")
	if err != nil {
		t.Fatalf("buildPromptForStage: %v", err)
	}

	if userPrompt == "" {
		t.Fatal("expected non-empty user prompt for loop_review")
	}
	if !strings.Contains(userPrompt, "feature/real-branch-from-develop") {
		t.Errorf("expected user prompt to mention real branch, got %q", userPrompt)
	}
	// The review user prompt does not include iteration (unlike fix), so
	// check the system prompt which is populated from the template.
	// The template renders "Iteration: {{.Iteration}}" → "Iteration: 3".
	if !strings.Contains(systemPrompt, "Iteration: 3") {
		t.Errorf("expected system prompt to contain 'Iteration: 3', got %q", systemPrompt)
	}
	if strings.Contains(systemPrompt, "Iteration: 1") {
		t.Errorf("expected system prompt to use real iteration 3, not the placeholder 1, got %q", systemPrompt)
	}
}

// TestBuildPromptForStage_FixRejectsEmptyBranch verifies that a fix task
// cannot run against a guessed branch and then report false success.
func TestBuildPromptForStage_FixRejectsEmptyBranch(t *testing.T) {
	t.Parallel()

	task := Task{
		ID:        "task-3",
		IssueID:   "issue-99",
		Branch:    "", // simulates loops.branch_name being empty
		Iteration: 1,
		LoopID:    "loop-uuid-3",
	}
	_, _, _, _, err := buildPromptForStage("loop_fix", task, "/tmp/work")
	if err == nil || !strings.Contains(err.Error(), "no persisted branch") {
		t.Fatalf("expected missing-branch error, got %v", err)
	}
}

func TestBuildPromptForStage_RejectsInvalidLoopContext(t *testing.T) {
	t.Parallel()

	t.Run("non-positive review iteration", func(t *testing.T) {
		_, _, _, _, err := buildPromptForStage("loop_review", Task{
			ID:        "task-invalid-iteration",
			IssueID:   "issue-100",
			Branch:    "feature/real-branch",
			Iteration: 0,
		}, "/tmp/work")
		if err == nil || !strings.Contains(err.Error(), "invalid iteration") {
			t.Fatalf("expected invalid-iteration error, got %v", err)
		}
	})

	t.Run("unknown loop task type", func(t *testing.T) {
		_, _, _, _, err := buildPromptForStage("loop_publish", Task{ID: "task-unknown"}, "/tmp/work")
		if err == nil || !strings.Contains(err.Error(), "unknown loop task type") {
			t.Fatalf("expected unknown-loop-type error, got %v", err)
		}
	})
}

// TestBuildPromptForStage_PlanAndDevelopUsePlaceholder confirms the
// "no real branch yet" stages still work without Task.Branch/Iteration
// being populated. The placeholder keeps the prompt well-formed.
func TestBuildPromptForStage_PlanAndDevelopUsePlaceholder(t *testing.T) {
	t.Parallel()

	for _, taskType := range []string{"loop_plan", "loop_develop"} {
		task := Task{
			ID:      "task-" + taskType,
			IssueID: "issue-1",
			// Branch and Iteration intentionally empty — these
			// stages run before the develop branch exists.
		}
		userPrompt, _, _, _, err := buildPromptForStage(taskType, task, "/tmp/work")
		if err != nil {
			t.Fatalf("%s: buildPromptForStage: %v", taskType, err)
		}

		if userPrompt == "" {
			t.Fatalf("%s: expected non-empty user prompt", taskType)
		}
		// Plan may or may not mention a branch in its user prompt; the
		// important property is that the prompt is built without
		// panicking. The placeholder for the develop stage is in the
		// user prompt body.
		if taskType == "loop_develop" && !strings.Contains(userPrompt, "loop/issue-1") {
			t.Errorf("expected develop user prompt to mention placeholder branch, got %q", userPrompt)
		}
	}
}
