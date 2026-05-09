package tools

import (
	"context"
	"fmt"

	"github.com/agentra-ai/agentra/pkg/mcp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InboxService handles inbox-related tools
type InboxService struct {
	db *pgxpool.Pool
}

// NewInboxService creates a new inbox service
func NewInboxService(db *pgxpool.Pool) *InboxService {
	return &InboxService{db: db}
}

// InboxList lists notifications for a user
func (s *InboxService) InboxList(ctx context.Context, params map[string]any) (any, error) {
	userID, ok := params["user_id"].(string)
	if !ok || userID == "" {
		return nil, mcp.NewValidationError("user_id is required")
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid user_id format")
	}

	query := `SELECT id, user_id, type, title, content, read, created_at
		FROM inbox WHERE user_id = $1`

	args := []any{userUUID}

	if read, ok := params["read"].(bool); ok {
		query += " AND read = $2"
		args = append(args, read)
	}

	query += " ORDER BY created_at DESC LIMIT 50"

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, mcp.NewInternalError("failed to query inbox")
	}
	defer rows.Close()

	notifications := []map[string]any{}
	for rows.Next() {
		var id, userIDOut any
		var notifType, title, content string
		var read bool
		var createdAt any

		if err := rows.Scan(&id, &userIDOut, &notifType, &title, &content, &read, &createdAt); err != nil {
			return nil, mcp.NewInternalError("failed to scan notification")
		}

		notifications = append(notifications, map[string]any{
			"id":         fmt.Sprintf("%v", id),
			"user_id":   fmt.Sprintf("%v", userIDOut),
			"type":      notifType,
			"title":     title,
			"content":   content,
			"read":      read,
			"created_at": createdAt,
		})
	}

	return map[string]any{"notifications": notifications, "total": len(notifications)}, nil
}

// InboxMarkRead marks a notification as read
func (s *InboxService) InboxMarkRead(ctx context.Context, params map[string]any) (any, error) {
	notificationID, ok := params["notification_id"].(string)
	if !ok || notificationID == "" {
		return nil, mcp.NewValidationError("notification_id is required")
	}

	id, err := uuid.Parse(notificationID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid notification_id format")
	}

	query := "UPDATE inbox SET read = true WHERE id = $1"
	_, err = s.db.Exec(ctx, query, id)
	if err != nil {
		return nil, mcp.NewInternalError("failed to mark notification as read")
	}

	return map[string]any{"success": true}, nil
}

// Package-level wrappers
func InboxList(ctx mcp.ToolContext, params map[string]any) (any, error) {
	return NewInboxService(nil).InboxList(context.Background(), params)
}

func InboxMarkRead(ctx mcp.ToolContext, params map[string]any) (any, error) {
	return NewInboxService(nil).InboxMarkRead(context.Background(), params)
}