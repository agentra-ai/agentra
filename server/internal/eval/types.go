package eval

import (
	"context"
	"regexp"
	"time"

	"github.com/agentra-ai/agentra/server/pkg/codex/dna"
)

// Evaluator orchestrates one benchmark run over the golden dataset.
type Evaluator struct {
	// Runner executes a single case in isolation. The production implementation
	// claims an agent from the daemon + runs the task; the no-op test
	// implementation returns a deterministic pass.
	Runner func(ctx context.Context, gc GoldenIssue) (string, time.Duration, error)

	// Headless means "no daemon/deps available" — the evaluator skips real
	// agent runs and produces a smoke report that exercises scoring only.
	Headless bool

	// WorkspaceRoot is where dna.Extract looks for signals to inject.
	WorkspaceRoot string

	// Regex cache (compiled once per eval run).
	cache map[string]*regexp.Regexp
}

// New constructs an Evaluator with safe defaults.
func New(root string) *Evaluator {
	return &Evaluator{
		Headless:      true, // flip to false once a daemon is reachable
		WorkspaceRoot: root,
		cache:         map[string]*regexp.Regexp{},
	}
}

// CompileExpected caches + compiles the expected-test regex for a case.
func (e *Evaluator) CompileExpected(gc GoldenIssue) (*regexp.Regexp, error) {
	if r, ok := e.cache[gc.Slug]; ok || gc.ExpectedTest == "" {
		return r, nil
	}
	r, err := regexp.Compile(gc.ExpectedTest)
	if err != nil {
		return nil, err
	}
	e.cache[gc.Slug] = r
	return r, nil
}

// Score returns 0-1 based on whether the agent output matches expected test.
func (e *Evaluator) Score(gc GoldenIssue, output string) float64 {
	r, err := e.compileExpectedHelper(gc)
	if err != nil || r == nil {
		return 0
	}
	if r.MatchString(output) {
		return 1.0
	}
	return 0
}
