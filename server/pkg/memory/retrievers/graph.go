package retrievers

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// GraphRetriever performs entity/temporal/causal graph traversal for memory retrieval.
type GraphRetriever struct {
	pool *pgxpool.Pool
}

// NewGraphRetriever creates a new graph-based retriever.
func NewGraphRetriever(pool *pgxpool.Pool) *GraphRetriever {
	return &GraphRetriever{pool: pool}
}

// Name returns the identifier for this retriever.
func (r *GraphRetriever) Name() string {
	return "graph"
}

// Retrieve performs graph-based retrieval using entity extraction and relationship traversal.
func (r *GraphRetriever) Retrieve(ctx context.Context, query string, opts RetrieveOptions) ([]Memory, error) {
	limit := int32(opts.Limit)
	if limit <= 0 {
		limit = 20
	}

	// Extract entities from query using simple heuristics
	entities := extractEntities(query)
	if len(entities) == 0 {
		return nil, nil
	}

	primaryEntity := entities[0]
	var relatedEntity string
	if len(entities) > 1 {
		relatedEntity = entities[1]
	} else {
		relatedEntity = primaryEntity
	}

	// Build memory types filter
	memTypes := opts.MemoryTypes
	typeStrs := make([]string, len(memTypes))
	for i, t := range memTypes {
		typeStrs[i] = string(t)
	}

	// Use workspace-level graph search
	queries := db.New(r.pool)

	wsUUID, err := uuid.Parse("") // TODO: get from context
	if err != nil {
		return nil, err
	}

	params := db.ListMemoriesGraphParams{
		WorkspaceID: uuidToPg(wsUUID),
		Column2:     typeStrs,
		Column3:     pgtype.Text{String: primaryEntity, Valid: true},
		Column4:     pgtype.Text{String: relatedEntity, Valid: true},
		Limit:       limit,
	}

	rows, err := queries.ListMemoriesGraph(ctx, params)
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
			Source:     "graph",
		})
	}

	return mems, nil
}

// extractEntities extracts potential entity mentions from a query.
func extractEntities(query string) []string {
	var entities []string

	parts := strings.Fields(query)
	for _, p := range parts {
		clean := strings.Trim(p, ".,!?;:\"'()[]{}")
		if len(clean) > 2 {
			if strings.ToUpper(clean[:1]) == clean[:1] && strings.ToLower(clean[1:]) != clean[1:] {
				entities = append(entities, clean)
			}
		}
	}

	if len(entities) == 0 {
		for _, p := range parts {
			clean := strings.Trim(p, ".,!?;:\"'()[]{}")
			if len(clean) > 4 {
				entities = append(entities, clean)
			}
		}
	}

	if len(entities) > 2 {
		entities = entities[:2]
	}

	return entities
}