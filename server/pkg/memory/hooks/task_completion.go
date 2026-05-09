package hooks

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/agentra-ai/agentra/server/pkg/memory"
)

// ExtractLearnings extracts key learnings from task output.
func ExtractLearnings(output string) []string {
	var learnings []string

	// Simple pattern-based extraction
	// Look for lines starting with "-", "*", or numbered patterns
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			learnings = append(learnings, strings.TrimPrefix(trimmed, "- "))
		}
	}

	// If no structured learnings found, extract last paragraph as summary
	if len(learnings) == 0 && len(output) > 100 {
		paragraphs := strings.Split(output, "\n\n")
		if len(paragraphs) > 0 {
			lastParagraph := strings.TrimSpace(paragraphs[len(paragraphs)-1])
			if len(lastParagraph) > 50 && len(lastParagraph) < 500 {
				learnings = append(learnings, lastParagraph)
			}
		}
	}

	return learnings
}

// TaskResult represents the result of a completed task.
type TaskResult struct {
	Output string
}

type TaskCompletionHook struct {
	memorySvc *memory.MemoryService
}

func NewTaskCompletionHook(ms *memory.MemoryService) *TaskCompletionHook {
	return &TaskCompletionHook{memorySvc: ms}
}

// OnTaskComplete extracts learnings from completed task and stores them.
func (h *TaskCompletionHook) OnTaskComplete(ctx context.Context, taskAgentID, taskIssueID uuid.UUID, workspaceID string, result string) error {
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
			taskAgentID.String(),
			workspaceID,
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
		taskAgentID.String(),
		workspaceID,
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

// OnTaskComplete extracts learnings from completed task and stores them.
func OnTaskComplete(ctx context.Context, svc *memory.MemoryService, task *AgentTask, result *TaskResult) error {
	learnings := ExtractLearnings(result.Output)

	for _, learning := range learnings {
		_, err := svc.StoreAgentMemory(ctx, task.AgentID, task.WorkspaceID, memory.MemoryTypeLearning, learning, true)
		if err != nil {
			return err
		}
	}

	return nil
}