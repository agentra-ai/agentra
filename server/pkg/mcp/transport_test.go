package mcp

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestTransportReadRequest(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	reader := bytes.NewReader([]byte(input + "\n"))

	var req JSONRPCRequest
	dec := json.NewDecoder(reader)
	if err := dec.Decode(&req); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("expected 2.0, got %s", req.JSONRPC)
	}
	if req.Method != "tools/list" {
		t.Errorf("expected tools/list, got %s", req.Method)
	}
}

func TestTransportWriteResponse(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  map[string]any{"tools": []any{}},
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(resp); err != nil {
		t.Fatalf("encode error: %v", err)
	}

	var decoded JSONRPCResponse
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.JSONRPC != "2.0" {
		t.Errorf("expected 2.0, got %s", decoded.JSONRPC)
	}
}