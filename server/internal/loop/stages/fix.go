package stages

import (
	"context"
	"fmt"

	"github.com/agentra-ai/agentra/server/pkg/agent"
)

// FixPrompt is what the Fix stage returns to the daemon — prompt text,
// tool set, and turn cap. Mirrors PlanPrompt / DevelopPrompt /
// ReviewPrompt: the daemon uses this to construct its ExecOptions and
// call backend.Execute. The split: stages own prompt construction; the
// daemon owns the actual LLM call.
type FixPrompt struct {
	SystemPrompt string
	UserPrompt   string
	Tools        []string
	MaxTurns     int
}

// BuildFixPrompt loads the fix template from the embedded FS, substitutes
// TaskRef fields, and packages the result with the Fix stage's tool set
// and turn cap. Pure function — no I/O, no agent backend calls.
//
// MaxTurns is 20: Fix is a focused, single-purpose iteration on a known
// branch (read review, edit, test, commit, push). It is slightly under
// Develop's 25 (Develop is the cold-start path with more exploration) and
// double Review's 10 (Review is read-only and short).
//
// The user prompt explicitly includes the iteration number so the model
// can tell this fix attempt from a prior one. The loop coordinator
// already bumps the iteration in decideNextStage
// (TestDecideNextStage_ReviewRejectedToFix pins the contract); we just
// surface it to the model.
func BuildFixPrompt(task TaskRef) (*FixPrompt, error) {
	systemPrompt, err := loadPrompt("fix", task)
	if err != nil {
		return nil, fmt.Errorf("fix: load prompt: %w", err)
	}
	userPrompt := fmt.Sprintf("Address the review feedback on branch %s (iteration %d).", task.Branch, task.Iteration)
	return &FixPrompt{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Tools:        toolsForStage("loop_fix"),
		MaxTurns:     20,
	}, nil
}

// Fix is the registry-bound executor for the Fix stage. It loads the fix
// template, populates a FixPrompt, and wraps the system prompt in the
// stages.Result envelope. It does NOT call backend.Execute — the daemon
// is responsible for that. This split keeps stages free of agent-backend
// coupling and testable in isolation (see TestFix_Registered which passes
// a nil backend).
//
// Fix reuses the develop stage's branch and PR — it is never the first
// stage in a loop, and its tool set (loop_fix) intentionally omits
// create_branch and github_pr_create. See TestFixPrompt_DoesNotCreateBranch
// for the regression guard.
func Fix(_ context.Context, task TaskRef, _ agent.Backend) (*Result, error) {
	p, err := BuildFixPrompt(task)
	if err != nil {
		return nil, err
	}
	return &Result{Output: p.SystemPrompt}, nil
}

func init() {
	Register("loop_fix", Fix)
}
