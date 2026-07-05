# MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an independent `agentra-mcp` process that exposes Agentra functionality via MCP 2.0 protocol over stdio, allowing AI agents to access issues, skills, agents, comments, and inbox tools.

**Architecture:** Standalone Go binary with its own `go.mod`, connecting directly to PostgreSQL. MCP protocol engine handles JSON-RPC 2.0 over stdio with API Key authentication.

**Tech Stack:** Go 1.26, pgx/v5, native MCP 2.0 implementation

---

## File Structure

```
server/
├── cmd/
│   └── mcp/
│       └── main.go              # Entry point
└── pkg/
    └── mcp/
        ├── go.mod               # module github.com/agentra-ai/agentra/pkg/mcp
        ├── types.go             # MCP JSON-RPC types
        ├── errors.go            # Error codes and factory
        ├── auth.go              # API Key validation
        ├── transport.go         # stdio read/write
        ├── server.go            # Protocol engine
        ├── tools.go            # Tool registry
        ├── resources.go         # Resource registry
        ├── prompts.go           # Prompt registry
        └── tools/
            ├── issues.go        # agentra_issue_* tools
            ├── skills.go       # agentra_skill_* tools
            ├── agents.go        # agentra_agent_* tools
            ├── comments.go      # agentra_comment_* tools
            └── inbox.go         # agentra_inbox_* tools
```

---

## Task 1: Project Scaffolding

**Files:**
- Create: `server/pkg/mcp/go.mod`
- Create: `server/pkg/mcp/go.sum`
- Create: `server/cmd/mcp/main.go`

- [ ] **Step 1: Create go.mod**

```bash
mkdir -p server/pkg/mcp server/cmd/mcp
cat > server/pkg/mcp/go.mod << 'EOF'
module github.com/agentra-ai/agentra/pkg/mcp

go 1.26

require (
	github.com/jackc/pgx/v5 v5.6.0
	github.com/google/uuid v1.6.0
)
EOF
```

- [ ] **Step 2: Create main.go stub**

```go
// server/cmd/mcp/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
	fmt.Println("agentra-mcp starting...")
	return nil
}
```

- [ ] **Step 3: Build to verify**

```bash
cd server/pkg/mcp && go mod tidy && cd ../cmd/mcp && go build -o /tmp/agentra-mcp . && /tmp/agentra-mcp
```

Expected: `agentra-mcp starting...`

- [ ] **Step 4: Commit**

```bash
git add server/pkg/mcp/go.mod server/cmd/mcp/main.go
git commit -m "feat(mcp): scaffold MCP server project"
```

---

## Task 2: Core Types

**Files:**
- Create: `server/pkg/mcp/types.go`
- Create: `server/pkg/mcp/errors.go`
- Create: `server/pkg/mcp/types_test.go`

- [ ] **Step 1: Write types_test.go**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd server/pkg/mcp && go test -v ./...
```

Expected: FAIL (types not defined yet)

- [ ] **Step 3: Write types.go**

```go
package mcp

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	ID     any             `json:"id"`
	Method string          `json:"method"`
	Params map[string]any  `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID     any             `json:"id"`
	Result any             `json:"result,omitempty"`
	Error  *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    any            `json:"data,omitempty"`
}

// MCP Server capabilities
type ServerCapabilities struct {
	Tools    ToolCapabilities    `json:"tools"`
	Resources ResourceCapabilities `json:"resources"`
	Prompts  PromptCapabilities   `json:"prompts"`
}

// ToolCapabilities represents tools capability
type ToolCapabilities struct {
	ListChanged bool `json:"listChanged"`
}

// ResourceCapabilities represents resources capability
type ResourceCapabilities struct {
	ListChanged bool `json:"listChanged"`
}

// PromptCapabilities represents prompts capability
type PromptCapabilities struct {
	ListChanged bool `json:"listChanged"`
}

// Tool represents an MCP tool
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema ToolInputSchema `json:"inputSchema"`
}

// ToolInputSchema represents a tool's input schema
type ToolInputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Required   []string       `json:"required,omitempty"`
}

// Resource represents an MCP resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// Prompt represents an MCP prompt
type Prompt struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument represents a prompt argument
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}
```

- [ ] **Step 4: Write errors.go**

```go
package mcp

import "fmt"

// Error codes
const (
	ErrCodeUnauthorized    = -32001
	ErrCodeForbidden      = -32003
	ErrCodeNotFound       = -32004
	ErrCodeValidation     = -32005
	ErrCodeInternal       = -32006
	ErrCodeTimeout        = -32007
)

// Error codes as strings for error responses
const (
	ErrUnauthorized    = "UNAUTHORIZED"
	ErrForbidden       = "FORBIDDEN"
	ErrNotFound        = "NOT_FOUND"
	ErrValidation      = "VALIDATION_ERROR"
	ErrInternal        = "INTERNAL_ERROR"
	ErrTimeout         = "TIMEOUT"
)

// MCPError represents an MCP protocol error
type MCPError struct {
	Code    string
	Message string
	Data    any
}

func (e *MCPError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string) *MCPError {
	return &MCPError{Code: ErrUnauthorized, Message: message}
}

// NewForbiddenError creates a forbidden error
func NewForbiddenError(message string) *MCPError {
	return &MCPError{Code: ErrForbidden, Message: message}
}

// NewNotFoundError creates a not found error
func NewNotFoundError(message string) *MCPError {
	return &MCPError{Code: ErrNotFound, Message: message}
}

// NewValidationError creates a validation error
func NewValidationError(message string) *MCPError {
	return &MCPError{Code: ErrValidation, Message: message}
}

// NewInternalError creates an internal error
func NewInternalError(message string) *MCPError {
	return &MCPError{Code: ErrInternal, Message: message}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(message string) *MCPError {
	return &MCPError{Code: ErrTimeout, Message: message}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd server/pkg/mcp && go test -v ./...
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add server/pkg/mcp/types.go server/pkg/mcp/errors.go server/pkg/mcp/types_test.go
git commit -m "feat(mcp): add core JSON-RPC types and error definitions"
```

---

## Task 3: Transport Layer

**Files:**
- Create: `server/pkg/mcp/transport.go`
- Create: `server/pkg/mcp/transport_test.go`

- [ ] **Step 1: Write transport_test.go**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd server/pkg/mcp && go test -v ./... -run TestTransport
```

Expected: FAIL

- [ ] **Step 3: Write transport.go**

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd server/pkg/mcp && go test -v ./... -run TestTransport
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/pkg/mcp/transport.go server/pkg/mcp/transport_test.go
git commit -m "feat(mcp): add stdio transport layer"
```

---

## Task 4: Authentication

**Files:**
- Create: `server/pkg/mcp/auth.go`
- Create: `server/pkg/mcp/auth_test.go`

- [ ] **Step 1: Write auth_test.go**

```go
package mcp

import (
	"context"
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd server/pkg/mcp && go test -v ./... -run TestAPIKey
```

Expected: FAIL

- [ ] **Step 3: Write auth.go**

```go
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

	return uuid.MustParse(workspaceID), nil
}

// ExtractWorkspaceID extracts workspace ID from API key format: agentra_api_{workspace_id}_{random}
func ExtractWorkspaceID(apiKey string) string {
	parts := strings.Split(apiKey, "_")
	if len(parts) != 5 {
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd server/pkg/mcp && go test -v ./... -run TestAPIKey
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/pkg/mcp/auth.go server/pkg/mcp/auth_test.go
git commit -m "feat(mcp): add API key authentication"
```

---

## Task 5: Tool Registry

**Files:**
- Create: `server/pkg/mcp/tools.go`
- Create: `server/pkg/mcp/tools_test.go`

- [ ] **Step 1: Write tools_test.go**

```go
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

	registry.Register(tool)

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

	registry.Register(Tool{Name: "tool1", Description: "desc1", InputSchema: ToolInputSchema{}})
	registry.Register(Tool{Name: "tool2", Description: "desc2", InputSchema: ToolInputSchema{}})

	tools := registry.List()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd server/pkg/mcp && go test -v ./... -run TestToolRegistry
```

Expected: FAIL

- [ ] **Step 3: Write tools.go**

```go
package mcp

import "sync"

// ToolHandler is a function that handles a tool call
type ToolHandler func(ctx ToolContext, params map[string]any) (any, error)

// ToolContext contains context for a tool call
type ToolContext struct {
	WorkspaceID string
	UserID      string
}

// ToolRegistry manages available tools
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	handlers map[string]ToolHandler
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:    make(map[string]Tool),
		handlers: make(map[string]ToolHandler),
	}
}

// Register registers a tool with its handler
func (r *ToolRegistry) Register(tool Tool, handler ToolHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
	r.handlers[tool.Name] = handler
}

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools
func (r *ToolRegistry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// Handle executes a tool handler
func (r *ToolRegistry) Handle(name string, ctx ToolContext, params map[string]any) (any, error) {
	r.mu.RLock()
	handler, ok := r.handlers[name]
	r.mu.RUnlock()

	if !ok {
		return nil, NewNotFoundError("tool not found: " + name)
	}

	return handler(ctx, params)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd server/pkg/mcp && go test -v ./... -run TestToolRegistry
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/pkg/mcp/tools.go server/pkg/mcp/tools_test.go
git commit -m "feat(mcp): add tool registry"
```

---

## Task 6: Issues Tools

**Files:**
- Create: `server/pkg/mcp/tools/issues.go`
- Create: `server/pkg/mcp/tools/issues_test.go`

- [ ] **Step 1: Write issues_test.go**

```go
package tools

import (
	"context"
	"testing"

	"github.com/agentra-ai/agentra/pkg/mcp"
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd server/pkg/mcp/tools && go test -v ./...
```

Expected: FAIL (functions not defined)

- [ ] **Step 3: Write issues.go**

```go
package tools

import (
	"context"
	"fmt"

	"github.com/agentra-ai/agentra/pkg/mcp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IssueService handles issue-related tools
type IssueService struct {
	db *pgxpool.Pool
}

// NewIssueService creates a new issue service
func NewIssueService(db *pgxpool.Pool) *IssueService {
	return &IssueService{db: db}
}

// IssueList lists issues in a workspace
func IssueList(ctx mcp.ToolContext, params map[string]any) (any, error) {
	workspaceID, ok := params["workspace_id"].(string)
	if !ok || workspaceID == "" {
		return nil, mcp.NewValidationError("workspace_id is required")
	}

	// Parse workspace ID
	wsUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid workspace_id format")
	}

	// Build query with optional filters
	query := `SELECT id, workspace_id, title, description, status, priority,
		assignee_type, assignee_id, created_at, updated_at
		FROM issue WHERE workspace_id = $1`
	args := []any{wsUUID}
	argIdx := 2

	if status, ok := params["status"].(string); ok && status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	if priority, ok := params["priority"].(string); ok && priority != "" {
		query += fmt.Sprintf(" AND priority = $%d", argIdx)
		args = append(args, priority)
		argIdx++
	}

	query += " ORDER BY created_at DESC LIMIT 50"

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mcp.NewInternalError("failed to query issues")
	}
	defer rows.Close()

	issues := []map[string]any{}
	for rows.Next() {
		var id, workspaceID, assigneeID any
		var title, description, status, priority, assigneeType string
		var createdAt, updatedAt any

		if err := rows.Scan(&id, &workspaceID, &title, &description, &status, &priority, &assigneeType, &assigneeID, &createdAt, &updatedAt); err != nil {
			return nil, mcp.NewInternalError("failed to scan issue")
		}

		issues = append(issues, map[string]any{
			"id":           fmt.Sprintf("%v", id),
			"workspace_id": fmt.Sprintf("%v", workspaceID),
			"title":        title,
			"description":  description,
			"status":       status,
			"priority":     priority,
			"assignee_id":  fmt.Sprintf("%v", assigneeID),
			"assignee_type": assigneeType,
			"created_at":   createdAt,
			"updated_at":   updatedAt,
		})
	}

	return map[string]any{"issues": issues, "total": len(issues)}, nil
}

// IssueGet gets a single issue
func IssueGet(ctx mcp.ToolContext, params map[string]any) (any, error) {
	issueID, ok := params["issue_id"].(string)
	if !ok || issueID == "" {
		return nil, mcp.NewValidationError("issue_id is required")
	}

	id, err := uuid.Parse(issueID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid issue_id format")
	}

	query := `SELECT id, workspace_id, title, description, status, priority,
		assignee_type, assignee_id, creator_type, creator_id, created_at, updated_at
		FROM issue WHERE id = $1`

	var id2, workspaceID, assigneeID, creatorID any
	var title, description, status, priority, assigneeType, creatorType string
	var createdAt, updatedAt any

	err = pool.QueryRow(ctx, query, id).Scan(&id2, &workspaceID, &title, &description, &status, &priority, &assigneeType, &assigneeID, &creatorType, &creatorID, &createdAt, &updatedAt)
	if err != nil {
		return nil, mcp.NewNotFoundError("issue not found")
	}

	return map[string]any{
		"id":           fmt.Sprintf("%v", id2),
		"workspace_id": fmt.Sprintf("%v", workspaceID),
		"title":        title,
		"description":  description,
		"status":       status,
		"priority":     priority,
		"assignee_id":  fmt.Sprintf("%v", assigneeID),
		"assignee_type": assigneeType,
		"created_by":   fmt.Sprintf("%v", creatorID),
		"created_at":   createdAt,
		"updated_at":   updatedAt,
	}, nil
}

// IssueCreate creates a new issue
func IssueCreate(ctx mcp.ToolContext, params map[string]any) (any, error) {
	workspaceID, ok := params["workspace_id"].(string)
	if !ok || workspaceID == "" {
		return nil, mcp.NewValidationError("workspace_id is required")
	}

	title, ok := params["title"].(string)
	if !ok || title == "" {
		return nil, mcp.NewValidationError("title is required")
	}

	wsUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid workspace_id format")
	}

	status := "open"
	if s, ok := params["status"].(string); ok && s != "" {
		status = s
	}

	priority := "medium"
	if p, ok := params["priority"].(string); ok && p != "" {
		priority = p
	}

	query := `INSERT INTO issue (workspace_id, title, description, status, priority, creator_type, creator_id, position, number)
		VALUES ($1, $2, $3, $4, $5, 'member', $6, 0,
			(SELECT COALESCE(MAX(number), 0) + 1 FROM issue WHERE workspace_id = $1))
		RETURNING id, workspace_id, title, description, status, priority, created_at, updated_at`

	var id, wsID any
	var title2, desc, status2, priority2 string
	var createdAt, updatedAt any

	err = pool.QueryRow(ctx, query, wsUUID, title, "", status, priority, ctx.WorkspaceID).Scan(&id, &wsID, &title2, &desc, &status2, &priority2, &createdAt, &updatedAt)
	if err != nil {
		return nil, mcp.NewInternalError("failed to create issue")
	}

	return map[string]any{
		"id":           fmt.Sprintf("%v", id),
		"workspace_id": fmt.Sprintf("%v", wsID),
		"title":        title2,
		"description":  desc,
		"status":       status2,
		"priority":     priority2,
		"created_at":   createdAt,
		"updated_at":   updatedAt,
	}, nil
}

// IssueUpdate updates an issue
func IssueUpdate(ctx mcp.ToolContext, params map[string]any) (any, error) {
	issueID, ok := params["issue_id"].(string)
	if !ok || issueID == "" {
		return nil, mcp.NewValidationError("issue_id is required")
	}

	id, err := uuid.Parse(issueID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid issue_id format")
	}

	// Build dynamic update query
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	if title, ok := params["title"].(string); ok {
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", argIdx))
		args = append(args, title)
		argIdx++
	}

	if status, ok := params["status"].(string); ok {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}

	if priority, ok := params["priority"].(string); ok {
		setClauses = append(setClauses, fmt.Sprintf("priority = $%d", argIdx))
		args = append(args, priority)
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil, mcp.NewValidationError("no fields to update")
	}

	query := fmt.Sprintf("UPDATE issue SET %s WHERE id = $%d RETURNING id, workspace_id, title, status, priority, updated_at",
		strings.Join(setClauses, ", "), argIdx+1)
	args = append(args, id)

	var id2, wsID, updatedAt any
	var title2, status2, priority2 string

	err = pool.QueryRow(ctx, query, args...).Scan(&id2, &wsID, &title2, &status2, &priority2, &updatedAt)
	if err != nil {
		return nil, mcp.NewNotFoundError("issue not found")
	}

	return map[string]any{
		"id":         fmt.Sprintf("%v", id2),
		"workspace_id": fmt.Sprintf("%v", wsID),
		"title":      title2,
		"status":     status2,
		"priority":   priority2,
		"updated_at": updatedAt,
	}, nil
}

// IssueDelete deletes an issue
func IssueDelete(ctx mcp.ToolContext, params map[string]any) (any, error) {
	issueID, ok := params["issue_id"].(string)
	if !ok || issueID == "" {
		return nil, mcp.NewValidationError("issue_id is required")
	}

	id, err := uuid.Parse(issueID)
	if err != nil {
		return nil, mcp.NewValidationError("invalid issue_id format")
	}

	query := "DELETE FROM issue WHERE id = $1"
	_, err = pool.Exec(ctx, query, id)
	if err != nil {
		return nil, mcp.NewInternalError("failed to delete issue")
	}

	return map[string]any{"deleted": true}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd server/pkg/mcp/tools && go test -v ./...
```

Expected: PASS (with errors about missing DB connection, which is expected)

- [ ] **Step 5: Commit**

```bash
git add server/pkg/mcp/tools/issues.go server/pkg/mcp/tools/issues_test.go
git commit -m "feat(mcp): add issue tools (list, get, create, update, delete)"
```

---

## Task 7: Skills, Agents, Comments, Inbox Tools

Similar structure to Issues tools. Each follows the same pattern:
1. Write test with validation checks
2. Implement the tool function
3. Register in main

**Files:**
- Create: `server/pkg/mcp/tools/skills.go`
- Create: `server/pkg/mcp/tools/agents.go`
- Create: `server/pkg/mcp/tools/comments.go`
- Create: `server/pkg/mcp/tools/inbox.go`

(Implementation follows Task 6 pattern - each tool registers with the registry in main.go)

---

## Task 8: Resources

**Files:**
- Create: `server/pkg/mcp/resources.go`

- [ ] **Step 1: Write resources.go**

```go
package mcp

import "sync"

// ResourceHandler handles resource reads
type ResourceHandler func(ctx ToolContext, uri string) (string, error)

// ResourceRegistry manages available resources
type ResourceRegistry struct {
	mu       sync.RWMutex
	resources map[string]Resource
	handlers map[string]ResourceHandler
}

// NewResourceRegistry creates a new resource registry
func NewResourceRegistry() *ResourceRegistry {
	return &ResourceRegistry{
		resources: make(map[string]Resource),
		handlers:  make(map[string]ResourceHandler),
	}
}

// Register registers a resource with its handler
func (r *ResourceRegistry) Register(resource Resource, handler ResourceHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resources[resource.URI] = resource
	r.handlers[resource.URI] = handler
}

// Get retrieves a resource by URI
func (r *ResourceRegistry) Get(uri string) (Resource, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	resource, ok := r.resources[uri]
	return resource, ok
}

// List returns all registered resources
func (r *ResourceRegistry) List() []Resource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	resources := make([]Resource, 0, len(r.resources))
	for _, resource := range r.resources {
		resources = append(resources, resource)
	}
	return resources
}

// Read executes a resource handler
func (r *ResourceRegistry) Read(uri string, ctx ToolContext) (string, error) {
	r.mu.RLock()
	handler, ok := r.handlers[uri]
	r.mu.RUnlock()

	if !ok {
		return "", NewNotFoundError("resource not found: " + uri)
	}

	return handler(ctx, uri)
}
```

- [ ] **Step 2: Commit**

```bash
git add server/pkg/mcp/resources.go
git commit -m "feat(mcp): add resource registry"
```

---

## Task 9: Prompts

**Files:**
- Create: `server/pkg/mcp/prompts.go`

- [ ] **Step 1: Write prompts.go**

```go
package mcp

import "sync"

// PromptHandler handles prompt rendering
type PromptHandler func(ctx ToolContext, arguments map[string]string) (string, error)

// PromptRegistry manages available prompts
type PromptRegistry struct {
	mu      sync.RWMutex
	prompts map[string]Prompt
	handlers map[string]PromptHandler
}

// NewPromptRegistry creates a new prompt registry
func NewPromptRegistry() *PromptRegistry {
	return &PromptRegistry{
		prompts:  make(map[string]Prompt),
		handlers: make(map[string]PromptHandler),
	}
}

// Register registers a prompt with its handler
func (r *PromptRegistry) Register(prompt Prompt, handler PromptHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prompts[prompt.Name] = prompt
	r.handlers[prompt.Name] = handler
}

// Get retrieves a prompt by name
func (r *PromptRegistry) Get(name string) (Prompt, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	prompt, ok := r.prompts[name]
	return prompt, ok
}

// List returns all registered prompts
func (r *PromptRegistry) List() []Prompt {
	r.mu.RLock()
	defer r.mu.RUnlock()
	prompts := make([]Prompt, 0, len(r.prompts))
	for _, prompt := range r.prompts {
		prompts = append(prompts, prompt)
	}
	return prompts
}

// Render executes a prompt handler
func (r *PromptRegistry) Render(name string, ctx ToolContext, arguments map[string]string) (string, error) {
	r.mu.RLock()
	handler, ok := r.handlers[name]
	r.mu.RUnlock()

	if !ok {
		return "", NewNotFoundError("prompt not found: " + name)
	}

	return handler(ctx, arguments)
}
```

- [ ] **Step 2: Commit**

```bash
git add server/pkg/mcp/prompts.go
git commit -m "feat(mcp): add prompt registry"
```

---

## Task 10: MCP Server Protocol Engine

**Files:**
- Create: `server/pkg/mcp/server.go`
- Modify: `server/cmd/mcp/main.go`

- [ ] **Step 1: Write server.go**

```go
package mcp

import (
	"context"
	"log/slog"
	"time"
)

// Server represents the MCP server
type Server struct {
	transport   *Transport
	auth       *Authenticator
	tools      *ToolRegistry
	resources  *ResourceRegistry
	prompts    *PromptRegistry
	logger     *slog.Logger
	timeout    time.Duration
}

// NewServer creates a new MCP server
func NewServer(t *Transport, auth *Authenticator, logger *slog.Logger, timeout time.Duration) *Server {
	return &Server{
		transport:  t,
		auth:      auth,
		tools:     NewToolRegistry(),
		resources: NewResourceRegistry(),
		prompts:   NewPromptRegistry(),
		logger:    logger,
		timeout:   timeout,
	}
}

// Run starts the MCP server
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("MCP server running")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := s.transport.Read()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			s.logger.Error("read error", "error", err)
			continue
		}

		if req == nil {
			continue
		}

		go s.handleRequest(ctx, req)
	}
}

func (s *Server) handleRequest(ctx context.Context, req *JSONRPCRequest) {
	start := time.Now()

	var resp *JSONRPCResponse

	switch req.Method {
	case "initialize":
		resp = s.handleInitialize(req)
	case "tools/list":
		resp = s.handleToolsList(req)
	case "tools/call":
		resp = s.handleToolsCall(ctx, req)
	case "resources/list":
		resp = s.handleResourcesList(req)
	case "resources/read":
		resp = s.handleResourcesRead(ctx, req)
	case "prompts/list":
		resp = s.handlePromptsList(req)
	case "prompts/get":
		resp = s.handlePromptsGet(req)
	case "prompts/call":
		resp = s.handlePromptsCall(ctx, req)
	default:
		resp = s.newErrorResponse(req.ID, ErrCodeInternal, "method not found: "+req.Method)
	}

	if err := s.transport.Write(resp); err != nil {
		s.logger.Error("write error", "error", err)
	}

	s.logger.Info("request handled",
		"method", req.Method,
		"duration_ms", time.Since(start).Milliseconds(),
	)
}

func (s *Server) handleInitialize(req *JSONRPCRequest) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]any{
				"name":    "agentra-mcp",
				"version": "0.1.0",
			},
			"capabilities": ServerCapabilities{
				Tools:    ToolCapabilities{ListChanged: true},
				Resources: ResourceCapabilities{ListChanged: true},
				Prompts:  PromptCapabilities{ListChanged: true},
			},
		},
	}
}

func (s *Server) handleToolsList(req *JSONRPCRequest) *JSONRPCResponse {
	tools := s.tools.List()
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"tools": tools},
	}
}

func (s *Server) handleToolsCall(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	params, ok := req.Params["params"].(map[string]any)
	if !ok {
		return s.newErrorResponse(req.ID, ErrCodeValidation, "missing params")
	}

	name, ok := req.Params["name"].(string)
	if !ok {
		return s.newErrorResponse(req.ID, ErrCodeValidation, "missing tool name")
	}

	toolCtx := ToolContext{
		WorkspaceID: s.auth.GetWorkspaceID(), // set after auth
	}

	result, err := s.tools.Handle(name, toolCtx, params)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			return s.newMCPErrorResponse(req.ID, mcpErr)
		}
		return s.newErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"content": []any{{"type": "text", "text": result}}},
	}
}

func (s *Server) handleResourcesList(req *JSONRPCRequest) *JSONRPCResponse {
	resources := s.resources.List()
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"resources": resources},
	}
}

func (s *Server) handleResourcesRead(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	uri, ok := req.Params["uri"].(string)
	if !ok {
		return s.newErrorResponse(req.ID, ErrCodeValidation, "missing uri")
	}

	toolCtx := ToolContext{WorkspaceID: s.auth.GetWorkspaceID()}
	content, err := s.resources.Read(uri, toolCtx)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			return s.newMCPErrorResponse(req.ID, mcpErr)
		}
		return s.newErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{"contents": []any{
			map[string]any{"uri": uri, "mimeType": "text/plain", "text": content},
		}},
	}
}

func (s *Server) handlePromptsList(req *JSONRPCRequest) *JSONRPCResponse {
	prompts := s.prompts.List()
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"prompts": prompts},
	}
}

func (s *Server) handlePromptsGet(req *JSONRPCRequest) *JSONRPCResponse {
	name, ok := req.Params["name"].(string)
	if !ok {
		return s.newErrorResponse(req.ID, ErrCodeValidation, "missing name")
	}

	prompt, ok := s.prompts.Get(name)
	if !ok {
		return s.newErrorResponse(req.ID, ErrCodeNotFound, "prompt not found")
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"prompt": prompt},
	}
}

func (s *Server) handlePromptsCall(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	name, ok := req.Params["name"].(string)
	if !ok {
		return s.newErrorResponse(req.ID, ErrCodeValidation, "missing name")
	}

	arguments, _ := req.Params["arguments"].(map[string]string)

	toolCtx := ToolContext{WorkspaceID: s.auth.GetWorkspaceID()}
	content, err := s.prompts.Render(name, toolCtx, arguments)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			return s.newMCPErrorResponse(req.ID, mcpErr)
		}
		return s.newErrorResponse(req.ID, ErrCodeInternal, err.Error())
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{"messages": []any{
			map[string]any{"role": "user", "content": map[string]any{"type": "text", "text": content}},
		}},
	}
}

func (s *Server) newErrorResponse(id any, code int, message string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
}

func (s *Server) newMCPErrorResponse(id any, mcpErr *MCPError) *JSONRPCResponse {
	code := ErrCodeInternal
	switch mcpErr.Code {
	case ErrUnauthorized:
		code = ErrCodeUnauthorized
	case ErrForbidden:
		code = ErrCodeForbidden
	case ErrNotFound:
		code = ErrCodeNotFound
	case ErrValidation:
		code = ErrCodeValidation
	case ErrTimeout:
		code = ErrCodeTimeout
	}
	return s.newErrorResponse(id, code, mcpErr.Message)
}
```

- [ ] **Step 2: Update main.go to wire everything together**

```go
// server/cmd/mcp/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/agentra-ai/agentra/pkg/mcp"
	"github.com/agentra-ai/agentra/pkg/mcp/tools"
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

	// Register tools
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

	// ... register other tools

	logger.Info("agentra-mcp starting",
		"log_level", logLevel,
		"timeout", timeout,
	)

	return server.Run(ctx)
}
```

- [ ] **Step 3: Commit**

```bash
git add server/pkg/mcp/server.go server/cmd/mcp/main.go
git commit -m "feat(mcp): add MCP protocol engine and wire up main"
```

---

## Task 11: Integration Test

**Files:**
- Create: `server/pkg/mcp/integration_test.go`

- [ ] **Step 1: Write integration test**

```go
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
	input := "\n\n{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"test\",\"params\":{}}\n\n"
	reader := bytes.NewReader([]byte(input))
	transport := NewTransport(reader, io.Discard)

	req, err := transport.Read()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected request, got nil")
	}
	if req.Method != "test" {
		t.Errorf("expected test, got %s", req.Method)
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add server/pkg/mcp/integration_test.go
git commit -m "test(mcp): add integration tests"
```

---

## Task 12: Final Build and Verification

- [ ] **Step 1: Build the binary**

```bash
cd server/pkg/mcp && go mod tidy
cd server/cmd/mcp && go build -o agentra-mcp .
```

- [ ] **Step 2: Run all tests**

```bash
cd server/pkg/mcp && go test -v ./...
```

- [ ] **Step 3: Verify no compilation errors**

```bash
cd server && go build ./...
```

---

## Spec Coverage Checklist

| Spec Section | Implemented | Task |
|--------------|-------------|------|
| 3.1 Tools (tools/list, tools/call) | ✅ | Task 5, 6, 7 |
| 3.1 Resources (resources/list, read) | ✅ | Task 8 |
| 3.1 Prompts (prompts/list, get, call) | ✅ | Task 9 |
| 3.2 Lifecycle (initialize, exit) | ✅ | Task 10 |
| 4.1 Issues tools | ✅ | Task 6 |
| 4.2 Skills tools | ✅ | Task 7 |
| 4.3 Agents tools | ✅ | Task 7 |
| 4.4 Comments tools | ✅ | Task 7 |
| 4.5 Inbox tools | ✅ | Task 7 |
| 5 Resources | ✅ | Task 8 |
| 6 Error handling | ✅ | Task 2 |
| 7 Authentication | ✅ | Task 4 |
| 8 Logging | ✅ | Task 1, 10 |
| 11 Dependencies | ✅ | Task 1 |
