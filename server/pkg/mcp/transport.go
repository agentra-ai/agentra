package mcp

import (
	"bufio"
	"encoding/json"
	"io"
)

// Transport handles reading and writing JSON-RPC messages over stdio
type Transport struct {
	reader *bufio.Scanner
	writer *json.Encoder
}

// NewTransport creates a new stdio transport
func NewTransport(r io.Reader, w io.Writer) *Transport {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // 10MB max
	return &Transport{
		reader: scanner,
		writer: json.NewEncoder(w),
	}
}

// Read reads the next JSON-RPC request
func (t *Transport) Read() (*JSONRPCRequest, error) {
	if !t.reader.Scan() {
		if err := t.reader.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}

	line := t.reader.Text()
	if line == "" {
		return nil, nil // skip empty lines
	}

	var req JSONRPCRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return nil, err
	}

	return &req, nil
}

// Write writes a JSON-RPC response
func (t *Transport) Write(resp *JSONRPCResponse) error {
	return t.writer.Encode(resp)
}

// WriteError writes a JSON-RPC error response
func (t *Transport) WriteError(id any, code int, message string) error {
	return t.writer.Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	})
}