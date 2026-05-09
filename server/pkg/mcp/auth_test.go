package mcp

import (
	"testing"
)

func TestAPIKeyFormat(t *testing.T) {
	key := "agentra_api_550e8400-e29b-41d4-a716-446655440000_abcdef1234567890abcdef1234567890"

	workspaceID := ExtractWorkspaceID(key)
	if workspaceID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("expected workspace ID, got %s", workspaceID)
	}
}

func TestAPIKeyFormatInvalid(t *testing.T) {
	key := "invalid_key_format"

	workspaceID := ExtractWorkspaceID(key)
	if workspaceID != "" {
		t.Errorf("expected empty, got %s", workspaceID)
	}
}

func TestExtractWorkspaceID(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "valid key",
			key:      "agentra_api_550e8400-e29b-41d4-a716-446655440000_abcdef1234567890",
			expected: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "invalid prefix",
			key:      "invalid_550e8400-e29b-41d4-a716-446655440000_xxx",
			expected: "",
		},
		{
			name:     "too few parts",
			key:      "agentra_api_550e8400",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractWorkspaceID(tt.key)
			if got != tt.expected {
				t.Errorf("ExtractWorkspaceID(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}