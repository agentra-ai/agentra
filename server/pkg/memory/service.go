package memory

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

type MemoryService struct {
	pool       *pgxpool.Pool
	queries    *db.Queries
	embedder   *EmbeddingClient
	fusion     *FusionRetriever
}

func NewMemoryService(pool *pgxpool.Pool, embedder *EmbeddingClient) *MemoryService {
	s := &MemoryService{
		pool:     pool,
		queries:  db.New(pool),
		embedder: embedder,
	}
	s.fusion = NewFusionRetriever(pool, embedder)
	return s
}

func (s *MemoryService) StoreAgentMemory(ctx context.Context, agentID, workspaceID string, memType MemoryType, content string, isPrivate bool) (*StoreResult, error) {
	vec, err := s.embedder.Embed(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}

	agentUUID, err := uuid.Parse(agentID)
	if err != nil {
		return nil, fmt.Errorf("invalid agent_id: %w", err)
	}
	workspaceUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace_id: %w", err)
	}

	row, err := s.queries.CreateAgentMemory(ctx, db.CreateAgentMemoryParams{
		AgentID:     uuidToPg(agentUUID),
		WorkspaceID: uuidToPg(workspaceUUID),
		MemoryType:  string(memType),
		Content:     content,
		Embedding:   vectorToPg(vec),
		Metadata:    []byte("{}"),
		IsPrivate:   boolToPg(isPrivate),
	})
	if err != nil {
		return nil, fmt.Errorf("create agent memory: %w", err)
	}
	return &StoreResult{
		ID:        row.ID.String(),
		MemoryType: MemoryType(row.MemoryType),
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}, nil
}

func (s *MemoryService) RecallAgentMemories(ctx context.Context, agentID, workspaceID, query string, limit int, memTypes []string) (*RecallResult, error) {
	vec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}

	agentUUID, err := uuid.Parse(agentID)
	if err != nil {
		return nil, fmt.Errorf("invalid agent_id: %w", err)
	}
	workspaceUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace_id: %w", err)
	}

	rows, err := s.queries.SearchAgentMemories(ctx, db.SearchAgentMemoriesParams{
		AgentID:     uuidToPg(agentUUID),
		Column2:     vectorToPg(vec),
		WorkspaceID: uuidToPg(workspaceUUID),
		Column4:     memTypes,
		Limit:       int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search agent memories: %w", err)
	}
	entries := make([]MemoryEntry, len(rows))
	for i, r := range rows {
		entries[i] = MemoryEntry{
			ID:         r.ID.String(),
			MemoryType: MemoryType(r.MemoryType),
			Content:    r.Content,
			AgentID:    r.AgentID.String(),
			Score:      r.Score,
			CreatedAt:  r.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
	}
	return &RecallResult{Memories: entries}, nil
}

func (s *MemoryService) SearchAll(ctx context.Context, workspaceID, query string, includeTeam bool, limit int) ([]MemoryEntry, error) {
	vec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}

	workspaceUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace_id: %w", err)
	}

	rows, err := s.queries.SearchAllMemories(ctx, db.SearchAllMemoriesParams{
		WorkspaceID: uuidToPg(workspaceUUID),
		Column2:     vectorToPg(vec),
		Column3:     nil,
		Limit:       int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search all memories: %w", err)
	}
	entries := make([]MemoryEntry, len(rows))
	for i, r := range rows {
		entries[i] = MemoryEntry{
			ID:         r.ID.String(),
			MemoryType: MemoryType(r.MemoryType),
			Content:    r.Content,
			AgentID:    r.AgentID.String(),
			Score:      r.Score,
			CreatedAt:  r.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
	}
	return entries, nil
}

func (s *MemoryService) DeleteAgentMemory(ctx context.Context, memoryID, agentID string) error {
	memoryUUID, err := uuid.Parse(memoryID)
	if err != nil {
		return fmt.Errorf("invalid memory_id: %w", err)
	}
	agentUUID, err := uuid.Parse(agentID)
	if err != nil {
		return fmt.Errorf("invalid agent_id: %w", err)
	}

	_, err = s.queries.DeleteAgentMemory(ctx, db.DeleteAgentMemoryParams{
		ID:      uuidToPg(memoryUUID),
		AgentID: uuidToPg(agentUUID),
	})
	if err != nil {
		return fmt.Errorf("delete agent memory: %w", err)
	}
	return nil
}

func (s *MemoryService) StoreTeamMemory(ctx context.Context, workspaceID string, memType MemoryType, content string, createdBy string) (*StoreResult, error) {
	vec, err := s.embedder.Embed(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}

	workspaceUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace_id: %w", err)
	}

	var createdByUUID *uuid.UUID
	if createdBy != "" {
		v, err := uuid.Parse(createdBy)
		if err != nil {
			return nil, fmt.Errorf("invalid created_by: %w", err)
		}
		createdByUUID = &v
	}

	var createdByPg pgtype.UUID
	if createdByUUID != nil {
		createdByPg = uuidToPg(*createdByUUID)
	}

	row, err := s.queries.CreateTeamMemory(ctx, db.CreateTeamMemoryParams{
		WorkspaceID: uuidToPg(workspaceUUID),
		MemoryType:  string(memType),
		Content:     content,
		Embedding:   vectorToPg(vec),
		Metadata:    []byte("{}"),
		CreatedBy:   createdByPg,
	})
	if err != nil {
		return nil, fmt.Errorf("create team memory: %w", err)
	}
	return &StoreResult{
		ID:        row.ID.String(),
		MemoryType: MemoryType(row.MemoryType),
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}, nil
}

func (s *MemoryService) ListTeamMemories(ctx context.Context, workspaceID string) ([]TeamMemory, error) {
	workspaceUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace_id: %w", err)
	}

	rows, err := s.queries.ListTeamMemories(ctx, uuidToPg(workspaceUUID))
	if err != nil {
		return nil, fmt.Errorf("list team memories: %w", err)
	}
	result := make([]TeamMemory, len(rows))
	for i, r := range rows {
		createdBy := ""
		if r.CreatedBy.Valid {
			createdBy = uuid.UUID(r.CreatedBy.Bytes).String()
		}
		result[i] = TeamMemory{
			ID:          r.ID.String(),
			WorkspaceID: r.WorkspaceID.String(),
			MemoryType:  MemoryType(r.MemoryType),
			Content:     r.Content,
			CreatedBy:   createdBy,
			CreatedAt:   r.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
	}
	return result, nil
}

// MultiStrategyRecall uses the FusionRetriever to combine semantic, keyword, graph,
// and temporal retrieval strategies with Reciprocal Rank Fusion.
func (s *MemoryService) MultiStrategyRecall(ctx context.Context, agentID, workspaceID, query string, opts RetrieveOptions) ([]MemoryEntry, error) {
	// Set defaults
	if opts.Limit == 0 {
		opts.Limit = 20
	}

	// Set agent ID if not provided
	if agentID != "" {
		opts.AgentID = agentID
	}

	// Retrieve using fusion retriever
	memories, err := s.fusion.Retrieve(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("fusion retrieve: %w", err)
	}

	// Convert to MemoryEntry format
	entries := make([]MemoryEntry, len(memories))
	for i, mem := range memories {
		entries[i] = MemoryEntry{
			ID:         mem.ID,
			MemoryType: mem.MemoryType,
			Content:    mem.Content,
			AgentID:    mem.AgentID,
			Score:      mem.Score,
			CreatedAt:  mem.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return entries, nil
}