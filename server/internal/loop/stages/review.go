package stages

import (
	"context"
	"fmt"

	"github.com/agentra-ai/agentra/server/pkg/agent"
)

// ReviewPrompt is what the Review stage returns to the daemon — prompt
// text, tool set, and turn cap. Mirrors PlanPrompt / DevelopPrompt: the
// daemon uses this to construct its ExecOptions and call backend.Execute.
// The split: stages own prompt construction; the daemon owns the actual
// LLM call.
type ReviewPrompt struct {
	SystemPrompt string
	UserPrompt   string
	Tools        []string
	MaxTurns     int
}

// BuildReviewPrompt loads the review template from the embedded FS,
// substitutes TaskRef fields, and packages the result with the Review
// stage's tool set and turn cap. Pure function — no I/O, no agent
// backend calls.
//
// MaxTurns is 10: review is read-only (read_file, search_code, git_diff
// per toolsForStage) and the model mostly reads the diff and emits a
// short JSON verdict. The plan stage caps at 5 because it has no diff to
// inspect; develop caps at 25 because it iterates edit/test/fix cycles.
func BuildReviewPrompt(task TaskRef) (*ReviewPrompt, error) {
	systemPrompt, err := loadPrompt("review", task)
	if err != nil {
		return nil, fmt.Errorf("review: load prompt: %w", err)
	}
	userPrompt := fmt.Sprintf("Review the diff for branch %s against the original plan for issue %s.", task.Branch, task.IssueTitle)
	return &ReviewPrompt{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Tools:        toolsForStage("loop_review"),
		MaxTurns:     10,
	}, nil
}

// Review is the registry-bound executor for the Review stage. It loads
// the review template, populates a ReviewPrompt, and wraps the system
// prompt in the stages.Result envelope. It does NOT call backend.Execute
// — the daemon is responsible for that. This split keeps stages free of
// agent-backend coupling and testable in isolation (see
// TestReview_Registered which passes a nil backend).
//
// The review stage emits a JSON verdict (review_approved, review_issues,
// pr_url, pr_number, branch_name) that the loop coordinator's
// parseTaskResult decodes into a loop.TaskResult. JSON validation lives
// in the coordinator, not here — the stage's only job is to build a
// prompt that asks the LLM for the right field names.
func Review(_ context.Context, task TaskRef, _ agent.Backend) (*Result, error) {
	p, err := BuildReviewPrompt(task)
	if err != nil {
		return nil, err
	}
	return &Result{Output: p.SystemPrompt}, nil
}

func init() {
	Register("loop_review", Review)
}
