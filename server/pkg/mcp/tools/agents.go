package tools

import (
	"context"
	"fmt"

	"github.com/agentra-ai/agentra/pkg/mcp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentService handles agent-related tools
type AgentService struct {
	db *pgxpool.Pool
}

// NewAgentService creates a new agent service
func NewAgentService(db *pgxpool.Pool) *AgentService {
	return &AgentService{db: db}
}

// AgentList lists agents in a workspace
func (s *AgentService) AgentList(ctx context.Context, params map[string]any) (any, error) {
	workspaceID, ok := params["workspace_id"].(string)
	if !ok || workspaceID == "" {
		return nil, mcp.NewValidationError("workspace_id is required")
	}

	wsUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid workspace_id format")
	}

	query := `SELECT id, workspace_id, name, provider, status, runtime_id, created_at, updated_at
		FROM agent WHERE workspace_id = $1 ORDER BY created_at DESC LIMIT 50`

	rows, err := s.db.Query(ctx, query, wsUUID)
	if err != nil {
		return nil, mcp.NewInternalError("failed to query agents")
	}
	defer rows.Close()

	agents := []map[string]any{}
	for rows.Next() {
		var id, workspaceID, runtimeID any
		var name, provider, status string
		var createdAt, updatedAt any

		if err := rows.Scan(&id, &workspaceID, &name, &provider, &status, &runtimeID, &createdAt, &updatedAt); err != nil {
			return nil, mcp.NewInternalError("failed to scan agent")
		}

		agents = append(agents, map[string]any{
			"id":           fmt.Sprintf("%v", id),
			"workspace_id": fmt.Sprintf("%v", workspaceID),
			"name":         name,
			"provider":     provider,
			"status":      status,
			"runtime_id":  fmt.Sprintf("%v", runtimeID),
			"created_at":  createdAt,
			"updated_at":  updatedAt,
		})
	}

	return map[string]any{"agents": agents, "total": len(agents)}, nil
}

// AgentGet gets a single agent
func (s *AgentService) AgentGet(ctx context.Context, params map[string]any) (any, error) {
	agentID, ok := params["agent_id"].(string)
	if !ok || agentID == "" {
		return nil, mcp.NewValidationError("agent_id is required")
	}

	id, err := uuid.Parse(agentID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid agent_id format")
	}

	query := `SELECT id, workspace_id, name, provider, status, runtime_id, created_at, updated_at
		FROM agent WHERE id = $1`

	var id2, workspaceID, runtimeID any
	var name, provider, status string
	var createdAt, updatedAt any

	err = s.db.QueryRow(ctx, query, id).Scan(&id2, &workspaceID, &name, &provider, &status, &runtimeID, &createdAt, &updatedAt)
	if err != nil {
		return nil, mcp.NewNotFoundError("agent not found")
	}

	return map[string]any{
		"id":           fmt.Sprintf("%v", id2),
		"workspace_id": fmt.Sprintf("%v", workspaceID),
		"name":         name,
		"provider":     provider,
		"status":      status,
		"runtime_id":  fmt.Sprintf("%v", runtimeID),
		"created_at":  createdAt,
		"updated_at":  updatedAt,
	}, nil
}

// Package-level wrappers
func AgentList(ctx mcp.ToolContext, params map[string]any) (any, error) {
	return NewAgentService(nil).AgentList(context.Background(), params)
}

func AgentGet(ctx mcp.ToolContext, params map[string]any) (any, error) {
	return NewAgentService(nil).AgentGet(context.Background(), params)
}