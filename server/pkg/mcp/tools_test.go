package mcp

import (
	"testing"
)

func TestToolRegistryRegister(t *testing.T) {
	registry := NewToolRegistry()

	tool := Tool{
		Name:        "agentra_issue_list",
		Description: "List issues in a workspace",
		InputSchema: ToolInputSchema{
			Type:       "object",
			Properties: map[string]any{},
			Required:   []string{"workspace_id"},
		},
	}

	registry.Register(tool, nil)

	if len(registry.tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(registry.tools))
	}

	retrieved, ok := registry.Get("agentra_issue_list")
	if !ok {
		t.Error("expected to retrieve tool")
	}
	if retrieved.Name != "agentra_issue_list" {
		t.Errorf("expected agentra_issue_list, got %s", retrieved.Name)
	}
}

func TestToolRegistryList(t *testing.T) {
	registry := NewToolRegistry()

	registry.Register(Tool{Name: "tool1", Description: "desc1", InputSchema: ToolInputSchema{}}, nil)
	registry.Register(Tool{Name: "tool2", Description: "desc2", InputSchema: ToolInputSchema{}}, nil)

	tools := registry.List()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}