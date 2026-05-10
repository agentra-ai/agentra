package memory

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/agentra-ai/agentra/server/pkg/memory/retrievers"
)

func TestRRFFusion(t *testing.T) {
	// 3 retrievers, each returning a ranked list. Memory "1" appears in 2 lists.
	retrieverLists := [][]retrievers.Memory{
		{ // retriever 1: semantic
			{ID: "1", Content: "test1", Score: 0.9, Source: "semantic"},
			{ID: "2", Content: "test2", Score: 0.8, Source: "semantic"},
		},
		{ // retriever 2: keyword
			{ID: "1", Content: "test1", Score: 0.85, Source: "keyword"},
			{ID: "3", Content: "test3", Score: 0.7, Source: "keyword"},
		},
		{ // retriever 3: graph
			{ID: "2", Content: "test2", Score: 0.75, Source: "graph"},
			{ID: "4", Content: "test4", Score: 0.6, Source: "graph"},
		},
	}

	result := rrfFusion(retrieverLists, 60)

	require.Len(t, result, 4)

	// IDs should be unique
	seen := make(map[string]bool)
	for _, mem := range result {
		assert.False(t, seen[mem.ID], "duplicate ID in result")
		seen[mem.ID] = true
	}

	// Memory "1" appears at rank 1 in two retrievers: 1/(1+60) + 1/(1+60) = 0.0328
	// Memory "2" appears at rank 2 in semantic, rank 1 in graph: 1/(2+60) + 1/(1+60) = 0.0161+0.0164 = 0.0325
	// So "1" should be first
	assert.Equal(t, "1", result[0].ID)
}

func TestRRFFusionEmpty(t *testing.T) {
	result := rrfFusion([][]retrievers.Memory{}, 60)
	assert.Nil(t, result)
}

func TestRRFScoreCalculation(t *testing.T) {
	// Memory "a" appears at rank 1 in 3 retrievers: RRF = 1/61 + 1/61 + 1/61 = 0.0492
	// Memory "b" appears at rank 2 in 2 retrievers: RRF = 1/62 + 1/62 = 0.0323
	retrieverLists := [][]retrievers.Memory{
		{{ID: "a", Content: "a1", Score: 1.0, Source: "s1"}},
		{{ID: "a", Content: "a1", Score: 1.0, Source: "s2"}},
		{{ID: "a", Content: "a1", Score: 1.0, Source: "s3"}},
		{{ID: "b", Content: "b1", Score: 1.0, Source: "s1"}},
		{{ID: "b", Content: "b1", Score: 1.0, Source: "s2"}},
	}

	result := rrfFusion(retrieverLists, 60)

	require.Len(t, result, 2)
	assert.Equal(t, "a", result[0].ID)
	assert.True(t, result[0].Score > result[1].Score, "a's RRF score %.4f should exceed b's %.4f", result[0].Score, result[1].Score)
}

func TestRRFRankOrdering(t *testing.T) {
	// Verify that a memory ranked higher within a single retriever gets a better RRF score
	// Memory "top" at rank 1: 1/61 = 0.0164
	// Memory "bottom" at rank 50: 1/110 = 0.0091
	retrieverLists := [][]retrievers.Memory{
		{
			{ID: "top", Content: "a", Score: 1.0, Source: "semantic"},
			{ID: "mid1", Content: "b", Score: 0.9, Source: "semantic"},
			{ID: "mid2", Content: "c", Score: 0.8, Source: "semantic"},
			{ID: "mid3", Content: "d", Score: 0.7, Source: "semantic"},
			{ID: "bottom", Content: "e", Score: 0.6, Source: "semantic"},
		},
	}

	result := rrfFusion(retrieverLists, 60)
	require.Len(t, result, 5)
	assert.Equal(t, "top", result[0].ID)
	assert.Equal(t, "bottom", result[4].ID)
	assert.True(t, result[0].Score > result[4].Score)
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
