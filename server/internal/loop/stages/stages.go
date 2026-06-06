// Package stages contains per-stage executors for the Agentic Engineering
// Loop. Real implementations land in Task 9 (stages package skeleton + prompt
// templates). This file is the stub the daemon's buildPromptForStage imports
// in Task 8; Task 9 replaces the body with the real registry + prompt loader.
package stages

import (
	"github.com/agentra-ai/agentra/server/pkg/agent"
)

// TaskRef is the subset of daemon.Task that stages need. Defined here (not
// imported from the daemon package) to avoid an import cycle: daemon imports
// stages, so stages cannot import daemon. The daemon's buildPromptForStage
// helper populates this struct before calling into stages.
type TaskRef struct {
	ID         string
	IssueID    string
	IssueTitle string
	Branch     string
	Iteration  int
	WorkDir    string
}

// LoopStagePrompt returns the system prompt for a given stage. Stub for Task 8;
// Task 9 will load it from prompts/{stage}.md via embed.
func LoopStagePrompt(stage string, _ *TaskRef) string {
	_ = stage
	return "" // Task 9: load from embedded template
}

// LoopToolsByStage returns the tool names available to a given stage. Stub for
// Task 8; Task 9 will return the real per-stage tool set.
func LoopToolsByStage(_ string) []string {
	return nil // Task 9: return real tool set
}

// _ ensures agent is used (it'll be wired in Task 9).
var _ = agent.ExecOptions{}
