package retrievers

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// KeywordRetriever implements BM25 keyword-based retrieval using PostgreSQL full-text search.
type KeywordRetriever struct {
	pool *pgxpool.Pool
}

// NewKeywordRetriever creates a new keyword-based retriever.
func NewKeywordRetriever(pool *pgxpool.Pool) *KeywordRetriever {
	return &KeywordRetriever{pool: pool}
}

// Name returns the identifier for this retriever.
func (r *KeywordRetriever) Name() string {
	return "keyword"
}

// Retrieve performs BM25 keyword search on memory content.
func (r *KeywordRetriever) Retrieve(ctx context.Context, query string, opts RetrieveOptions) ([]Memory, error) {
	limit := int32(opts.Limit)
	if limit <= 0 {
		limit = 20
	}

	// Build memory types filter
	memTypes := opts.MemoryTypes
	typeStrs := make([]string, len(memTypes))
	for i, t := range memTypes {
		typeStrs[i] = string(t)
	}

	// If AgentID is provided, search agent memories; otherwise search all
	if opts.AgentID != "" {
		return r.retrieveAgentMemories(ctx, query, opts, limit, typeStrs)
	}

	return r.retrieveAllMemories(ctx, query, opts, limit, typeStrs)
}

func (r *KeywordRetriever) retrieveAgentMemories(ctx context.Context, query string, opts RetrieveOptions, limit int32, memTypes []string) ([]Memory, error) {
	queries := db.New(r.pool)

	agentUUID, err := uuid.Parse(opts.AgentID)
	if err != nil {
		return nil, err
	}

	workspaceID := uuid.Nil // TODO: get from context

	params := db.SearchAgentMemoriesBM25Params{
		AgentID:        uuidToPg(agentUUID),
		PlaintoTsquery: query,
		WorkspaceID:    uuidToPg(workspaceID),
		Column4:        memTypes,
		Limit:          limit,
	}

	rows, err := queries.SearchAgentMemoriesBM25(ctx, params)
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
			Score:      0, // BM25 doesn't provide similarity score in this implementation
			Source:     "keyword",
		})
	}

	return mems, nil
}

func (r *KeywordRetriever) retrieveAllMemories(ctx context.Context, query string, opts RetrieveOptions, limit int32, memTypes []string) ([]Memory, error) {
	queries := db.New(r.pool)

	// TODO: Get workspaceID from opts or context
	wsUUID, err := uuid.Parse("") // placeholder
	if err != nil {
		return nil, err
	}

	params := db.SearchAllMemoriesBM25Params{
		WorkspaceID:    uuidToPg(wsUUID),
		PlaintoTsquery: query,
		Column3:        memTypes,
		Limit:          limit,
	}

	rows, err := queries.SearchAllMemoriesBM25(ctx, params)
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
			Score:      float64(row.Score),
			Source:     "keyword",
		})
	}

	return mems, nil
}