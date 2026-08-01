package seed

// FixtureAnswers are deterministic strings used only to prove that each
// FixtureCase expected-test regex is syntactically valid and matchable.
var FixtureAnswers = map[string]string{
	"feat-001-cli-status-json":     `"version": "0.4.1"`,
	"feat-002-list-agents":         `"name": "Frontend Engineer"`,
	"feat-003-issue-create":        `id: 507f1f77-bcf8-4d79-9439-011507f1f77b`,
	"feat-004-metrics-row":         `metrics`,
	"feat-005-conventions":         `validate-agent-conventions`,
	"bug-001-typo-readme":          `README.en.md:45`,
	"bug-002-env-missing":          `MINIO_SERVER_URL`,
	"bug-003-status-error":         `status stopped`,
	"bug-004-fk-mismatch":          `REFERENCES issue(id)`,
	"bug-005-missing-test":         `internal/service/task.go`,
	"refactor-001-dead-flag":       `--poll-interval`,
	"refactor-002-duplicate-error": `handler.go:87`,
	"refactor-003-long-fn":         `func (s *TaskService) CompleteTask`,
	"refactor-004-any-type":        `interface{}`,
	"refactor-005-dead-import":     `import (`,
	"test-001-unit-cover":          `func TestRecordTaskMetric`,
	"test-002-mock-external":       `MockRoundTripper`,
	"test-003-e2e-fixture":         `TestApiClient`,
	"test-004-golden-count":        `4`,
	"test-005-coverage-baseline":   `test|coverage`,
	"docs-001-readme-updated":      `Quick Start`,
	"docs-002-changelog-missing":   `no`,
	"docs-003-license-file":        `Apache`,
	"docs-004-contributing":        `## Setup`,
	"docs-005-self-index":          `yes`,
}
