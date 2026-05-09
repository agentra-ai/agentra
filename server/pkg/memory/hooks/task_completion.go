package hooks

import (
	"context"
	"strings"

	"github.com/agentra-ai/agentra/server/pkg/memory"
	"github.com/agentra-ai/agentra/server/pkg/db/generated"
)

type TaskCompletionHook struct {
	memorySvc *memory.MemoryService
}

func NewTaskCompletionHook(ms *memory.MemoryService) *TaskCompletionHook {
	return &TaskCompletionHook{memorySvc: ms}
}

// OnTaskComplete extracts learnings from completed task and stores them.
func (h *TaskCompletionHook) OnTaskComplete(ctx context.Context, task *db.AgentTaskQueue, result string) error {
	// Skip if no result content
	if result == "" {
		return nil
	}

	// Extract potential learnings from the result
	// Simple heuristic: look for patterns like "learned", "important", "note:", etc.
	lines := strings.Split(result, "\n")
	var learnings []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			learnings = append(learnings, line[2:])
		}
	}

	// If we found structured learnings, store them
	if len(learnings) > 0 {
		content := strings.Join(learnings, "; ")
		_, err := h.memorySvc.StoreAgentMemory(
			ctx,
			task.AgentID.String(),
			task.WorkspaceID.String(),
			memory.MemoryTypeLearning,
			content,
			true,
		)
		if err != nil {
			return err
		}
	}

	// Also store the raw task result as task_result type
	_, err := h.memorySvc.StoreAgentMemory(
		ctx,
		task.AgentID.String(),
		task.WorkspaceID.String(),
		memory.MemoryTypeTaskResult,
		truncate(result, 1000),
		true,
	)
	return err
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}