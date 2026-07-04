// Package seed loads the default golden-issue dataset for v0. The dataset
// covers the 5 Agentra categories with enough surface to catch template
// regressions without bursting the time budget.

package seed // auto-generated dataset

import "github.com/agentra-ai/agentra/server/internal/eval"

// DefaultCases is the v0 golden dataset. Categories are interleaved so a
// regression that drops, say, all bug-fixing capability shows up immediately
// even if the overall score change is small.
var DefaultCases = []eval.GoldenIssue{
	// ---------------- feature ----------------
	{
		Slug:        "feat-001-cli-status-json",
		Category:    "feature",
		Title:       "`agentra daemon status` output",
		Description:  "Run `agentra daemon status --output json` and confirm the JSON contains a 'version' key. Report the version string.",
		ExpectedTest: `"version"`,
	},
	{
		Slug:        "feat-002-list-agents",
		Category:    "feature",
		Title:       "List agents",
		Description:  "Use `agentra agent list --output json` to count agents. Print the count and the first agent name.",
		ExpectedTest: `"name"`,
	},
	{
		Slug:        "feat-003-issue-create",
		Category:    "feature",
		Title:       "Create an issue",
		Description:  "Use `agentra issue create --title '[eval] dogfood smoke'` to create a test issue and print its ID.",
		ExpectedTest: `[0-9a-f]{8}-`,
	},
	{
		Slug:        "feat-004-metrics-row",
		Category:    "feature",
		Title:       "Metrics table exists",
		Description:  "Use `agentra --help` to confirm the metrics subcommand exists.",
		ExpectedTest: `metrics`,
	},
	{
		Slug:        "feat-005-conventions",
		Category:    "feature",
		Title:       "Conventions CLI",
		Description:  "Use `agentra conventions validate-agent-conventions --help` to confirm the subcommand works.",
		ExpectedTest: `validate-agent-conventions`,
	},

	// ---------------- bug ----------------
	{
		Slug:        "bug-001-typo-readme",
		Category:    "bug",
		Title:       "Fix typo in README",
		Description:  "README.md: README.zh-CN.md exists in the repo root. Use grep to find which README references 'README.zh-CN.md' without the .md extension. State the file and line number.",
		ExpectedTest: `README`,
	},
	{
		Slug:        "bug-002-env-missing",
		Category:    "bug",
		Title:       "Missing env var",
		Description:  "Inspect .env.example and list any variable that appears in docker-compose.yml but not in .env.example.",
		ExpectedTest: `[A-Z_]+`,
	},
	{
		Slug:        "bug-003-status-error",
		Category:    "bug",
		Title:       "Handle daemon not running",
		Description:  "Run `agentra daemon status` when no daemon is running. State the exit code or output.",
		ExpectedTest: `status`,
	},
	{
		Slug:        "bug-004-fk-mismatch",
		Category:    "bug",
		Title:       "Migration FK correctness",
		Description:  "Read server/migrations/039_agent_task_metrics.up.sql. Which table does the `issue_id` column reference?",
		ExpectedTest: `issue`,
	},
	{
		Slug:        "bug-005-missing-test",
		Category:    "bug",
		Title:       "Untested source file",
		Description:  "Find a .go file in server/internal/service that has no corresponding _test.go file. State the file path.",
		ExpectedTest: `internal`,
	},

	// ---------------- refactor ----------------
	{
		Slug:        "refactor-001-dead-flag",
		Category:    "refactor",
		Title:       "Unused CLI flag",
		Description:  "Search cmd/agentra for a flag defined but never referenced outside its registration. State the flag name and command.",
		ExpectedTest: `^\s*--`,
	},
	{
		Slug:        "refactor-002-duplicate-error",
		Category:    "refactor",
		Title:       "Duplicated error message",
		Description:  "Find two Go files in server/internal/handler with the same error message literal. State both files.",
		ExpectedTest: `\.go`,
	},
	{
		Slug:        "refactor-003-long-fn",
		Category:    "refactor",
		Title:       "Long function",
		Description:  "Find a Go function longer than 80 lines in server/internal. State the function name and file.",
		ExpectedTest: `func `,
	},
	{
		Slug:        "refactor-004-any-type",
		Category:    "refactor",
		Title:       "Avoid any",
		Description:  "Find any usage of `interface{}` in server/internal/handler/*.go that could be typed. State the file.",
		ExpectedTest: `interface`,
	},
	{
		Slug:        "refactor-005-dead-import",
		Category:    "refactor",
		Title:       "Unused import",
		Description:  "Find a Go file in server/internal/cli with an unused import. State the file.",
		ExpectedTest: `import`,
	},

	// ---------------- test ----------------
	{
		Slug:        "test-001-unit-cover",
		Category:    "test",
		Title:       "Add unit test",
		Description:  "Read server/internal/util/parse.go (or similar small util). Identify the #1 pure function missing test coverage and state the name.",
		ExpectedTest: `func Test`,
	},
	{
		Slug:        "test-002-mock-external",
		Category:    "test",
		Title:       "Mock best practice",
		Description:  "Read any *_test.go in server/internal/handler. Does it mock external deps only (not stdlib)? State the test file and mock pattern.",
		ExpectedTest: `mock|Mock`,
	},
	{
		Slug:        "test-003-e2e-fixture",
		Category:    "test",
		Title:       "E2E fixture pattern",
		Description:  "Read e2e/tests/*.spec.ts. What's the shared fixture for data setup? State the function name.",
		ExpectedTest: `createTestApi|TestApiClient`,
	},
	{
		Slug:        "test-004-golden-count",
		Category:    "test",
		Title:       "Golden dataset count",
		Description:  "Read server/internal/eval/seed/seeds.go. How many test+docs golden cases exist? State the number.",
		ExpectedTest: `(4|5)`,
	},
	{
		Slug:        "test-005-coverage-baseline",
		Category:    "test",
		Title:       "Coverage comment",
		Description:  "Read CLAUDE.md. Does it mandate unit test coverage for new code? Quote the relevant line.",
		ExpectedTest: `test|coverage`,
	},

	// ---------------- docs ----------------
	{
		Slug:        "docs-001-readme-updated",
		Category:    "docs",
		Title:       "README structure check",
		Description:  "Read README.md top section. Does it contain a 'Quick Start' heading? State yes or no.",
		ExpectedTest: `[Qq]uick [Ss]tart`,
	},
	{
		Slug:        "docs-002-changelog-missing",
		Category:    "docs",
		Title:       "CHANGELOG presence",
		Description:  "Does the repo have a CHANGELOG.md or CHANGELOG? State yes or no with path.",
		ExpectedTest: `(yes|no|CHANGELOG)`,
	},
	{
		Slug:        "docs-003-license-file",
		Category:    "docs",
		Title:       "LICENSE file",
		Description:  "What license is stated in LICENSE file? State the SPDX identifier.",
		ExpectedTest: `(Apache|MIT|BSD|ISC)`,
	},
	{
		Slug:        "docs-004-contributing",
		Category:    "docs",
		Title:       "CONTRIBUTING guide",
		Description:  "Run `grep -l 'setup' CONTRIBUTING.md` or explain what CONTRIBUTING.md describes. State the first H2 heading.",
		ExpectedTest: `#+ `,
	},
	{
		Slug:        "docs-005-self-index",
		Category:    "docs",
		Title:       "Docs index",
		Description:  "Does docs/ folder have an index.md? State yes or no.",
		ExpectedTest: `(yes|no|index)`,
	},
}
