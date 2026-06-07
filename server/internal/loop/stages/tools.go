package stages

// toolsForStage returns the tool names available to a given stage. Stub
// for Task 10; Task 11 expands this to the real per-stage tool set as
// read_file / write_file / search_code / run_command / run_test and the
// git_* / create_pr tools come online.
//
// Read-only stages (plan, review) get the read tools. Mutating stages
// (develop, fix) get the full set. Unknown task types get nothing — the
// daemon will fall back to whatever tools the spawned agent CLI exposes
// by default, which is the safe baseline.
func toolsForStage(taskType string) []string {
	switch taskType {
	case "loop_plan", "loop_review":
		return []string{"read_file", "search_code"}
	case "loop_develop", "loop_fix":
		return []string{"read_file", "write_file", "search_code", "run_command", "run_test"}
	}
	return nil
}
