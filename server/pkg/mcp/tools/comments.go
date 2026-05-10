package tools

import (
	"context"
	"fmt"

	"github.com/agentra-ai/agentra/server/pkg/mcp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CommentService handles comment-related tools
type CommentService struct {
	db *pgxpool.Pool
}

// NewCommentService creates a new comment service
func NewCommentService(db *pgxpool.Pool) *CommentService {
	return &CommentService{db: db}
}

// CommentList lists comments for an issue
func (s *CommentService) CommentList(ctx context.Context, params map[string]any) (any, error) {
	issueID, ok := params["issue_id"].(string)
	if !ok || issueID == "" {
		return nil, mcp.NewValidationError("issue_id is required")
	}

	id, err := uuid.Parse(issueID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid issue_id format")
	}

	query := `SELECT id, issue_id, author_type, author_id, content, created_at, updated_at
		FROM comment WHERE issue_id = $1 ORDER BY created_at ASC LIMIT 100`

	rows, err := s.db.Query(ctx, query, id)
	if err != nil {
		return nil, mcp.NewInternalError("failed to query comments")
	}
	defer rows.Close()

	comments := []map[string]any{}
	for rows.Next() {
		var id, issueID, authorID any
		var authorType, content string
		var createdAt, updatedAt any

		if err := rows.Scan(&id, &issueID, &authorType, &authorID, &content, &createdAt, &updatedAt); err != nil {
			return nil, mcp.NewInternalError("failed to scan comment")
		}

		comments = append(comments, map[string]any{
			"id":         fmt.Sprintf("%v", id),
			"issue_id":   fmt.Sprintf("%v", issueID),
			"author_type": authorType,
			"author_id":  fmt.Sprintf("%v", authorID),
			"content":    content,
			"created_at": createdAt,
			"updated_at": updatedAt,
		})
	}

	return map[string]any{"comments": comments, "total": len(comments)}, nil
}

// CommentCreate creates a new comment
func (s *CommentService) CommentCreate(ctx context.Context, params map[string]any) (any, error) {
	issueID, ok := params["issue_id"].(string)
	if !ok || issueID == "" {
		return nil, mcp.NewValidationError("issue_id is required")
	}

	content, ok := params["content"].(string)
	if !ok || content == "" {
		return nil, mcp.NewValidationError("content is required")
	}

	issueUUID, err := uuid.Parse(issueID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid issue_id format")
	}

	query := `INSERT INTO comment (issue_id, author_type, author_id, content)
		VALUES ($1, 'member', $2, $3)
		RETURNING id, issue_id, author_type, author_id, content, created_at, updated_at`

	var id, issueIDOut, authorIDOut any
	var authorType, contentOut string
	var createdAt, updatedAt any

	err = s.db.QueryRow(ctx, query, issueUUID, params["workspace_id"], content).Scan(&id, &issueIDOut, &authorType, &authorIDOut, &contentOut, &createdAt, &updatedAt)
	if err != nil {
		return nil, mcp.NewInternalError("failed to create comment")
	}

	return map[string]any{
		"id":          fmt.Sprintf("%v", id),
		"issue_id":    fmt.Sprintf("%v", issueIDOut),
		"author_type": authorType,
		"author_id":  fmt.Sprintf("%v", authorIDOut),
		"content":     contentOut,
		"created_at": createdAt,
		"updated_at": updatedAt,
	}, nil
}

// Package-level wrappers
func CommentList(ctx mcp.ToolContext, params map[string]any) (any, error) {
	return NewCommentService(nil).CommentList(context.Background(), params)
}

func CommentCreate(ctx mcp.ToolContext, params map[string]any) (any, error) {
	return NewCommentService(nil).CommentCreate(context.Background(), params)
}