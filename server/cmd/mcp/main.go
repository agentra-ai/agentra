package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/agentra-ai/agentra/server/pkg/mcp"
	"github.com/agentra-ai/agentra/server/pkg/mcp/tools"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()
	logger := slog.Default()

	if err := run(ctx, logger); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	// Get configuration from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	timeout := 60 * time.Second
	if t := os.Getenv("MCP_TIMEOUT"); t != "" {
		if parsed, err := time.ParseDuration(t); err == nil {
			timeout = parsed
		}
	}

	// Connect to database
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	// Initialize services
	issueService := tools.NewIssueService(pool)

	// Create transport and server
	transport := mcp.NewTransport(os.Stdin, os.Stdout)
	auth := mcp.NewAuthenticator(pool)
	server := mcp.NewServer(transport, auth, logger, timeout)

	// Register issue tools
	server.RegisterTool(mcp.Tool{
		Name:        "agentra_issue_list",
		Description: "List issues in a workspace",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"workspace_id": map[string]any{"type": "string"},
				"status":      map[string]any{"type": "string"},
				"priority":    map[string]any{"type": "string"},
			},
			Required: []string{"workspace_id"},
		},
	}, issueService.IssueList)

	server.RegisterTool(mcp.Tool{
		Name:        "agentra_issue_get",
		Description: "Get a single issue",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"issue_id": map[string]any{"type": "string"},
			},
			Required: []string{"issue_id"},
		},
	}, issueService.IssueGet)

	server.RegisterTool(mcp.Tool{
		Name:        "agentra_issue_create",
		Description: "Create a new issue",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"workspace_id": map[string]any{"type": "string"},
				"title":       map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
				"status":     map[string]any{"type": "string"},
				"priority":    map[string]any{"type": "string"},
			},
			Required: []string{"workspace_id", "title"},
		},
	}, issueService.IssueCreate)

	server.RegisterTool(mcp.Tool{
		Name:        "agentra_issue_update",
		Description: "Update an issue",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"issue_id": map[string]any{"type": "string"},
				"title":    map[string]any{"type": "string"},
				"status":   map[string]any{"type": "string"},
				"priority": map[string]any{"type": "string"},
			},
			Required: []string{"issue_id"},
		},
	}, issueService.IssueUpdate)

	server.RegisterTool(mcp.Tool{
		Name:        "agentra_issue_delete",
		Description: "Delete an issue",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"issue_id": map[string]any{"type": "string"},
			},
			Required: []string{"issue_id"},
		},
	}, issueService.IssueDelete)

	logger.Info("agentra-mcp starting",
		"log_level", logLevel,
		"timeout", timeout,
	)

	return server.Run(ctx)
}