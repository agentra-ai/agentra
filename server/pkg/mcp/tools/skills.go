package tools

import (
	"context"
	"fmt"

	"github.com/agentra-ai/agentra/server/pkg/mcp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SkillService handles skill-related tools
type SkillService struct {
	db *pgxpool.Pool
}

// NewSkillService creates a new skill service
func NewSkillService(db *pgxpool.Pool) *SkillService {
	return &SkillService{db: db}
}

// SkillList lists skills in a workspace
func (s *SkillService) SkillList(ctx context.Context, params map[string]any) (any, error) {
	workspaceID, ok := params["workspace_id"].(string)
	if !ok || workspaceID == "" {
		return nil, mcp.NewValidationError("workspace_id is required")
	}

	wsUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid workspace_id format")
	}

	query := `SELECT id, workspace_id, name, description, content, config, created_at, updated_at
		FROM skill WHERE workspace_id = $1 ORDER BY created_at DESC LIMIT 50`

	rows, err := s.db.Query(ctx, query, wsUUID)
	if err != nil {
		return nil, mcp.NewInternalError("failed to query skills")
	}
	defer rows.Close()

	skills := []map[string]any{}
	for rows.Next() {
		var id, workspaceID, config any
		var name, description, content string
		var createdAt, updatedAt any

		if err := rows.Scan(&id, &workspaceID, &name, &description, &content, &config, &createdAt, &updatedAt); err != nil {
			return nil, mcp.NewInternalError("failed to scan skill")
		}

		skills = append(skills, map[string]any{
			"id":           fmt.Sprintf("%v", id),
			"workspace_id": fmt.Sprintf("%v", workspaceID),
			"name":         name,
			"description":  description,
			"content":      content,
			"config":       config,
			"created_at":   createdAt,
			"updated_at":   updatedAt,
		})
	}

	return map[string]any{"skills": skills, "total": len(skills)}, nil
}

// SkillGet gets a single skill
func (s *SkillService) SkillGet(ctx context.Context, params map[string]any) (any, error) {
	skillID, ok := params["skill_id"].(string)
	if !ok || skillID == "" {
		return nil, mcp.NewValidationError("skill_id is required")
	}

	id, err := uuid.Parse(skillID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid skill_id format")
	}

	query := `SELECT id, workspace_id, name, description, content, config, created_at, updated_at
		FROM skill WHERE id = $1`

	var id2, workspaceID, config any
	var name, description, content string
	var createdAt, updatedAt any

	err = s.db.QueryRow(ctx, query, id).Scan(&id2, &workspaceID, &name, &description, &content, &config, &createdAt, &updatedAt)
	if err != nil {
		return nil, mcp.NewNotFoundError("skill not found")
	}

	return map[string]any{
		"id":           fmt.Sprintf("%v", id2),
		"workspace_id": fmt.Sprintf("%v", workspaceID),
		"name":         name,
		"description":  description,
		"content":      content,
		"config":       config,
		"created_at":   createdAt,
		"updated_at":   updatedAt,
	}, nil
}

// SkillApply applies a skill to an issue (placeholder implementation)
func (s *SkillService) SkillApply(ctx context.Context, params map[string]any) (any, error) {
	skillID, ok := params["skill_id"].(string)
	if !ok || skillID == "" {
		return nil, mcp.NewValidationError("skill_id is required")
	}

	issueID, ok := params["issue_id"].(string)
	if !ok || issueID == "" {
		return nil, mcp.NewValidationError("issue_id is required")
	}

	// In a full implementation, this would create a skill_application record
	// For now, just validate the IDs exist
	_, err := uuid.Parse(skillID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid skill_id format")
	}

	_, err = uuid.Parse(issueID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid issue_id format")
	}

	return map[string]any{
		"applied":    true,
		"skill_id":   skillID,
		"issue_id":   issueID,
	}, nil
}

// Package-level wrappers for ToolContext compatibility
func SkillList(ctx mcp.ToolContext, params map[string]any) (any, error) {
	return NewSkillService(nil).SkillList(context.Background(), params)
}

func SkillGet(ctx mcp.ToolContext, params map[string]any) (any, error) {
	return NewSkillService(nil).SkillGet(context.Background(), params)
}

func SkillApply(ctx mcp.ToolContext, params map[string]any) (any, error) {
	return NewSkillService(nil).SkillApply(context.Background(), params)
}