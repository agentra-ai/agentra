package memory

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
	"github.com/agentra-ai/agentra/pkg/db/generated"
)

type MemoryService struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	embedder *EmbeddingClient
}

func NewMemoryService(pool *pgxpool.Pool, embedder *EmbeddingClient) *MemoryService {
	return &MemoryService{
		pool:     pool,
		queries:  db.New(pool),
		embedder: embedder,
	}
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
		AgentID:     agentUUID,
		WorkspaceID: workspaceUUID,
		MemoryType:  string(memType),
		Content:     content,
		Embedding:   vec,
		Metadata:    []byte("{}"),
		IsPrivate:   isPrivate,
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
		AgentID:     agentUUID,
		Column2:     vec,
		WorkspaceID: workspaceUUID,
		Column4:     memTypes,
		Limit:       int64(limit),
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
		WorkspaceID: workspaceUUID,
		Column2:     vec,
		Column3:     nil,
		Limit:       int64(limit),
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

	_, err = s.queries.DeleteAgentMemory(ctx, memoryUUID, agentUUID)
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

	row, err := s.queries.CreateTeamMemory(ctx, db.CreateTeamMemoryParams{
		WorkspaceID: workspaceUUID,
		MemoryType:  string(memType),
		Content:     content,
		Embedding:   vec,
		Metadata:    []byte("{}"),
		CreatedBy:   createdByUUID,
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

	rows, err := s.queries.ListTeamMemories(ctx, workspaceUUID)
	if err != nil {
		return nil, fmt.Errorf("list team memories: %w", err)
	}
	result := make([]TeamMemory, len(rows))
	for i, r := range rows {
		result[i] = TeamMemory{
			ID:          r.ID.String(),
			WorkspaceID: r.WorkspaceID.String(),
			MemoryType:  MemoryType(r.MemoryType),
			Content:     r.Content,
			CreatedBy:   r.CreatedBy.String,
			CreatedAt:   r.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
	}
	return result, nil
}