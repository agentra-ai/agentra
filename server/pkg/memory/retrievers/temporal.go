package retrievers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// TemporalRetriever performs time-range based memory retrieval.
type TemporalRetriever struct {
	pool *pgxpool.Pool
}

// NewTemporalRetriever creates a new time-range based retriever.
func NewTemporalRetriever(pool *pgxpool.Pool) *TemporalRetriever {
	return &TemporalRetriever{pool: pool}
}

// Name returns the identifier for this retriever.
func (r *TemporalRetriever) Name() string {
	return "temporal"
}

// Retrieve performs time-range based retrieval.
// If TimeRange is not set in opts, returns memories from the last 24 hours.
func (r *TemporalRetriever) Retrieve(ctx context.Context, query string, opts RetrieveOptions) ([]Memory, error) {
	limit := int32(opts.Limit)
	if limit <= 0 {
		limit = 20
	}

	// Default to last 24 hours if no time range specified
	var start, end time.Time
	if opts.TimeRange != nil {
		start = opts.TimeRange.Start
		end = opts.TimeRange.End
	} else {
		end = time.Now()
		start = end.Add(-24 * time.Hour)
	}

	// Build memory types filter
	memTypes := opts.MemoryTypes
	typeStrs := make([]string, len(memTypes))
	for i, t := range memTypes {
		typeStrs[i] = string(t)
	}

	// Determine whether to search agent-specific or all memories
	if opts.AgentID != "" {
		return r.retrieveAgentMemories(ctx, opts, typeStrs, start, end, limit)
	}

	return r.retrieveAllMemories(ctx, opts, typeStrs, start, end, limit)
}

func (r *TemporalRetriever) retrieveAgentMemories(ctx context.Context, opts RetrieveOptions, memTypes []string, start, end time.Time, limit int32) ([]Memory, error) {
	queries := db.New(r.pool)

	agentUUID, err := uuid.Parse(opts.AgentID)
	if err != nil {
		return nil, err
	}

	workspaceID := uuid.Nil // TODO: get from context

	params := db.ListAgentMemoriesTimeRangeParams{
		AgentID:     uuidToPg(agentUUID),
		WorkspaceID: uuidToPg(workspaceID),
		Column3:     memTypes,
		CreatedAt:   pgtype.Timestamptz{Time: start, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: end, Valid: true},
		Limit:       limit,
	}

	rows, err := queries.ListAgentMemoriesTimeRange(ctx, params)
	if err != nil {
		return nil, err
	}

	mems := make([]Memory, 0, len(rows))
	for _, row := range rows {
		mems = append(mems, Memory{
			ID:         row.ID.String(),
			MemoryType: MemoryType(row.MemoryType),
			Content:    row.Content,
			AgentID:    row.AgentID.String(),
			CreatedAt:  row.CreatedAt.Time,
			Source:     "temporal",
		})
	}

	return mems, nil
}

func (r *TemporalRetriever) retrieveAllMemories(ctx context.Context, opts RetrieveOptions, memTypes []string, start, end time.Time, limit int32) ([]Memory, error) {
	queries := db.New(r.pool)

	wsUUID, err := uuid.Parse("") // TODO: get from context
	if err != nil {
		return nil, err
	}

	params := db.ListAllMemoriesTimeRangeParams{
		WorkspaceID: uuidToPg(wsUUID),
		Column2:     memTypes,
		CreatedAt:   pgtype.Timestamptz{Time: start, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: end, Valid: true},
		Limit:       limit,
	}

	rows, err := queries.ListAllMemoriesTimeRange(ctx, params)
	if err != nil {
		return nil, err
	}

	mems := make([]Memory, 0, len(rows))
	for _, row := range rows {
		mems = append(mems, Memory{
			ID:         row.ID.String(),
			MemoryType: MemoryType(row.MemoryType),
			Content:    row.Content,
			AgentID:    row.AgentID.String(),
			CreatedAt:  row.CreatedAt.Time,
			Source:     "temporal",
		})
	}

	return mems, nil
}