package mcp

import "testing"

func TestJSONRPCRequest(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  map[string]any{"workspace_id": "test"},
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("expected 2.0, got %s", req.JSONRPC)
	}
	if req.Method != "tools/list" {
		t.Errorf("expected tools/list, got %s", req.Method)
	}
}

func TestJSONRPCResponse(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  map[string]any{"tools": []any{}},
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected 2.0, got %s", resp.JSONRPC)
	}
}