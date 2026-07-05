package seed

import (
	"github.com/agentra-ai/agentra/server/internal/eval"
)

// RegisterLookup wires the headless-mode answer lookup into the evaluator
// without creating a circular dependency. Call this once at process start
// (daemon startup, CLI init, test main) before running any eval.
func RegisterLookup() {
	eval.LookupAnswer = LookupAnswer
}
