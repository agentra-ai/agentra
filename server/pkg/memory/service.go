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
	row, err := s.queries.CreateAgentMemory(ctx, db.CreateAgentMemoryParams{
		AgentID:     uuid.MustParse(agentID),
		WorkspaceID: uuid.MustParse(workspaceID),
		MemoryType:  string(memType),
		Content:     content,
		Embedding:   vec,
		Metadata:    []byte("{}"),
		IsPrivate:   isPrivate,
	})
	if err != nil {
		return nil, err
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
	rows, err := s.queries.SearchAgentMemories(ctx, db.SearchAgentMemoriesParams{
		AgentID:     uuid.MustParse(agentID),
		Column2:     vec,
		WorkspaceID: uuid.MustParse(workspaceID),
		Column4:     memTypes,
		Limit:       int64(limit),
	})
	if err != nil {
		return nil, err
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
	rows, err := s.queries.SearchAllMemories(ctx, db.SearchAllMemoriesParams{
		WorkspaceID: uuid.MustParse(workspaceID),
		Column2:     vec,
		Column3:     nil,
		Limit:       int64(limit),
	})
	if err != nil {
		return nil, err
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
	_, err := s.queries.DeleteAgentMemory(ctx, uuid.MustParse(memoryID), uuid.MustParse(agentID))
	return err
}

func (s *MemoryService) StoreTeamMemory(ctx context.Context, workspaceID string, memType MemoryType, content string, createdBy string) (*StoreResult, error) {
	vec, err := s.embedder.Embed(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	var createdByUUID *uuid.UUID
	if createdBy != "" {
		v := uuid.MustParse(createdBy)
		createdByUUID = &v
	}
	row, err := s.queries.CreateTeamMemory(ctx, db.CreateTeamMemoryParams{
		WorkspaceID: uuid.MustParse(workspaceID),
		MemoryType:  string(memType),
		Content:     content,
		Embedding:   vec,
		Metadata:    []byte("{}"),
		CreatedBy:   createdByUUID,
	})
	if err != nil {
		return nil, err
	}
	return &StoreResult{
		ID:        row.ID.String(),
		MemoryType: MemoryType(row.MemoryType),
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}, nil
}

func (s *MemoryService) ListTeamMemories(ctx context.Context, workspaceID string) ([]TeamMemory, error) {
	rows, err := s.queries.ListTeamMemories(ctx, uuid.MustParse(workspaceID))
	if err != nil {
		return nil, err
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
