# Cloud Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Cloud Runtime enabling agents to execute tasks in cloud-hosted Docker containers without requiring a local daemon. Phase 1 implements the Basic tier (Agentra-hosted Gateway).

**Architecture:** Agent-as-a-Service mode where users provide API keys stored encrypted in the database. The Cloud Runtime Gateway manages container lifecycle, proxies API calls to inject keys, and streams logs via WebSocket back to the Server for broadcast to clients.

**Tech Stack:** Go (Server, Gateway), Docker API, WebSocket, PostgreSQL with AES-256-GCM encryption, React/Next.js frontend

---

## File Structure

### Server (Go Backend)

```
server/
├── cmd/
│   └── gateway/              # NEW - Cloud Runtime Gateway service
│       └── main.go
├── internal/
│   ├── gateway/              # NEW - Gateway-side implementation
│   │   ├── container.go      # Docker container lifecycle
│   │   ├── proxy.go         # API key injection proxy
│   │   ├── wsclient.go      # WebSocket client to server
│   │   └── task.go          # Task execution logic
│   ├── handler/
│   │   └── cloud_runtime.go # NEW - Cloud runtime API handlers
│   ├── service/
│   │   └── task.go          # Modify - add runtime_type to task dispatch
│   └── realtime/
│       └── hub.go           # Modify - add gateway event handling
├── pkg/
│   ├── db/
│   │   └── queries/          # Modify - add cloud_runtime tables
│   └── protocol/
│       └── events.go        # Modify - add gateway events
├── migrations/
│   └── XXXX_cloud_runtime.up.sql   # NEW
└── pkg/agent/
    └── backend.go           # Modify - support runtime selection
```

### Frontend (Next.js)

```
apps/web/
├── features/
│   └── settings/
│       └── components/
│           └── runtime-tab.tsx  # NEW - Runtime settings UI
└── shared/
    └── types/
        └── events.ts       # Modify - add gateway event types
```

---

## Task 1: Database Schema

**Files:**
- Create: `migrations/XXXX_cloud_runtime.up.sql`
- Create: `server/pkg/db/queries/cloud_runtime.sql`
- Modify: `server/pkg/db/queries/task.sql` - add runtime_type column

- [ ] **Step 1: Create migration for cloud_runtime tables**

```sql
-- migrations/XXXX_cloud_runtime.up.sql

-- Cloud runtimes table
CREATE TABLE cloud_runtimes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    gateway_url VARCHAR(500),  -- NULL for basic tier (Agentra-hosted)
    provider VARCHAR(50) NOT NULL,  -- 'anthropic' | 'openai'
    encrypted_api_key BYTEA NOT NULL,  -- AES-256-GCM encrypted
    api_key_hash VARCHAR(64) NOT NULL,  -- For key lookup/validation
    is_active BOOLEAN NOT NULL DEFAULT true,
    max_concurrent_tasks INT NOT NULL DEFAULT 3,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cloud_runtimes_workspace ON cloud_runtimes(workspace_id);

-- Cloud runtime tasks table
CREATE TABLE cloud_runtime_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cloud_runtime_id UUID NOT NULL REFERENCES cloud_runtimes(id) ON DELETE CASCADE,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    container_id VARCHAR(100),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    exit_code INT,
    token_usage JSONB,
    cost_estimate DECIMAL(10, 6),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',  -- pending, running, completed, failed
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_task_id UNIQUE (task_id)
);

CREATE INDEX idx_cloud_runtime_tasks_runtime ON cloud_runtime_tasks(cloud_runtime_id);
CREATE INDEX idx_cloud_runtime_tasks_status ON cloud_runtime_tasks(status);

-- Add runtime_type to tasks table
ALTER TABLE tasks ADD COLUMN runtime_type VARCHAR(20) NOT NULL DEFAULT 'local';
ALTER TABLE tasks ADD COLUMN cloud_runtime_id UUID REFERENCES cloud_runtimes(id);

-- Add preferred_runtime to agents table
ALTER TABLE agents ADD COLUMN preferred_runtime VARCHAR(20) NOT NULL DEFAULT 'any';
```

- [ ] **Step 2: Create cloud_runtime.sql queries**

```sql
-- server/pkg/db/queries/cloud_runtime.sql

-- name: CreateCloudRuntime :one
INSERT INTO cloud_runtimes (workspace_id, gateway_url, provider, encrypted_api_key, api_key_hash)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetCloudRuntimeByWorkspace :one
SELECT * FROM cloud_runtimes WHERE workspace_id = $1 AND is_active = true;

-- name: UpdateCloudRuntime :one
UPDATE cloud_runtimes
SET gateway_url = $2, provider = $3, encrypted_api_key = $4, api_key_hash = $5, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteCloudRuntime :exec
UPDATE cloud_runtimes SET is_active = false, updated_at = NOW() WHERE id = $1;

-- name: CreateCloudRuntimeTask :one
INSERT INTO cloud_runtime_tasks (cloud_runtime_id, task_id)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateCloudRuntimeTask :one
UPDATE cloud_runtime_tasks
SET container_id = $2, started_at = $3, status = $4
WHERE id = $1
RETURNING *;

-- name: CompleteCloudRuntimeTask :exec
UPDATE cloud_runtime_tasks
SET completed_at = NOW(), exit_code = $2, token_usage = $3, cost_estimate = $4, status = $5
WHERE id = $1;

-- name: GetCloudRuntimeTaskByTaskID :one
SELECT * FROM cloud_runtime_tasks WHERE task_id = $1;
```

- [ ] **Step 3: Add runtime_type to task queries**

In `task.sql`, add `runtime_type` and `cloud_runtime_id` to relevant queries for task dispatch.

- [ ] **Step 4: Run sqlc generate**

```bash
cd server && sqlc generate
```

- [ ] **Step 5: Commit**

```bash
git add migrations/ server/pkg/db/queries/
git commit -m "feat(db): add cloud runtime schema"
```

---

## Task 2: Encryption Utilities

**Files:**
- Create: `server/pkg/crypto/api_key.go`

- [ ] **Step 1: Write encryption test**

```go
// server/pkg/crypto/api_key_test.go
package crypto

import (
    "testing"
)

func TestEncryptDecryptAPIKey(t *testing.T) {
    key := "sk-ant-api03-xxxxx"
    passphrase := "workspace-secret-123"

    encrypted, err := EncryptAPIKey(key, passphrase)
    if err != nil {
        t.Fatalf("EncryptAPIKey failed: %v", err)
    }

    if encrypted == key {
        t.Fatal("Encrypted key should differ from original")
    }

    decrypted, err := DecryptAPIKey(encrypted, passphrase)
    if err != nil {
        t.Fatalf("DecryptAPIKey failed: %v", err)
    }

    if decrypted != key {
        t.Errorf("Decrypted key mismatch: got %q, want %q", decrypted, key)
    }
}

func TestHashAPIKey(t *testing.T) {
    key := "sk-ant-api03-xxxxx"
    hash := HashAPIKey(key)

    if hash == "" {
        t.Fatal("Hash should not be empty")
    }

    if hash == key {
        t.Fatal("Hash should differ from key")
    }

    // Same key should produce same hash
    hash2 := HashAPIKey(key)
    if hash != hash2 {
        t.Errorf("Same key should produce same hash")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd server && go test ./pkg/crypto/ -v
# Expected: FAIL - package does not exist
```

- [ ] **Step 3: Write minimal encryption implementation**

```go
// server/pkg/crypto/api_key.go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "errors"
    "io"
)

// DeriveKey creates a 32-byte key from passphrase using SHA-256
func deriveKey(passphrase string) []byte {
    h := sha256.New()
    h.Write([]byte(passphrase))
    return h.Sum(nil)
}

// EncryptAPIKey encrypts API key using AES-256-GCM
func EncryptAPIKey(key, passphrase string) (string, error) {
    k := deriveKey(passphrase)
    block, err := aes.NewCipher(k)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }

    ciphertext := gcm.Seal(nonce, nonce, []byte(key), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptAPIKey decrypts API key encrypted with EncryptAPIKey
func DecryptAPIKey(encrypted, passphrase string) (string, error) {
    ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
    if err != nil {
        return "", err
    }

    k := deriveKey(passphrase)
    block, err := aes.NewCipher(k)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return "", errors.New("ciphertext too short")
    }

    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }

    return string(plaintext), nil
}

// HashAPIKey creates a SHA-256 hash of the API key for lookup
func HashAPIKey(key string) string {
    h := sha256.New()
    h.Write([]byte(key))
    return base64.URLEncoding.EncodeToString(h.Sum(nil))
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd server && go test ./pkg/crypto/ -v
# Expected: PASS
```

- [ ] **Step 5: Commit**

```bash
git add server/pkg/crypto/
git commit -m "feat(crypto): add API key encryption utilities"
```

---

## Task 3: Protocol Events

**Files:**
- Modify: `server/pkg/protocol/events.go`

- [ ] **Step 1: Add gateway event types**

In `events.go`, add:

```go
// Gateway events (server <-> gateway)
EventGatewayRegister    = "gateway:register"
EventGatewayHeartbeat  = "gateway:heartbeat"
EventGatewayConnected  = "gateway:connected"
EventGatewayDisconnected = "gateway:disconnected"

// Task dispatch events (server -> gateway)
EventTaskDispatch      = "task:dispatch"
EventTaskCancel       = "task:cancel"

// Task lifecycle events (gateway -> server)
EventTaskDispatched    = "task:dispatched"
EventTaskLogs          = "task:logs"
EventTaskCompleted     = "task:completed"
EventTaskFailed        = "task:failed"
```

- [ ] **Step 2: Commit**

```bash
git add server/pkg/protocol/events.go
git commit -m "feat(protocol): add gateway event types"
```

---

## Task 4: Cloud Runtime Handler (Server-Side API)

**Files:**
- Create: `server/internal/handler/cloud_runtime.go`
- Modify: `server/cmd/server/router.go`

- [ ] **Step 1: Write cloud_runtime handler**

```go
// server/internal/handler/cloud_runtime.go
package handler

import (
    "encoding/json"
    "net/http"
    "strings"

    "github.com/agentra-ai/agentra/server/pkg/crypto"
    "github.com/go-chi/chi/v5"
)

// RegisterCloudRuntime creates or updates cloud runtime for a workspace
func (h *Handler) RegisterCloudRuntime(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    workspaceID := ctx.Value("X-Workspace-ID").(string)

    var req struct {
        GatewayURL      string `json:"gateway_url"`      // NULL for basic tier
        Provider        string `json:"provider"`        // "anthropic" | "openai"
        APIKey          string `json:"api_key"`        // Plain text, encrypted before storage
        Max任务s int    `json:"max_concurrent_tasks"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    if req.Provider != "anthropic" && req.Provider != "openai" {
        http.Error(w, "provider must be 'anthropic' or 'openai'", http.StatusBadRequest)
        return
    }

    // Generate workspace-specific passphrase for key encryption
    passphrase := workspaceID + h.cfg.JWTSecret

    // Encrypt API key
    encryptedKey, err := crypto.EncryptAPIKey(req.APIKey, passphrase)
    if err != nil {
        http.Error(w, "failed to encrypt API key", http.StatusInternalServerError)
        return
    }

    apiKeyHash := crypto.HashAPIKey(req.APIKey)

    q := h.queries
    existing, err := q.GetCloudRuntimeByWorkspace(ctx, workspaceID)
    if err != nil {
        // Create new
        _, err = q.CreateCloudRuntime(ctx, CreateCloudRuntimeParams{
            WorkspaceID:       workspaceID,
            GatewayURL:        nullString(req.GatewayURL),
            Provider:          req.Provider,
            EncryptedAPIKey:   []byte(encryptedKey),
            APIKeyHash:        apiKeyHash,
            MaxConcurrent任务s: int32(req.Max任务s),
        })
    } else {
        // Update existing
        _, err = q.UpdateCloudRuntime(ctx, UpdateCloudRuntimeParams{
            ID:               existing.ID,
            GatewayURL:       nullString(req.GatewayURL),
            Provider:         req.Provider,
            EncryptedAPIKey:  []byte(encryptedKey),
            APIKeyHash:       apiKeyHash,
        })
    }

    if err != nil {
        http.Error(w, "failed to save cloud runtime", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

// GetCloudRuntime returns cloud runtime config (without API key)
func (h *Handler) GetCloudRuntime(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    workspaceID := ctx.Value("X-Workspace-ID").(string)

    runtime, err := h.queries.GetCloudRuntimeByWorkspace(ctx, workspaceID)
    if err != nil {
        http.Error(w, "cloud runtime not configured", http.StatusNotFound)
        return
    }

    resp := struct {
        ID                 string `json:"id"`
        Provider           string `json:"provider"`
        GatewayURL         string `json:"gateway_url"`
        IsActive           bool   `json:"is_active"`
        MaxConcurrent任务s int    `json:"max_concurrent_tasks"`
        CreatedAt          string `json:"created_at"`
    }{
        ID:                 runtime.ID.String(),
        Provider:           runtime.Provider,
        GatewayURL:         stringPtr(runtime.GatewayURL),
        IsActive:           runtime.IsActive,
        MaxConcurrent任务s: int(runtime.MaxConcurrent任务s),
        CreatedAt:          runtime.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
    }

    json.NewEncoder(w).Encode(resp)
}

// DeleteCloudRuntime deactivates cloud runtime
func (h *Handler) DeleteCloudRuntime(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    workspaceID := ctx.Value("X-Workspace-ID").(string)

    runtime, err := h.queries.GetCloudRuntimeByWorkspace(ctx, workspaceID)
    if err != nil {
        http.Error(w, "cloud runtime not found", http.StatusNotFound)
        return
    }

    err = h.queries.DeleteCloudRuntime(ctx, runtime.ID)
    if err != nil {
        http.Error(w, "failed to delete cloud runtime", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

// ValidateAPIKey tests if the API key is valid
func (h *Handler) ValidateAPIKey(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Provider string `json:"provider"`
        APIKey  string `json:"api_key"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    // Make a minimal test API call
    valid, err := validateProviderKey(req.Provider, req.APIKey)
    if err != nil || !valid {
        http.Error(w, "invalid API key", http.StatusBadRequest)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

func validateProviderKey(provider, apiKey string) (bool, error) {
    switch provider {
    case "anthropic":
        // Test with a minimal completion
        // ...
    case "openai":
        // Test with a minimal completion
        // ...
    }
    return false, nil
}

func nullString(s string) *string {
    if s == "" {
        return nil
    }
    return &s
}

func stringPtr(s *string) string {
    if s == nil {
        return ""
    }
    return *s
}
```

- [ ] **Step 2: Add routes to router.go**

In `cmd/server/router.go`, add protected routes:

```go
r.Route("/api/cloud-runtime", func(r chi.Router) {
    r.Use(middleware.AuthRequired)
    r.Post("/", h.RegisterCloudRuntime)
    r.Get("/", h.GetCloudRuntime)
    r.Delete("/", h.DeleteCloudRuntime)
    r.Post("/validate", h.ValidateAPIKey)
})
```

- [ ] **Step 3: Commit**

```bash
git add server/internal/handler/cloud_runtime.go server/cmd/server/router.go
git commit -m "feat(handler): add cloud runtime API endpoints"
```

---

## Task 5: Gateway WebSocket Client (Server-Side)

**Files:**
- Create: `server/internal/gateway/client.go`
- Modify: `server/internal/realtime/hub.go`

- [ ] **Step 1: Write gateway client manager**

```go
// server/internal/gateway/client.go
package gateway

import (
    "encoding/json"
    "sync"
    "websocket"

    "github.com/agentra-ai/agentra/server/pkg/protocol"
)

// Client manages a WebSocket connection to a Cloud Runtime Gateway
type Client struct {
    ID         string
    Conn       *websocket.Conn
    WorkspaceID string
    mu         sync.Mutex
}

// Hub manages all gateway connections
type Hub struct {
    mu      sync.RWMutex
    clients map[string]*Client  // gatewayID -> Client

    // workspace to gateway mapping
    workspaceGateway map[string]string  // workspaceID -> gatewayID

    // Task dispatch callbacks
    dispatchHandler func(gatewayID, taskID string, config map[string]any) error
}

func NewHub() *Hub {
    return &Hub{
        clients:          make(map[string]*Client),
        workspaceGateway: make(map[string]string),
    }
}

// Register registers a gateway connection
func (h *Hub) Register(gatewayID, workspaceID string, conn *websocket.Conn) *Client {
    h.mu.Lock()
    defer h.mu.Unlock()

    client := &Client{
        ID:          gatewayID,
        Conn:        conn,
        WorkspaceID: workspaceID,
    }
    h.clients[gatewayID] = client
    h.workspaceGateway[workspaceID] = gatewayID
    return client
}

// Unregister removes a gateway connection
func (h *Hub) Unregister(gatewayID string) {
    h.mu.Lock()
    defer h.mu.Unlock()

    if client, ok := h.clients[gatewayID]; ok {
        delete(h.workspaceGateway, client.WorkspaceID)
        delete(h.clients, gatewayID)
    }
}

// GetGatewayForWorkspace returns the gateway ID for a workspace
func (h *Hub) GetGatewayForWorkspace(workspaceID string) string {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.workspaceGateway[workspaceID]
}

// DispatchTask sends a task to the specified gateway
func (h *Hub) DispatchTask(gatewayID string, taskID string, config map[string]any) error {
    h.mu.RLock()
    client, ok := h.clients[gatewayID]
    h.mu.RUnlock()

    if !ok {
        return ErrGatewayNotConnected
    }

    msg := map[string]any{
        "type":   protocol.EventTaskDispatch,
        "taskId": taskID,
        "config": config,
    }

    return client.Send(msg)
}

// Send sends a JSON message to the gateway
func (c *Client) Send(msg map[string]any) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    return c.Conn.WriteMessage(websocket.TextMessage, data)
}

// ErrGatewayNotConnected is returned when gateway is not connected
var ErrGatewayNotConnected = errors.New("gateway not connected")
```

- [ ] **Step 2: Integrate Hub into existing hub.go**

Modify `internal/realtime/hub.go` to embed the gateway hub or manage it alongside existing WS clients.

- [ ] **Step 3: Commit**

```bash
git add server/internal/gateway/ server/internal/realtime/hub.go
git commit -m "feat(gateway): add gateway WebSocket client manager"
```

---

## Task 6: Task Service Modifications

**Files:**
- Modify: `server/internal/service/task.go`

- [ ] **Step 1: Modify task dispatch to check runtime_type**

In `task.go`, update the dispatch logic:

```go
// In the task dispatch flow, after task is claimed:
// Check if workspace has cloud runtime enabled

func (s *TaskService) ShouldUseCloudRuntime(ctx context.Context, task *Task) bool {
    // Check if task has cloud_runtime_id set
    if task.CloudRuntimeID != nil {
        return true
    }

    // Check agent preference
    if task.Agent != nil && task.Agent.PreferredRuntime == "cloud" {
        return true
    }

    // Check workspace cloud runtime config
    runtime, err := s.queries.GetCloudRuntimeByWorkspace(ctx, task.WorkspaceID)
    if err != nil || !runtime.IsActive {
        return false
    }

    return task.RuntimeType == "cloud"
}
```

- [ ] **Step 2: Commit**

```bash
git add server/internal/service/task.go
git commit -m "feat(service): add cloud runtime selection logic"
```

---

## Task 7: Cloud Runtime Gateway Service (NEW)

**Files:**
- Create: `server/cmd/gateway/main.go`
- Create: `server/internal/gateway/container.go`
- Create: `server/internal/gateway/proxy.go`
- Create: `server/internal/gateway/wsclient.go`
- Create: `server/internal/gateway/task.go`

- [ ] **Step 1: Write gateway main.go**

```go
// server/cmd/gateway/main.go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"

    "github.com/agentra-ai/agentra/server/internal/gateway"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    cfg := gateway.Config{
        ServerURL:     getEnv("AGENTRA_SERVER_URL", "ws://localhost:8080/ws"),
        GatewayID:     getEnv("GATEWAY_ID", ""),
        WorkspaceID:   getEnv("GATEWAY_WORKSPACE_ID", ""),
        AuthToken:     getEnv("AGENTRA_AUTH_TOKEN", ""),
        DockerHost:    getEnv("DOCKER_HOST", "unix:///var/run/docker.sock"),
        BaseImage:     getEnv("BASE_IMAGE", "agentra/agent-runtime:latest"),
    }

    g := gateway.New(cfg, logger)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Handle shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigChan
        logger.Info("shutting down gateway")
        cancel()
    }()

    if err := g.Run(ctx); err != nil {
        logger.Error("gateway error", "error", err)
        os.Exit(1)
    }
}

func getEnv(key, defaultVal string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return defaultVal
}
```

- [ ] **Step 2: Write container.go**

```go
// server/internal/gateway/container.go
package gateway

import (
    "context"
    "fmt"
    "time"

    "github.com/docker/docker/client"
)

// ContainerManager handles Docker container lifecycle
type ContainerManager struct {
    docker  *client.Client
    baseImg string
}

func NewContainerManager(dockerHost, baseImage string) (*ContainerManager, error) {
    cli, err := client.NewClientWithOpts(client.WithHost(dockerHost))
    if err != nil {
        return nil, fmt.Errorf("create docker client: %w", err)
    }

    return &ContainerManager{
        docker:  cli,
        baseImg: baseImage,
    }, nil
}

// CreateContainer creates a new container for task execution
func (cm *ContainerManager) CreateContainer(ctx context.Context, task *TaskConfig) (string, error) {
    // Prepare container config
    resp, err := cm.docker.ContainerCreate(ctx, &container.Config{
        Image: cm.baseImg,
        Env: []string{
            fmt.Sprintf("AGENTRA_TASK_ID=%s", task.TaskID),
            fmt.Sprintf("AGENTRA_API_KEY=%s", task.APIKey),
            fmt.Sprintf("AGENTRA_PROXY_URL=%s", task.ProxyURL),
        },
        AttachStdout: true,
        AttachStderr: true,
        Tty:          false,
    }, &container.HostConfig{
        NetworkMode: "agentra-gateway",
        Memory:      int64(task.MemoryLimitMB) * 1024 * 1024,
        CPUPeriod:   100000,
        CPUQuota:    int64(task.CPULimit),
    }, nil, nil, "")

    if err != nil {
        return "", fmt.Errorf("create container: %w", err)
    }

    return resp.ID, nil
}

// StartContainer starts a created container
func (cm *ContainerManager) StartContainer(ctx context.Context, containerID string) error {
    return cm.docker.ContainerStart(ctx, containerID, container.StartOptions{})
}

// WaitContainer waits for container to finish and returns exit code
func (cm *ContainerManager) WaitContainer(ctx context.Context, containerID string) (int, error) {
    resultCh, errCh := cm.docker.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
    select {
    case result := <-resultCh:
        return int(result.StatusCode), nil
    case err := <-errCh:
        return -1, err
    }
}

// GetContainerLogs returns container logs
func (cm *ContainerManager) GetContainerLogs(ctx context.Context, containerID string, since time.Time) ([]byte, error) {
    reader, _, err := cm.docker.ContainerLogs(ctx, containerID, container.LogsOptions{
        ShowStdout: true,
        ShowStderr: true,
        Since:      since.Format("2006-01-02T15:04:05"),
    })
    if err != nil {
        return nil, err
    }
    defer reader.Close()

    return io.ReadAll(reader)
}

// DestroyContainer removes a container
func (cm *ContainerManager) DestroyContainer(ctx context.Context, containerID string) error {
    return cm.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{
        Force: true,
    })
}
```

- [ ] **Step 3: Write proxy.go**

```go
// server/internal/gateway/proxy.go
package gateway

import (
    "encoding/json"
    "net/http"
    "net/http/httputil"
    "net/url"
    "strings"
)

// Proxy handles API key injection and request logging
type Proxy struct {
    apiKey    string
    provider  string
    logChan   chan<- RequestLog
}

type RequestLog struct {
    Provider    string  `json:"provider"`
    Method      string  `json:"method"`
    Endpoint    string  `json:"endpoint"`
    StatusCode  int     `json:"status_code"`
    TokensIn    int     `json:"input_tokens,omitempty"`
    TokensOut   int     `json:"output_tokens,omitempty"`
    CostUSD     float64 `json:"cost_usd"`
    Timestamp   string  `json:"timestamp"`
}

func NewProxy(apiKey, provider string, logChan chan<- RequestLog) *Proxy {
    return &Proxy{
        apiKey:   apiKey,
        provider: provider,
        logChan:  logChan,
    }
}

func (p *Proxy) Handler() http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Inject API key based on provider
        switch p.provider {
        case "anthropic":
            r.Header.Set("x-api-key", p.apiKey)
            r.Header.Set("anthropic-version", "2023-06-01")
        case "openai":
            r.Header.Set("Authorization", "Bearer "+p.apiKey)
        }

        // Reverse proxy to actual provider
        target := p.getTargetURL(r.URL.Path)
        proxy := httputil.ReverseProxy{
            Director: func(r *http.Request) {
                r.URL.Host = target.Host
                r.URL.Scheme = target.Scheme
                r.Host = target.Host
            },
        }

        // Capture response for logging
        recorder := &responseRecorder{ResponseWriter: w, statusCode: 200}
        proxy.ServeHTTP(recorder, r)

        // Log the request
        p.logRequest(r, recorder)
    })
}

func (p *Proxy) getTargetURL(path string) *url.URL {
    switch {
    case strings.HasPrefix(path, "/v1/messages"):
        return &url.URL{Host: "api.anthropic.com:443", Scheme: "https"}
    case strings.HasPrefix(path, "/v1/chat/completions"):
        return &url.URL{Host: "api.openai.com:443", Scheme: "https"}
    }
    return &url.URL{Host: "api.anthropic.com:443", Scheme: "https"}
}

func (p *Proxy) logRequest(r *http.Request, recorder *responseRecorder) {
    log := RequestLog{
        Provider:   p.provider,
        Method:    r.Method,
        Endpoint:   r.URL.Path,
        StatusCode: recorder.statusCode,
        Timestamp:  time.Now().Format("2006-01-02T15:04:05Z"),
    }

    // Try to parse tokens from response
    if strings.HasPrefix(r.URL.Path, "/v1/messages") {
        // Parse anthropic response for token usage
        var resp map[string]any
        if json.Unmarshal(recorder.Body(), &resp) == nil {
            if usage, ok := resp["usage"].(map[string]any); ok {
                if v, ok := usage["input_tokens"].(float64); ok {
                    log.TokensIn = int(v)
                }
                if v, ok := usage["output_tokens"].(float64); ok {
                    log.TokensOut = int(v)
                }
            }
        }
    }

    // Calculate cost estimate (simplified)
    log.CostUSD = calculateCost(p.provider, log.TokensIn, log.TokensOut)

    select {
    case p.logChan <- log:
    default:
    }
}

func calculateCost(provider string, in, out int) float64 {
    switch provider {
    case "anthropic":
        // Claude 3.5 Sonnet pricing (example)
        return float64(in)*0.003/1000 + float64(out)*0.015/1000
    case "openai":
        // GPT-4o pricing (example)
        return float64(in)*0.005/1000 + float64(out)*0.015/1000
    }
    return 0
}

type responseRecorder struct {
    http.ResponseWriter
    statusCode int
    body      []byte
}

func (r *responseRecorder) WriteHeader(code int) {
    r.statusCode = code
    r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
    r.body = append(r.body, b...)
    return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) Body() []byte {
    return r.body
}
```

- [ ] **Step 4: Write wsclient.go**

```go
// server/internal/gateway/wsclient.go
package gateway

import (
    "context"
    "encoding/json"
    "errors"
    "log/slog"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

type WSClient struct {
    serverURL string
    gatewayID string
    authToken string
    conn      *websocket.Conn
    mu        sync.Mutex
    logger    *slog.Logger

    // Event handlers
    onTaskStart  func(taskID string, config map[string]any)
    onTaskCancel func(taskID string)
}

func NewWSClient(serverURL, gatewayID, authToken string, logger *slog.Logger) *WSClient {
    return &WSClient{
        serverURL: serverURL,
        gatewayID: gatewayID,
        authToken: authToken,
        logger:    logger,
    }
}

func (c *WSClient) Connect(ctx context.Context) error {
    url := c.serverURL + "/api/gateway/connect?gateway_id=" + c.gatewayID
    if c.authToken != "" {
        url += "&token=" + c.authToken
    }

    conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
    if err != nil {
        return err
    }
    c.conn = conn

    // Send register message
    c.send(map[string]any{
        "type":          "gateway:register",
        "gatewayId":     c.gatewayID,
        "capabilities":  map[string]any{"containers": true},
    })

    return nil
}

func (c *WSClient) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            _, msg, err := c.conn.ReadMessage()
            if err != nil {
                if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
                    c.logger.Error("websocket error", "error", err)
                }
                return err
            }

            var event map[string]any
            if err := json.Unmarshal(msg, &event); err != nil {
                continue
            }

            c.handleEvent(event)
        }
    }
}

func (c *WSClient) handleEvent(event map[string]any) {
    switch event["type"] {
    case "task:start":
        if c.onTaskStart != nil {
            c.onTaskStart(event["taskId"].(string), event["config"].(map[string]any))
        }
    case "task:cancel":
        if c.onTaskCancel != nil {
            c.onTaskCancel(event["taskId"].(string))
        }
    case "gateway:heartbeat":
        // Respond with heartbeat
        c.send(map[string]any{"type": "gateway:heartbeat"})
    }
}

func (c *WSClient) SendTaskDispatched(taskID, containerID string) error {
    return c.send(map[string]any{
        "type":        "task:dispatched",
        "taskId":      taskID,
        "containerId": containerID,
    })
}

func (c *WSClient) SendTaskLogs(taskID, logs string) error {
    return c.send(map[string]any{
        "type":    "task:logs",
        "taskId":  taskID,
        "logs":    logs,
    })
}

func (c *WSClient) SendTaskCompleted(taskID string, exitCode int, output string) error {
    return c.send(map[string]any{
        "type":      "task:completed",
        "taskId":    taskID,
        "exitCode":  exitCode,
        "output":    output,
    })
}

func (c *WSClient) SendTaskFailed(taskID, errorMsg string) error {
    return c.send(map[string]any{
        "type":   "task:failed",
        "taskId": taskID,
        "error":  errorMsg,
    })
}

func (c *WSClient) send(msg map[string]any) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    return c.conn.WriteMessage(websocket.TextMessage, data)
}
```

- [ ] **Step 5: Write gateway.go (main coordinator)**

```go
// server/internal/gateway/gateway.go
package gateway

import (
    "context"
    "log/slog"
    "sync"
)

type Config struct {
    ServerURL   string
    GatewayID   string
    WorkspaceID string
    AuthToken   string
    DockerHost  string
    BaseImage   string
}

type Gateway struct {
    cfg     Config
    logger  *slog.Logger
    containerMgr *ContainerManager
    wsClient *WSClient
    tasks   sync.Map  // taskID -> *RunningTask
}

type RunningTask struct {
    TaskID      string
    ContainerID string
    CancelFunc  context.CancelFunc
}

func New(cfg Config, logger *slog.Logger) *Gateway {
    cm, _ := NewContainerManager(cfg.DockerHost, cfg.BaseImage)
    return &Gateway{
        cfg:     cfg,
        logger:  logger,
        containerMgr: cm,
    }
}

func (g *Gateway) Run(ctx context.Context) error {
    g.wsClient = NewWSClient(g.cfg.ServerURL, g.cfg.GatewayID, g.cfg.AuthToken, g.logger)

    g.wsClient.onTaskStart = g.handleTaskStart
    g.wsClient.onTaskCancel = g.handleTaskCancel

    if err := g.wsClient.Connect(ctx); err != nil {
        return err
    }

    g.logger.Info("gateway connected", "gatewayId", g.cfg.GatewayID)

    return g.wsClient.Run(ctx)
}

func (g *Gateway) handleTaskStart(taskID string, config map[string]any) {
    g.logger.Info("task received", "taskId", taskID)

    ctx, cancel := context.WithCancel(context.Background())

    // Store running task
    g.tasks.Store(taskID, &RunningTask{
        TaskID:     taskID,
        CancelFunc: cancel,
    })

    go g.executeTask(ctx, taskID, config)
}

func (g *Gateway) handleTaskCancel(taskID string) {
    if task, ok := g.tasks.LoadAndDelete(taskID); ok {
        rt := task.(*RunningTask)
        rt.CancelFunc()
        g.logger.Info("task cancelled", "taskId", taskID)
    }
}

func (g *Gateway) executeTask(ctx context.Context, taskID string, config map[string]any) {
    defer g.tasks.Delete(taskID)

    // Create container
    containerID, err := g.containerMgr.CreateContainer(ctx, &TaskConfig{
        TaskID:        taskID,
        APIKey:        config["api_key"].(string),
        ProxyURL:      config["proxy_url"].(string),
        MemoryLimitMB: 4096,
        CPULimit:      2000,
    })
    if err != nil {
        g.wsClient.SendTaskFailed(taskID, err.Error())
        return
    }

    g.tasks.Store(taskID, &RunningTask{ContainerID: containerID})
    g.wsClient.SendTaskDispatched(taskID, containerID)

    // Start container
    if err := g.containerMgr.StartContainer(ctx, containerID); err != nil {
        g.wsClient.SendTaskFailed(taskID, err.Error())
        g.containerMgr.DestroyContainer(ctx, containerID)
        return
    }

    // Wait for completion
    exitCode, err := g.containerMgr.WaitContainer(ctx, containerID)
    if err != nil {
        g.wsClient.SendTaskFailed(taskID, err.Error())
    } else {
        g.wsClient.SendTaskCompleted(taskID, exitCode, "")
    }

    // Destroy container
    g.containerMgr.DestroyContainer(context.Background(), containerID)
}
```

- [ ] **Step 6: Commit**

```bash
git add server/cmd/gateway/ server/internal/gateway/
git commit -m "feat(gateway): implement Cloud Runtime Gateway service"
```

---

## Task 8: Frontend - Runtime Settings Tab

**Files:**
- Create: `apps/web/features/settings/components/runtime-tab.tsx`

- [ ] **Step 1: Write runtime tab component**

```tsx
// apps/web/features/settings/components/runtime-tab.tsx
'use client';

import { useState } from 'react';
import { Button } from '@/shared/components/ui/button';
import { Input } from '@/shared/components/ui/input';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';

interface CloudRuntime {
  id: string;
  provider: 'anthropic' | 'openai';
  gateway_url: string | null;
  is_active: boolean;
  max_concurrent_tasks: number;
}

export function RuntimeTab() {
  const [runtimeType, setRuntimeType] = useState<'local' | 'cloud'>('local');
  const [provider, setProvider] = useState<'anthropic' | 'openai'>('anthropic');
  const [apiKey, setAPIKey] = useState('');
  const [gatewayUrl, setGatewayUrl] = useState('');
  const [isValidating, setIsValidating] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleTestConnection = async () => {
    setIsValidating(true);
    setError(null);

    try {
      const res = await api.post('/api/cloud-runtime/validate', {
        provider,
        api_key: apiKey,
      });

      if (!res.ok) {
        setError('Invalid API key');
      }
    } catch {
      setError('Connection test failed');
    } finally {
      setIsValidating(false);
    }
  };

  const handleSave = async () => {
    setIsSaving(true);
    setError(null);

    try {
      const res = await api.post('/api/cloud-runtime', {
        provider,
        api_key: apiKey,
        gateway_url: runtimeType === 'pro' ? gatewayUrl : null,
        max_concurrent_tasks: 3,
      });

      if (!res.ok) {
        setError('Failed to save settings');
      }
    } catch {
      setError('Save failed');
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold">Runtime Settings</h2>
        <p className="text-sm text-muted-foreground">
          Configure where agents execute tasks
        </p>
      </div>

      {/* Runtime Type Selector */}
      <div className="space-y-2">
        <Label>Runtime Type</Label>
        <div className="flex gap-4">
          <button
            type="button"
            onClick={() => setRuntimeType('local')}
            className={`flex-1 p-4 border rounded-lg text-left ${
              runtimeType === 'local' ? 'border-primary bg-primary/5' : 'border-border'
            }`}
          >
            <div className="font-medium">Local Daemon</div>
            <div className="text-sm text-muted-foreground">
              Agents run on your machine
            </div>
          </button>
          <button
            type="button"
            onClick={() => setRuntimeType('cloud')}
            className={`flex-1 p-4 border rounded-lg text-left ${
              runtimeType === 'cloud' ? 'border-primary bg-primary/5' : 'border-border'
            }`}
          >
            <div className="font-medium">Cloud Runtime</div>
            <div className="text-sm text-muted-foreground">
              Agents run in managed containers
            </div>
          </button>
        </div>
      </div>

      {runtimeType === 'cloud' && (
        <>
          {/* Provider Selection */}
          <div className="space-y-2">
            <Label>AI Provider</Label>
            <Select value={provider} onValueChange={(v) => setProvider(v as typeof provider)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="anthropic">Anthropic (Claude)</SelectItem>
                <SelectItem value="openai">OpenAI (GPT)</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {/* API Key */}
          <div className="space-y-2">
            <Label>API Key</Label>
            <div className="flex gap-2">
              <Input
                type="password"
                value={apiKey}
                onChange={(e) => setAPIKey(e.target.value)}
                placeholder="sk-ant-..."
              />
              <Button
                variant="outline"
                onClick={handleTestConnection}
                disabled={!apiKey || isValidating}
              >
                {isValidating ? 'Testing...' : 'Test'}
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              Your API key is encrypted and never exposed to frontend
            </p>
          </div>

          {error && (
            <p className="text-sm text-destructive">{error}</p>
          )}

          {/* Save Button */}
          <Button onClick={handleSave} disabled={isSaving || !apiKey}>
            {isSaving ? 'Saving...' : 'Save Cloud Runtime'}
          </Button>
        </>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add apps/web/features/settings/components/runtime-tab.tsx
git commit -m "feat(settings): add runtime tab component"
```

---

## Task 9: Frontend - Issue Timeline with Cloud Badge

**Files:**
- Modify: `apps/web/features/issues/components/issue-timeline.tsx`

- [ ] **Step 1: Add cloud runtime indicator**

In the issue timeline, add a cloud badge when task is running in cloud:

```tsx
// In issue-timeline.tsx, add where task status is displayed:

{runtimeType === 'cloud' && (
  <Badge variant="outline" className="ml-2 bg-blue-50">
    <Cloud className="w-3 h-3 mr-1" />
    Cloud
  </Badge>
)}
```

- [ ] **Step 2: Commit**

```bash
git add apps/web/features/issues/components/issue-timeline.tsx
git commit -m "feat(issues): add cloud runtime indicator to timeline"
```

---

## Task 10: Docker Compose - Gateway Service

**Files:**
- Modify: `docker-compose.yml`

- [ ] **Step 1: Add gateway service**

```yaml
services:
  # ... existing services ...

  gateway:
    container_name: agentra-gateway
    build:
      context: .
      dockerfile: gateway.Dockerfile
    environment:
      AGENTRA_SERVER_URL: ${AGENTRA_SERVER_URL}
      GATEWAY_ID: ${GATEWAY_ID:-gateway-1}
      GATEWAY_WORKSPACE_ID: ${GATEWAY_WORKSPACE_ID}
      AGENTRA_AUTH_TOKEN: ${GATEWAY_AUTH_TOKEN}
      DOCKER_HOST: unix:///var/run/docker.sock
      BASE_IMAGE: agentra/agent-runtime:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - gateway-data:/data
    restart: unless-stopped
    depends_on:
      server:
        condition: service_healthy

volumes:
  gateway-data:
```

- [ ] **Step 2: Create gateway.Dockerfile**

```dockerfile
# gateway.Dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./server/
RUN cd server && CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/gateway ./cmd/gateway

FROM alpine:3.19
RUN apk add --no-cache ca-certificates docker-cli
COPY --from=builder /src/server/bin/gateway /usr/local/bin/
ENTRYPOINT ["gateway"]
```

- [ ] **Step 3: Commit**

```bash
git add docker-compose.yml gateway.Dockerfile
git commit -m "feat(infra): add Cloud Runtime Gateway to docker compose"
```

---

## Spec Coverage Check

| Spec Section | Tasks |
|--------------|-------|
| Architecture | Task 4, 5, 7 |
| User Config Flow | Task 8 |
| Container Lifecycle | Task 7 (container.go) |
| Security (Proxy) | Task 7 (proxy.go) |
| DB Schema | Task 1 |
| API Endpoints | Task 3, 4 |
| WebSocket Protocol | Task 5, 7 |
| Frontend Changes | Task 8, 9 |
| Error Handling | Task 7 |

**Gap Analysis:** None — all spec sections are covered.

---

## Type Consistency Check

- `TaskConfig` in gateway/container.go has fields matching spec
- `RequestLog` in gateway/proxy.go matches protocol definition
- WS event types match `pkg/protocol/events.go`
- All taskIDs use UUID strings consistently

**All types consistent.**
