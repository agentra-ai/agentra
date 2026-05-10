package retrievers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// uuidToPg converts a uuid.UUID to pgtype.UUID for SQL parameter binding.
func uuidToPg(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(u), Valid: true}
}

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

// MemoryType represents the type of memory.
type MemoryType string

const (
	MemoryTypeLearning   MemoryType = "learning"
	MemoryTypeTaskResult MemoryType = "task_result"
	MemoryTypeContext   MemoryType = "context"
	MemoryTypePattern   MemoryType = "pattern"
)

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

// Retriever is the interface implemented by all retrieval strategies.
type Retriever interface {
	// Retrieve executes the retrieval strategy and returns ranked memories.
	Retrieve(ctx context.Context, query string, opts RetrieveOptions) ([]Memory, error)
	// Name returns the identifier for this retriever (for debugging/logging).
	Name() string
}