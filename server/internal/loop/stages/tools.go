package stages

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
	switch taskType {
	case "loop_plan":
		return []string{"read_file", "search_code"}
	case "loop_review":
		return []string{"read_file", "search_code", "git_diff"}
	case "loop_develop":
		return []string{
			"read_file", "search_code", "write_file",
			"run_command", "run_test",
			"git_status", "git_diff", "git_commit", "git_push",
			"create_branch", "github_pr_create",
		}
	case "loop_fix":
		return []string{
			"read_file", "search_code", "write_file",
			"run_command", "run_test",
			"git_status", "git_diff", "git_commit", "git_push",
		}
	}
	return nil
}
