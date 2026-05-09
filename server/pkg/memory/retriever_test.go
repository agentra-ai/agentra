package memory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/agentra-ai/agentra/server/pkg/memory/retrievers"
)

func TestRRFFusion(t *testing.T) {
	// Test basic RRF fusion with memories from multiple sources
	memories := []retrievers.Memory{
		{ID: "1", Content: "test1", Score: 0.9, Source: "semantic"},
		{ID: "2", Content: "test2", Score: 0.8, Source: "keyword"},
		{ID: "1", Content: "test1", Score: 0.85, Source: "keyword"}, // duplicate ID
		{ID: "3", Content: "test3", Score: 0.7, Source: "graph"},
		{ID: "2", Content: "test2", Score: 0.75, Source: "temporal"}, // duplicate ID
	}

	result := rrfFusion(memories, 60)

	require.Len(t, result, 3)

	// IDs should be unique
	seen := make(map[string]bool)
	for _, mem := range result {
		assert.False(t, seen[mem.ID], "duplicate ID in result")
		seen[mem.ID] = true
	}

	// Memory "1" should be first due to highest combined score
	assert.Equal(t, "1", result[0].ID)
}

func TestRRFFusionEmpty(t *testing.T) {
	result := rrfFusion([]retrievers.Memory{}, 60)
	assert.Nil(t, result)
}

func TestRRFScoreCalculation(t *testing.T) {
	// Memory appearing at rank 0 from 3 sources: 1/60 + 1/60 + 1/60 = 0.05
	// Memory appearing at rank 1 from 2 sources: 1/61 + 1/61 = 0.0328
	memories := []retrievers.Memory{
		{ID: "a", Content: "a1", Score: 1.0, Source: "s1"},
		{ID: "a", Content: "a1", Score: 1.0, Source: "s2"},
		{ID: "a", Content: "a1", Score: 1.0, Source: "s3"},
		{ID: "b", Content: "b1", Score: 1.0, Source: "s1"},
		{ID: "b", Content: "b1", Score: 1.0, Source: "s2"},
	}

	result := rrfFusion(memories, 60)

	require.Len(t, result, 2)
	assert.Equal(t, "a", result[0].ID)
	assert.True(t, result[0].Score > result[1].Score)
}

func TestRetrieveOptionsDefaults(t *testing.T) {
	opts := RetrieveOptions{}

	assert.Equal(t, 0, opts.Limit)
	assert.Nil(t, opts.MemoryTypes)
	assert.Equal(t, "", opts.AgentID)
	assert.Nil(t, opts.TimeRange)
}

func TestTimeRange(t *testing.T) {
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()

	tr := TimeRange{
		Start: start,
		End:   end,
	}

	assert.True(t, tr.Start.Before(tr.End))
	assert.InDelta(t, time.Hour.Seconds(), tr.End.Sub(tr.Start).Seconds(), 0.01)
}

func TestMemoryType(t *testing.T) {
	assert.Equal(t, MemoryType("learning"), MemoryTypeLearning)
	assert.Equal(t, MemoryType("task_result"), MemoryTypeTaskResult)
	assert.Equal(t, MemoryType("context"), MemoryTypeContext)
	assert.Equal(t, MemoryType("pattern"), MemoryTypePattern)
}

func TestMemoryEntry(t *testing.T) {
	entry := MemoryEntry{
		ID:         "test-id",
		MemoryType: MemoryTypeLearning,
		Content:    "test content",
		AgentID:    "agent-123",
		Score:      0.95,
		CreatedAt:  "2024-01-01T00:00:00Z",
	}

	assert.Equal(t, "test-id", entry.ID)
	assert.Equal(t, MemoryTypeLearning, entry.MemoryType)
	assert.Equal(t, "test content", entry.Content)
	assert.Equal(t, "agent-123", entry.AgentID)
	assert.Equal(t, 0.95, entry.Score)
}