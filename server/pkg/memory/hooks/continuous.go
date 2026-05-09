package hooks

import (
	"context"
	"strings"

	"github.com/agentra-ai/agentra/server/pkg/memory"
)

// ShouldCapture determines if a trace step contains valuable memory content.
func ShouldCapture(step *TraceStep) bool {
	// Capture tool results that look like learnings
	if step.Action == "tool_result" {
		output := step.OutputText
		// Keywords that suggest valuable information
		keywords := []string{"remember", "note", "important", "use", "avoid", "don't", "always", "never"}
		for _, kw := range keywords {
			if strings.Contains(output, kw) {
				return true
			}
		}
		// Long outputs (>500 chars) with code patterns might be useful
		if len(output) > 500 && (strings.Contains(output, "function") || strings.Contains(output, "class")) {
			return true
		}
	}
	return false
}

// TraceStep represents a step in agent execution
type TraceStep struct {
	WorkspaceID string
	AgentID     string
	StepNumber  int
	Action      string
	Tool        string
	InputText   string
	OutputText  string
	Timestamp   string
}

// ContinuousCapture stores valuable insights from ongoing task execution.
func ContinuousCapture(ctx context.Context, svc *memory.MemoryService, step *TraceStep) error {
	if !ShouldCapture(step) {
		return nil
	}

	return svc.StoreAgentMemory(ctx, step.AgentID, step.WorkspaceID, memory.MemoryTypeContext, step.OutputText, true)
}
