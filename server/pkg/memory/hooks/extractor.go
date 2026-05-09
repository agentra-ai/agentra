package hooks

import (
	"context"
	"log/slog"

	"github.com/agentra-ai/agentra/server/pkg/memory"
)

type ExtractionConfig struct {
	TokenThreshold int
	StepThreshold  int
	ExtractOnError bool
}

func DefaultExtractionConfig() *ExtractionConfig {
	return &ExtractionConfig{
		TokenThreshold: 5000,
		StepThreshold:  50,
		ExtractOnError: true,
	}
}

// AgentTask represents a task for agent execution
type AgentTask struct {
	WorkspaceID string
	AgentID     string
	TotalTokens int
	TotalSteps  int
	Error       string
	Output      string
}

// ConfigurableExtractor checks if task meets threshold and extracts learnings.
func ConfigurableExtractor(ctx context.Context, svc *memory.MemoryService, task *AgentTask, config *ExtractionConfig) error {
	if config == nil {
		config = DefaultExtractionConfig()
	}

	shouldExtract := false

	// Check token threshold
	if task.TotalTokens > config.TokenThreshold {
		shouldExtract = true
	}

	// Check step threshold
	if task.TotalSteps > config.StepThreshold {
		shouldExtract = true
	}

	// Check error condition
	if config.ExtractOnError && task.Error != "" {
		shouldExtract = true
	}

	if !shouldExtract {
		return nil
	}

	// Extract and store learnings
	learnings := ExtractLearnings(task.Output)
	for _, learning := range learnings {
		err := svc.StoreAgentMemory(ctx, task.AgentID, task.WorkspaceID, memory.MemoryTypePattern, learning, true)
		if err != nil {
			slog.Error("failed to store learning", "error", err)
		}
	}

	return nil
}
