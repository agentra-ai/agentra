package stages

import (
	"context"
	"fmt"

	"github.com/agentra-ai/agentra/server/pkg/agent"
)

// DevelopPrompt is what the Develop stage returns to the daemon — prompt
// text, tool set, and turn cap. Mirrors PlanPrompt: the daemon uses this
// to construct its ExecOptions and call backend.Execute. The split:
// stages own prompt construction; the daemon owns the actual LLM call.
type DevelopPrompt struct {
	SystemPrompt string
	UserPrompt   string
	Tools        []string
	MaxTurns     int
}

// BuildDevelopPrompt loads the develop template from the embedded FS,
// substitutes TaskRef fields, and packages the result with the Develop
// stage's tool set and turn cap. Pure function — no I/O, no agent
// backend calls.
//
// MaxTurns is generous (25) because the develop stage typically iterates
// through edit / run-test / fix-test cycles before pushing a branch and
// opening a PR. The plan stage caps at 5 because it only inspects.
func BuildDevelopPrompt(task TaskRef) (*DevelopPrompt, error) {
	systemPrompt, err := loadPrompt("develop", task)
	if err != nil {
		return nil, fmt.Errorf("develop: load prompt: %w", err)
	}
	userPrompt := fmt.Sprintf("Branch: %s\nIssue: %s\n\nBegin implementation.", task.Branch, task.IssueTitle)
	return &DevelopPrompt{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		Tools:        toolsForStage("loop_develop"),
		MaxTurns:     25,
	}, nil
}

// Develop is the registry-bound executor for the Develop stage. It loads
// the develop template, populates a DevelopPrompt, and wraps the system
// prompt in the stages.Result envelope. It does NOT call backend.Execute
// — the daemon is responsible for that. This split keeps stages free of
// agent-backend coupling and testable in isolation (see TestDevelop_Registered
// which passes a nil backend).
//
// PR URL extraction is intentionally NOT done here — the Review stage
// already parses the develop output for the PR URL, and duplicating that
// here would create two sources of truth.
func Develop(_ context.Context, task TaskRef, _ agent.Backend) (*Result, error) {
	p, err := BuildDevelopPrompt(task)
	if err != nil {
		return nil, err
	}
	return &Result{Output: p.SystemPrompt}, nil
}

func init() {
	Register("loop_develop", Develop)
}
