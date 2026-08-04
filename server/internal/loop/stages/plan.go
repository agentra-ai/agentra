package stages

import (
	"context"
	"fmt"

	"github.com/agentra-ai/agentra/server/pkg/agent"
)

// PlanPrompt is what the Plan stage returns to the daemon — prompt text,
// tool set, and turn cap. The daemon uses this to construct its
// ExecOptions and call backend.Execute. The split: stages own prompt
// construction; the daemon owns the actual LLM call.
type PlanPrompt struct {
	SystemPrompt string
	UserPrompt   string
	Tools        []string
	MaxTurns     int
}

// BuildPlanPrompt loads the plan template from the embedded FS,
// substitutes TaskRef fields, and packages the result with the Plan
// stage's tool set and turn cap. Pure function — no I/O, no agent
// backend calls.
func BuildPlanPrompt(task TaskRef) (*PlanPrompt, error) {
	systemPrompt, err := loadPrompt("plan", task)
	if err != nil {
		return nil, fmt.Errorf("plan: load prompt: %w", err)
	}
	userPrompt := fmt.Sprintf("Issue: %s\n\n%s", task.IssueTitle, task.IssueDescription)
	return &PlanPrompt{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Tools:        toolsForStage("loop_plan"),
		MaxTurns:     maxTurnsForStage("loop_plan"),
	}, nil
}

// Plan is the registry-bound executor for the Plan stage. It loads the
// plan template, populates a PlanPrompt, and wraps the system prompt in
// the stages.Result envelope. It does NOT call backend.Execute — the
// daemon is responsible for that. This split keeps stages free of
// agent-backend coupling and testable in isolation (see TestPlan_Registered
// which passes a nil backend).
func Plan(_ context.Context, task TaskRef, _ agent.Backend) (*Result, error) {
	p, err := BuildPlanPrompt(task)
	if err != nil {
		return nil, err
	}
	return &Result{Output: p.SystemPrompt}, nil
}

func init() {
	Register("loop_plan", Plan)
}
