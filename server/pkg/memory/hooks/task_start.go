package hooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentra-ai/agentra/server/pkg/memory"
)

type TaskStartHook struct {
	memorySvc   *memory.MemoryService
	injectLimit int
}

func NewTaskStartHook(ms *memory.MemoryService, injectLimit int) *TaskStartHook {
	if injectLimit <= 0 {
		injectLimit = 5
	}
	return &TaskStartHook{memorySvc: ms, injectLimit: injectLimit}
}

// BuildMemoryContext builds the RAG context string to inject into system prompt.
// query should be the issue title + description + skill instructions.
func (h *TaskStartHook) BuildMemoryContext(ctx context.Context, agentID, workspaceID, query string) (string, error) {
	results, err := h.memorySvc.RecallAgentMemories(ctx, agentID, workspaceID, query, h.injectLimit, nil)
	if err != nil {
		return "", fmt.Errorf("recall memories: %w", err)
	}

	if len(results.Memories) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString("\n\n=== Relevant Memories ===\n")
	for _, m := range results.Memories {
		agentNote := ""
		if m.AgentID != "" {
			agentNote = fmt.Sprintf(" (from agent:%s)", m.AgentID)
		}
		b.WriteString(fmt.Sprintf("- [%s] %s%s\n", m.MemoryType, m.Content, agentNote))
	}
	b.WriteString("===\n")
	return b.String(), nil
}