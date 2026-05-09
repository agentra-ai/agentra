package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestTransportRoundTrip(t *testing.T) {
	// Test that we can encode and decode requests/responses
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  map[string]any{},
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(req); err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dec := json.NewDecoder(&buf)
	var decoded JSONRPCRequest
	if err := dec.Decode(&decoded); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if decoded.JSONRPC != "2.0" {
		t.Errorf("expected 2.0, got %s", decoded.JSONRPC)
	}
	if decoded.Method != "tools/list" {
		t.Errorf("expected tools/list, got %s", decoded.Method)
	}
}

func TestTransportEmptyLines(t *testing.T) {
	// Input starts with newlines - scanner returns empty tokens for leading newlines
	// Transport.Read() should skip empty lines and return the actual JSON payload
	input := "\n\n{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"test\",\"params\":{}}\n\n"
	reader := bytes.NewReader([]byte(input))
	transport := NewTransport(reader, io.Discard)

	// Read through empty lines until we get the actual request
	var req *JSONRPCRequest
	for i := 0; i < 10; i++ {
		r, err := transport.Read()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r != nil {
			req = r
			break
		}
	}

	if req == nil {
		t.Fatal("expected request, got nil after skipping empty lines")
	}
	if req.Method != "test" {
		t.Errorf("expected test, got %s", req.Method)
	}
}

func TestMCPErrorFormatting(t *testing.T) {
	err := NewValidationError("missing required field")

	if err.Code != ErrValidation {
		t.Errorf("expected VALIDATION_ERROR, got %s", err.Code)
	}

	if err.Message != "missing required field" {
		t.Errorf("expected 'missing required field', got %s", err.Message)
	}
}

func TestServerCapabilities(t *testing.T) {
	caps := ServerCapabilities{
		Tools:    ToolCapabilities{ListChanged: true},
		Resources: ResourceCapabilities{ListChanged: true},
		Prompts:  PromptCapabilities{ListChanged: true},
	}

	if !caps.Tools.ListChanged {
		t.Error("expected Tools.ListChanged to be true")
	}
	if !caps.Resources.ListChanged {
		t.Error("expected Resources.ListChanged to be true")
	}
	if !caps.Prompts.ListChanged {
		t.Error("expected Prompts.ListChanged to be true")
	}
}