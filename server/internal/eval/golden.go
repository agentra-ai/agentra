// Package eval provides the Agentra-Eval benchmark harness.
//
// Golden issues are stored in eval_golden_issues (per-workspace). Eval runs
// execute each case through the agent, score the outcome against the expected
// test regex, and detect regressions vs. the previous run's composite score.

package eval

import "time"

// GoldenIssue is a single benchmark case. It lives in the DB so operators
// can extend it without recompiling.
type GoldenIssue struct {
	ID           string
	Slug         string
	Category     string
	WorkspaceID  string
	IssueID      *string
	Title        string
	Description  string
	ExpectedTest string
	MaxDuration  time.Duration
}

// CaseResult is the per-case score for one golden issue run.
type CaseResult struct {
	Slug     string  `json:"slug"`
	Category string  `json:"category"`
	Passed   bool    `json:"passed"`
	Duration int64   `json:"duration_ms"`
	Score    float64 `json:"score"`
	Error    string  `json:"error,omitempty"`
}

// RunReport is persisted to eval_runs.summary.
type RunReport struct {
	Cases   []CaseResult `json:"cases"`
	Total   int          `json:"total"`
	Passed  int          `json:"passed"`
	Failed  int          `json:"failed"`
	Score   float64      `json:"score"`
}

// Score weights. Equal weight to start — per-category weighting is a v1 TODO.
const (
	weightDiffSimilarity = 0.4
	weightTestsPass      = 0.35
	weightLintClean      = 0.25
)
