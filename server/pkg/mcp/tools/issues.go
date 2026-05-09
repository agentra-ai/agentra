package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentra-ai/agentra/pkg/mcp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IssueService handles issue-related tools
type IssueService struct {
	db *pgxpool.Pool
}

// NewIssueService creates a new issue service
func NewIssueService(db *pgxpool.Pool) *IssueService {
	return &IssueService{db: db}
}

// IssueList lists issues in a workspace
func (s *IssueService) IssueList(ctx context.Context, params map[string]any) (any, error) {
	workspaceID, ok := params["workspace_id"].(string)
	if !ok || workspaceID == "" {
		return nil, mcp.NewValidationError("workspace_id is required")
	}

	wsUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid workspace_id format")
	}

	query := `SELECT id, workspace_id, title, description, status, priority,
		assignee_type, assignee_id, created_at, updated_at
		FROM issue WHERE workspace_id = $1`
	args := []any{wsUUID}
	argIdx := 2

	if status, ok := params["status"].(string); ok && status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	if priority, ok := params["priority"].(string); ok && priority != "" {
		query += fmt.Sprintf(" AND priority = $%d", argIdx)
		args = append(args, priority)
		argIdx++
	}

	query += " ORDER BY created_at DESC LIMIT 50"

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, mcp.NewInternalError("failed to query issues")
	}
	defer rows.Close()

	issues := []map[string]any{}
	for rows.Next() {
		var id, workspaceID, assigneeID any
		var title, description, status, priority, assigneeType string
		var createdAt, updatedAt any

		if err := rows.Scan(&id, &workspaceID, &title, &description, &status, &priority, &assigneeType, &assigneeID, &createdAt, &updatedAt); err != nil {
			return nil, mcp.NewInternalError("failed to scan issue")
		}

		issues = append(issues, map[string]any{
			"id":           fmt.Sprintf("%v", id),
			"workspace_id": fmt.Sprintf("%v", workspaceID),
			"title":        title,
			"description":  description,
			"status":       status,
			"priority":     priority,
			"assignee_id":  fmt.Sprintf("%v", assigneeID),
			"assignee_type": assigneeType,
			"created_at":   createdAt,
			"updated_at":   updatedAt,
		})
	}

	return map[string]any{"issues": issues, "total": len(issues)}, nil
}

// IssueGet gets a single issue
func (s *IssueService) IssueGet(ctx context.Context, params map[string]any) (any, error) {
	issueID, ok := params["issue_id"].(string)
	if !ok || issueID == "" {
		return nil, mcp.NewValidationError("issue_id is required")
	}

	id, err := uuid.Parse(issueID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid issue_id format")
	}

	query := `SELECT id, workspace_id, title, description, status, priority,
		assignee_type, assignee_id, creator_type, creator_id, created_at, updated_at
		FROM issue WHERE id = $1`

	var id2, workspaceID, assigneeID, creatorID any
	var title, description, status, priority, assigneeType, creatorType string
	var createdAt, updatedAt any

	err = s.db.QueryRow(ctx, query, id).Scan(&id2, &workspaceID, &title, &description, &status, &priority, &assigneeType, &assigneeID, &creatorType, &creatorID, &createdAt, &updatedAt)
	if err != nil {
		return nil, mcp.NewNotFoundError("issue not found")
	}

	return map[string]any{
		"id":           fmt.Sprintf("%v", id2),
		"workspace_id": fmt.Sprintf("%v", workspaceID),
		"title":        title,
		"description":  description,
		"status":       status,
		"priority":     priority,
		"assignee_id":  fmt.Sprintf("%v", assigneeID),
		"assignee_type": assigneeType,
		"created_by":   fmt.Sprintf("%v", creatorID),
		"created_at":   createdAt,
		"updated_at":   updatedAt,
	}, nil
}

// IssueCreate creates a new issue
func (s *IssueService) IssueCreate(ctx context.Context, params map[string]any) (any, error) {
	workspaceID, ok := params["workspace_id"].(string)
	if !ok || workspaceID == "" {
		return nil, mcp.NewValidationError("workspace_id is required")
	}

	title, ok := params["title"].(string)
	if !ok || title == "" {
		return nil, mcp.NewValidationError("title is required")
	}

	wsUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid workspace_id format")
	}

	status := "open"
	if st, ok := params["status"].(string); ok && st != "" {
		status = st
	}

	priority := "medium"
	if p, ok := params["priority"].(string); ok && p != "" {
		priority = p
	}

	description := ""
	if d, ok := params["description"].(string); ok {
		description = d
	}

	query := `INSERT INTO issue (workspace_id, title, description, status, priority, creator_type, creator_id, position, number)
		VALUES ($1, $2, $3, $4, $5, 'member', $6, 0,
			(SELECT COALESCE(MAX(number), 0) + 1 FROM issue WHERE workspace_id = $1))
		RETURNING id, workspace_id, title, description, status, priority, created_at, updated_at`

	var id, wsID any
	var title2, desc, status2, priority2 string
	var createdAt, updatedAt any

	err = s.db.QueryRow(ctx, query, wsUUID, title, description, status, priority, workspaceID).Scan(&id, &wsID, &title2, &desc, &status2, &priority2, &createdAt, &updatedAt)
	if err != nil {
		return nil, mcp.NewInternalError("failed to create issue")
	}

	return map[string]any{
		"id":           fmt.Sprintf("%v", id),
		"workspace_id": fmt.Sprintf("%v", wsID),
		"title":        title2,
		"description":  desc,
		"status":       status2,
		"priority":     priority2,
		"created_at":   createdAt,
		"updated_at":   updatedAt,
	}, nil
}

// IssueUpdate updates an issue
func (s *IssueService) IssueUpdate(ctx context.Context, params map[string]any) (any, error) {
	issueID, ok := params["issue_id"].(string)
	if !ok || issueID == "" {
		return nil, mcp.NewValidationError("issue_id is required")
	}

	id, err := uuid.Parse(issueID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid issue_id format")
	}

	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if title, ok := params["title"].(string); ok {
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", argIdx))
		args = append(args, title)
		argIdx++
	}

	if status, ok := params["status"].(string); ok {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}

	if priority, ok := params["priority"].(string); ok {
		setClauses = append(setClauses, fmt.Sprintf("priority = $%d", argIdx))
		args = append(args, priority)
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil, mcp.NewValidationError("no fields to update")
	}

	query := fmt.Sprintf("UPDATE issue SET %s WHERE id = $%d RETURNING id, workspace_id, title, status, priority, updated_at",
		strings.Join(setClauses, ", "), argIdx+1)
	args = append(args, id)

	var id2, wsID, updatedAt any
	var title2, status2, priority2 string

	err = s.db.QueryRow(ctx, query, args...).Scan(&id2, &wsID, &title2, &status2, &priority2, &updatedAt)
	if err != nil {
		return nil, mcp.NewNotFoundError("issue not found")
	}

	return map[string]any{
		"id":           fmt.Sprintf("%v", id2),
		"workspace_id": fmt.Sprintf("%v", wsID),
		"title":        title2,
		"status":       status2,
		"priority":     priority2,
		"updated_at":   updatedAt,
	}, nil
}

// IssueDelete deletes an issue
func (s *IssueService) IssueDelete(ctx context.Context, params map[string]any) (any, error) {
	issueID, ok := params["issue_id"].(string)
	if !ok || issueID == "" {
		return nil, mcp.NewValidationError("issue_id is required")
	}

	id, err := uuid.Parse(issueID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid issue_id format")
	}

	query := "DELETE FROM issue WHERE id = $1"
	_, err = s.db.Exec(ctx, query, id)
	if err != nil {
		return nil, mcp.NewInternalError("failed to delete issue")
	}

	return map[string]any{"deleted": true}, nil
}

// Package-level functions that wrap IssueService methods for backward compatibility
// These use a global service instance set via SetIssueService

var globalIssueService *IssueService

// SetIssueService sets the global issue service instance
func SetIssueService(svc *IssueService) {
	globalIssueService = svc
}

// IssueList lists issues in a workspace using the global service
func IssueList(ctx mcp.ToolContext, params map[string]any) (any, error) {
	if globalIssueService == nil {
		return nil, mcp.NewInternalError("issue service not initialized")
	}
	return globalIssueService.IssueList(context.Background(), params)
}

// IssueGet gets a single issue using the global service
func IssueGet(ctx mcp.ToolContext, params map[string]any) (any, error) {
	if globalIssueService == nil {
		return nil, mcp.NewInternalError("issue service not initialized")
	}
	return globalIssueService.IssueGet(context.Background(), params)
}

// IssueCreate creates a new issue using the global service
func IssueCreate(ctx mcp.ToolContext, params map[string]any) (any, error) {
	if globalIssueService == nil {
		return nil, mcp.NewInternalError("issue service not initialized")
	}
	return globalIssueService.IssueCreate(context.Background(), params)
}

// IssueUpdate updates an issue using the global service
func IssueUpdate(ctx mcp.ToolContext, params map[string]any) (any, error) {
	if globalIssueService == nil {
		return nil, mcp.NewInternalError("issue service not initialized")
	}
	return globalIssueService.IssueUpdate(context.Background(), params)
}

// IssueDelete deletes an issue using the global service
func IssueDelete(ctx mcp.ToolContext, params map[string]any) (any, error) {
	if globalIssueService == nil {
		return nil, mcp.NewInternalError("issue service not initialized")
	}
	return globalIssueService.IssueDelete(context.Background(), params)
}