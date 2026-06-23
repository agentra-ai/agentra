package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Authenticator handles API Key validation
type Authenticator struct {
	db *pgxpool.Pool
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(db *pgxpool.Pool) *Authenticator {
	return &Authenticator{db: db}
}

// ValidateAPIKey validates an API key and returns the workspace ID
func (a *Authenticator) ValidateAPIKey(ctx context.Context, apiKey string) (uuid.UUID, error) {
	workspaceID := ExtractWorkspaceID(apiKey)
	if workspaceID == "" {
		return uuid.Nil, NewUnauthorizedError("invalid API key format")
	}

	// Verify the API key exists in the database
	var exists bool
	query := `SELECT EXISTS(
		SELECT 1 FROM personal_access_token
		WHERE token_hash = encode(sha256($1::bytea), 'hex')
		AND workspace_id = $2
	)`

	err := a.db.QueryRow(ctx, query, apiKey, workspaceID).Scan(&exists)
	if err != nil {
		return uuid.Nil, NewInternalError("failed to validate API key")
	}

	if !exists {
		return uuid.Nil, NewUnauthorizedError("API key not found or expired")
	}

	parsed, err := uuid.Parse(workspaceID)
	if err != nil {
		return uuid.Nil, NewUnauthorizedError("invalid workspace id in API key")
	}
	return parsed, nil
}

// ExtractWorkspaceID extracts workspace ID from API key format: agentra_api_{workspace_id}_{random}
func ExtractWorkspaceID(apiKey string) string {
	parts := strings.Split(apiKey, "_")
	if len(parts) != 4 {
		return ""
	}
	if parts[0] != "agentra" || parts[1] != "api" {
		return ""
	}

	// Verify UUID format
	workspaceID := parts[2]
	if _, err := uuid.Parse(workspaceID); err != nil {
		return ""
	}

	return workspaceID
}

// GetAPIKeyWorkspaceID is a helper that only extracts the workspace ID without DB lookup
func GetAPIKeyWorkspaceID(apiKey string) (string, error) {
	id := ExtractWorkspaceID(apiKey)
	if id == "" {
		return "", fmt.Errorf("invalid API key format")
	}
	return id, nil
}
