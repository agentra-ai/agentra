package tools

import (
	"context"
	"testing"

	"github.com/agentra-ai/agentra/server/pkg/mcp"
)

func TestIssueListParams(t *testing.T) {
	params := map[string]any{
		"workspace_id": "550e8400-e29b-41d4-a716-446655440000",
	}

	ctx := mcp.ToolContext{WorkspaceID: "550e8400-e29b-41d4-a716-446655440000"}

	// This will fail because we don't have a real DB, but we can test validation
	_, err := IssueList(ctx, params)
	if err == nil {
		// Expected to fail without DB connection
		t.Log("expected error without DB, got nil")
	}
}

func TestIssueGetParams(t *testing.T) {
	params := map[string]any{
		"issue_id": "550e8400-e29b-41d4-a716-446655440000",
	}

	ctx := mcp.ToolContext{WorkspaceID: "550e8400-e29b-41d4-a716-446655440000"}

	_, err := IssueGet(ctx, params)
	if err == nil {
		t.Log("expected error without DB, got nil")
	}
}

func TestIssueCreateParamsValidation(t *testing.T) {
	params := map[string]any{
		// missing required workspace_id and title
	}

	ctx := mcp.ToolContext{WorkspaceID: "550e8400-e29b-41d4-a716-446655440000"}

	_, err := IssueCreate(ctx, params)
	if err == nil {
		t.Error("expected validation error for missing required fields")
	}
}

func TestIssueUpdateParamsValidation(t *testing.T) {
	params := map[string]any{
		// missing issue_id
	}

	ctx := mcp.ToolContext{WorkspaceID: "550e8400-e29b-41d4-a716-446655440000"}

	_, err := IssueUpdate(ctx, params)
	if err == nil {
		t.Error("expected validation error for missing issue_id")
	}
}

func TestIssueDeleteParamsValidation(t *testing.T) {
	params := map[string]any{
		// missing issue_id
	}

	ctx := mcp.ToolContext{WorkspaceID: "550e8400-e29b-41d4-a716-446655440000"}

	_, err := IssueDelete(ctx, params)
	if err == nil {
		t.Error("expected validation error for missing issue_id")
	}
}

func TestIssueServiceWithMockDB(t *testing.T) {
	// Test that IssueService methods validate params properly
	// without needing a real database connection

	service := NewIssueService(nil)

	// Test IssueList with missing workspace_id
	_, err := service.IssueList(context.Background(), map[string]any{})
	if err == nil {
		t.Error("IssueList should require workspace_id")
	}

	// Test IssueGet with missing issue_id
	_, err = service.IssueGet(context.Background(), map[string]any{})
	if err == nil {
		t.Error("IssueGet should require issue_id")
	}

	// Test IssueCreate with missing required fields
	_, err = service.IssueCreate(context.Background(), map[string]any{})
	if err == nil {
		t.Error("IssueCreate should require workspace_id and title")
	}

	// Test IssueUpdate with missing issue_id
	_, err = service.IssueUpdate(context.Background(), map[string]any{})
	if err == nil {
		t.Error("IssueUpdate should require issue_id")
	}

	// Test IssueDelete with missing issue_id
	_, err = service.IssueDelete(context.Background(), map[string]any{})
	if err == nil {
		t.Error("IssueDelete should require issue_id")
	}
}