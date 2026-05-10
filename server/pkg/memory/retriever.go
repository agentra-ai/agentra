package memory

import (
	"context"
	"time"
)

// RetrieveOptions controls the behavior of retrieval strategies.
type RetrieveOptions struct {
	Limit       int          // max results to return
	MemoryTypes []MemoryType // filter by memory types (empty = all)
	AgentID     string       // filter by agent (empty = any)
	WorkspaceID string       // required for workspace-scoped retrieval
	TimeRange   *TimeRange   // optional time filter
}

// TimeRange filters memories to a specific time window.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Retriever is the interface implemented by all retrieval strategies.
// Each retriever returns memories ranked by relevance to the query.
type Retriever interface {
	// Retrieve executes the retrieval strategy and returns ranked memories.
	Retrieve(ctx context.Context, query string, opts RetrieveOptions) ([]Memory, error)
	// Name returns the identifier for this retriever (for debugging/logging).
	Name() string
}

// Memory represents a generic memory entry from any source.
type Memory struct {
	ID         string
	MemoryType MemoryType
	Content    string
	AgentID    string
	Score      float64
	CreatedAt  time.Time
	Source     string // which retriever produced this result
}