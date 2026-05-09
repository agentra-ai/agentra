package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemoryService_StoreAgentMemory_Signature(t *testing.T) {
	// Verify the method exists and has correct signature
	svc := &MemoryService{}
	// This is a compile-time check essentially
	_ = func(ctx context.Context, agentID, workspaceID string, memType MemoryType, content string, isPrivate bool) (*StoreResult, error) {
		return svc.StoreAgentMemory(ctx, agentID, workspaceID, memType, content, isPrivate)
	}
}

func TestMemoryService_RecallAgentMemories_Signature(t *testing.T) {
	svc := &MemoryService{}
	_ = func(ctx context.Context, agentID, workspaceID, query string, limit int, memTypes []string) (*RecallResult, error) {
		return svc.RecallAgentMemories(ctx, agentID, workspaceID, query, limit, memTypes)
	}
}

func TestMemoryType_Constants(t *testing.T) {
	assert.Equal(t, MemoryType("learning"), MemoryTypeLearning)
	assert.Equal(t, MemoryType("task_result"), MemoryTypeTaskResult)
	assert.Equal(t, MemoryType("context"), MemoryTypeContext)
	assert.Equal(t, MemoryType("pattern"), MemoryTypePattern)
}

func TestMemoryEntry_Struct(t *testing.T) {
	entry := MemoryEntry{
		ID:         "test-id",
		MemoryType: MemoryTypeLearning,
		Content:    "test content",
		AgentID:    "agent-1",
		Score:      0.95,
		CreatedAt:  "2026-01-01T00:00:00Z",
	}
	assert.Equal(t, "test-id", entry.ID)
	assert.Equal(t, MemoryTypeLearning, entry.MemoryType)
	assert.Equal(t, "test content", entry.Content)
	assert.Equal(t, "agent-1", entry.AgentID)
	assert.Equal(t, 0.95, entry.Score)
}

func TestStoreResult_Struct(t *testing.T) {
	result := StoreResult{
		ID:         "result-id",
		MemoryType: MemoryTypeTaskResult,
		Content:    "result content",
		CreatedAt:  "2026-01-01T00:00:00Z",
	}
	assert.Equal(t, "result-id", result.ID)
	assert.Equal(t, MemoryTypeTaskResult, result.MemoryType)
}