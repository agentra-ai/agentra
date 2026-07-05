package eval

import (
	"context"
	"regexp"
	"time"
)

// LookupAnswer is a package-level func var so the seed package can register
// its answer lookup without a circular import (eval ↔ seed).
// Callers must call seed.RegisterLookup() before using Evaluator.RunHeadless.
var LookupAnswer func(slug string) string

func init() {
	LookupAnswer = func(slug string) string { return "" }
}

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
	r, err := e.CompileExpected(gc)
	if err != nil || r == nil {
		return 0
	}
	if r.MatchString(output) {
		return 1.0
	}
	return 0
}

// RunHeadless runs eval against the golden dataset without spawning an
// agent. It scores each golden case against pre-computed canned answers
// exported from seed.DefaultAnswers (v0 smoke test only — real runs hook
// into the daemon via Evaluator.Runner).
func (e *Evaluator) RunHeadless(ctx context.Context, cases []GoldenIssue) RunReport {
	report := RunReport{Total: len(cases)}
	for _, c := range cases {
		output := LookupAnswer(c.Slug)
		score := e.Score(c, output)
		cr := CaseResult{
			Slug:     c.Slug,
			Category: c.Category,
			Passed:   score >= 0.5,
			Score:    score,
		}
		if !cr.Passed {
			cr.Error = "score < 0.5 in headless mode"
			report.Failed++
		} else {
			report.Passed++
		}
		report.Cases = append(report.Cases, cr)
	}
	if report.Total > 0 {
		report.Score = float64(report.Passed) / float64(report.Total) * 100.0
	}
	return report
}
