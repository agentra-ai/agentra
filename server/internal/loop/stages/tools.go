package stages

import (
	"fmt"
	"strings"

	"github.com/agentra-ai/agentra/server/pkg/agent"
)

type executionPolicy struct {
	tools    []string
	maxTurns int
}

var executionPolicies = map[string]executionPolicy{
	"loop_plan": {
		tools:    []string{"read_file", "search_code"},
		maxTurns: 5,
	},
	"loop_review": {
		tools:    []string{"read_file", "search_code", "git_diff"},
		maxTurns: 10,
	},
	"loop_develop": {
		tools: []string{
			"read_file", "search_code", "write_file",
			"run_command", "run_test",
			"git_status", "git_diff", "git_commit", "git_push",
			"create_branch", "github_pr_create",
		},
		maxTurns: 25,
	},
	"loop_fix": {
		tools: []string{
			"read_file", "search_code", "write_file",
			"run_command", "run_test",
			"git_status", "git_diff", "git_commit", "git_push",
		},
		maxTurns: 20,
	},
}

// toolsForStage returns the tool names available to a given stage. The
// underlying tool implementations live in server/internal/loop/tools/ —
// this function only enumerates which tool names a stage is allowed to
// call. The daemon resolves the names to concrete tools.Tool values when
// it builds ExecOptions for backend.Execute.
//
// Per-stage rationale:
//   - loop_plan: read-only inspection (no diff context yet — plan runs
//     before there is a branch).
//   - loop_review: read-only inspection PLUS git_diff, because the
//     reviewer needs to see the develop stage's changes.
//   - loop_develop: full mutating set — edit files, run commands and
//     tests, manage a branch, push, and open a PR.
//   - loop_fix: same as develop minus create_branch and github_pr_create.
//     Fix iterations push to the existing branch and update the existing
//     PR, so they have no business opening new ones.
//
// Unknown task types get nil — the daemon falls back to whatever tools
// the spawned agent CLI exposes by default, which is the safe baseline.
func toolsForStage(taskType string) []string {
	policy, ok := executionPolicies[taskType]
	if !ok {
		return nil
	}
	return append([]string(nil), policy.tools...)
}

func maxTurnsForStage(taskType string) int {
	return executionPolicies[taskType].maxTurns
}

// ValidateAdapterForTaskType applies the same execution policy used by the
// daemon before a queued task is dispatched. Unknown loop stages fail closed;
// standard and legacy non-loop task types have no stage-specific options.
func ValidateAdapterForTaskType(descriptor agent.AdapterDescriptor, taskType string) error {
	taskType = strings.TrimSpace(taskType)
	if taskType == "" || taskType == "standard" {
		return nil
	}
	policy, ok := executionPolicies[taskType]
	if !ok {
		if strings.HasPrefix(taskType, "loop_") {
			return fmt.Errorf("unsupported loop task type %q", taskType)
		}
		return nil
	}
	if err := agent.ValidateExecOptions(descriptor, agent.ExecOptions{
		SystemPrompt: "engineering-loop-stage",
		MaxTurns:     policy.maxTurns,
		Tools:        append([]string(nil), policy.tools...),
	}); err != nil {
		return fmt.Errorf("task type %q: %w", taskType, err)
	}
	return nil
}
