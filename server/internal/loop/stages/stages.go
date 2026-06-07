// Package stages contains the per-stage executors for the Agentic
// Engineering Loop. The four canonical stages — Plan, Develop, Review,
// Fix — are registered against task_type strings (loop_plan,
// loop_develop, loop_review, loop_fix) and resolved when the daemon
// dispatches a task.
//
// Task 9 lands the package skeleton, the registry, the per-stage prompt
// loader, and the four prompt templates. Real Executor bodies (one per
// stage) land in Tasks 10-15.
package stages

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

// ErrUnknownStage is returned by Resolve when no Executor is registered for
// the requested task type.
var ErrUnknownStage = errors.New("stages: unknown stage")

// TaskRef is the subset of daemon.Task that stages need. Defined here (not
// imported from the daemon package) to avoid an import cycle: daemon
// imports stages, so stages cannot import daemon. The daemon's
// buildPromptForStage helper populates this struct before calling into
// stages.
type TaskRef struct {
	ID         string
	IssueID    string
	IssueTitle string
	Branch     string
	Iteration  int
	WorkDir    string
}

// Result is the outcome of a single stage execution. Real executors
// populate it as the stage runs; consumers (e.g. the loop Coordinator)
// read the Status and Output to decide the next state.
type Result struct {
	Status  string // "completed", "blocked", "needs_fix", etc.
	Output  string // free-form text the executor produced
	Branch  string // populated when the executor created or moved a branch
	PRURL   string // populated when the executor opened a PR
	PRNum   int    // populated alongside PRURL
	Iter    int    // iteration number, mirrors TaskRef.Iteration for convenience
	RawJSON []byte // optional structured output (e.g. the review stage's verdict)
}

// Executor runs a single stage for a given task. The exact signature will
// grow in later tasks as we add the real Plan/Develop/Review/Fix bodies;
// the TaskRef parameter is stable.
type Executor func(ref *TaskRef) (Result, error)

// registry maps task_type strings (loop_plan, loop_develop, loop_review,
// loop_fix) to the Executor that implements that stage.
var (
	registryMu sync.RWMutex
	registry   = map[string]Executor{}
)

// Register attaches an Executor to a task type. Intended to be called from
// package init() functions when real executor implementations land in
// Tasks 10-15. Registering the same task type twice panics — that is a
// programmer error caught at startup, not a runtime fallback.
func Register(taskType string, e Executor) {
	if taskType == "" {
		panic("stages: Register requires non-empty taskType")
	}
	if e == nil {
		panic("stages: Register requires non-nil Executor for " + taskType)
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[taskType]; exists {
		panic("stages: duplicate registration for " + taskType)
	}
	registry[taskType] = e
}

// Resolve returns the Executor registered for the given task type, or
// ErrUnknownStage if no executor is registered.
func Resolve(taskType string) (Executor, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	e, ok := registry[taskType]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownStage, taskType)
	}
	return e, nil
}

// AllRegistered returns the set of registered task types. Returned slice
// is a fresh copy — callers may mutate it safely.
func AllRegistered() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}

// LoopStagePrompt returns the loaded and substituted system prompt for a
// given stage. On any failure (missing template, nil ref) it logs a
// warning and returns an empty string — the daemon falls back to its
// legacy BuildPrompt output in that case.
func LoopStagePrompt(stage string, ref *TaskRef) string {
	if ref == nil {
		return ""
	}
	p, err := loadPrompt(stage, *ref)
	if err != nil {
		slog.Warn("stages: load prompt failed", "stage", stage, "err", err)
		return ""
	}
	return p
}

// LoopToolsByStage returns the tool names available to a given stage.
// Task 9 returns nil — per-stage tool wiring lands in Tasks 11-12, when
// the read_file/write_file/search_code/run_command/run_test and
// git_*/create_pr tools come online.
func LoopToolsByStage(_ string) []string {
	return nil
}
