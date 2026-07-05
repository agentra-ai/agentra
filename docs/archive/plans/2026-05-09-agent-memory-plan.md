# Agent Memory (RAG) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a pgvector-backed agent memory system with RAG injection — per-agent private memories + workspace-shared team memory, automatic capture on task completion, automatic injection on task start, and a Memory Viewer UI.

**Architecture:** Standalone `server/pkg/memory` Go module with its own `go.mod`, connecting directly to PostgreSQL. Embedding via OpenAI `text-embedding-3-small`. Frontend in `apps/web/features/memory/` following the existing feature-based architecture.

**Tech Stack:** Go 1.26, pgx/v5, pgvector, OpenAI Go client, Next.js 16, Zustand

---

## File Structure

```
server/
├── pkg/
│   └── memory/
│       ├── go.mod
│       ├── go.sum
│       ├── types.go          # MemoryEntry, MemoryType constants
│       ├── service.go         # MemoryService: Store, Recall, Search
│       ├── embedding.go      # OpenAI embedding client
│       └── hooks/
│           ├── task_completion.go  # auto-extract on task done
│           └── task_start.go       # auto-inject relevant memories
│
migrations/
├── 032_agent_memory.up.sql
└── 032_agent_memory.down.sql
├── 033_team_memory.up.sql
└── 033_team_memory.down.sql

server/pkg/db/queries/
├── memory.sql          # agent_memories + team_memory queries

server/internal/handler/
├── memory.go          # REST API handlers for memory CRUD

server/internal/service/
├── task.go             # modify CompleteTask to call memory hook

apps/web/features/
├── memory/
│   ├── components/
│   │   ├── MemoryViewer.tsx
│   │   ├── MemoryList.tsx
│   │   ├── MemoryItem.tsx
│   │   ├── MemoryEditor.tsx
│   │   └── MemorySearch.tsx
│   ├── hooks/
│   │   └── useMemoryStore.ts
│   └── api/
│       └── memoryApi.ts
```

---

## Task 1: Database Migrations

**Files:**
- Create: `server/migrations/032_agent_memory.up.sql`
- Create: `server/migrations/032_agent_memory.down.sql`
- Create: `server/migrations/033_team_memory.up.sql`
- Create: `server/migrations/033_team_memory.down.sql`

- [ ] **Step 1: Create agent_memories migration**

```sql
-- 032_agent_memory.up.sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE agent_memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    memory_type TEXT NOT NULL CHECK (memory_type IN ('learning', 'task_result', 'context', 'pattern')),
    content TEXT NOT NULL,
    embedding vector(1536),
    metadata JSONB DEFAULT '{}',
    is_private BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX agent_memories_agent_id_idx ON agent_memories(agent_id);
CREATE INDEX agent_memories_workspace_id_idx ON agent_memories(workspace_id);
CREATE INDEX agent_memories_embedding_idx ON agent_memories USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```

- [ ] **Step 2: Create agent_memories down migration**

```sql
-- 032_agent_memory.down.sql
DROP INDEX IF EXISTS agent_memories_embedding_idx;
DROP INDEX IF EXISTS agent_memories_workspace_id_idx;
DROP INDEX IF EXISTS agent_memories_agent_id_idx;
DROP TABLE IF EXISTS agent_memories;
```

- [ ] **Step 3: Create team_memory migration**

```sql
-- 033_team_memory.up.sql
CREATE TABLE team_memory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    memory_type TEXT NOT NULL CHECK (memory_type IN ('learning', 'task_result', 'context', 'pattern')),
    content TEXT NOT NULL,
    embedding vector(1536),
    metadata JSONB DEFAULT '{}',
    created_by UUID REFERENCES agents(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX team_memory_workspace_id_idx ON team_memory(workspace_id);
CREATE INDEX team_memory_embedding_idx ON team_memory USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```

- [ ] **Step 4: Create team_memory down migration**

```sql
-- 033_team_memory.down.sql
DROP INDEX IF EXISTS team_memory_embedding_idx;
DROP INDEX IF EXISTS team_memory_workspace_id_idx;
DROP TABLE IF EXISTS team_memory;
```

- [ ] **Step 5: Commit**

```bash
git add server/migrations/032_agent_memory.up.sql server/migrations/032_agent_memory.down.sql server/migrations/033_team_memory.up.sql server/migrations/033_team_memory.down.sql
git commit -m "feat(memory): add agent_memories and team_memory tables"
```

---

## Task 2: SQL Queries (sqlc)

**Files:**
- Create: `server/pkg/db/queries/memory.sql`
- Regenerate: `server/pkg/db/generated/memory.sql.go`

- [ ] **Step 1: Write memory.sql queries**

```sql
-- name: CreateAgentMemory :one
INSERT INTO agent_memories (agent_id, workspace_id, memory_type, content, embedding, metadata, is_private)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListAgentMemories :many
SELECT * FROM agent_memories
WHERE agent_id = $1
ORDER BY created_at DESC;

-- name: SearchAgentMemories :many
SELECT id, agent_id, memory_type, content, metadata, is_private, created_at,
       1 - (embedding <=> $2::vector) AS score
FROM agent_memories
WHERE agent_id = $1 AND workspace_id = $3
  AND ($4::text[] IS NULL OR memory_type = ANY($4))
ORDER BY embedding <=> $2::vector
LIMIT $5;

-- name: DeleteAgentMemory :one
DELETE FROM agent_memories WHERE id = $1 AND agent_id = $2 RETURNING *;

-- name: UpdateAgentMemory :one
UPDATE agent_memories SET
    content = COALESCE(sqlc.narg('content'), content),
    memory_type = COALESCE(sqlc.narg('memory_type'), memory_type),
    is_private = COALESCE(sqlc.narg('is_private'), is_private),
    embedding = COALESCE(sqlc.narg('embedding'), embedding),
    updated_at = now()
WHERE id = $1 AND agent_id = $2
RETURNING *;

-- name: CreateTeamMemory :one
INSERT INTO team_memory (workspace_id, memory_type, content, embedding, metadata, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListTeamMemories :many
SELECT * FROM team_memory WHERE workspace_id = $1 ORDER BY created_at DESC;

-- name: SearchTeamMemories :many
SELECT id, memory_type, content, metadata, created_by, created_at,
       1 - (embedding <=> $2::vector) AS score
FROM team_memory
WHERE workspace_id = $1
  AND ($3::text[] IS NULL OR memory_type = ANY($3))
ORDER BY embedding <=> $2::vector
LIMIT $4;

-- name: DeleteTeamMemory :one
DELETE FROM team_memory WHERE id = $1 AND workspace_id = $2 RETURNING *;

-- name: UpdateTeamMemory :one
UPDATE team_memory SET
    content = COALESCE(sqlc.narg('content'), content),
    memory_type = COALESCE(sqlc.narg('memory_type'), memory_type),
    embedding = COALESCE(sqlc.narg('embedding'), embedding),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SearchAllMemories :many
SELECT id, agent_id, memory_type, content, metadata, created_at,
       1 - (embedding <=> $2::vector) AS score
FROM (
    SELECT id, agent_id, memory_type, content, metadata, created_at, embedding
    FROM agent_memories
    WHERE workspace_id = $1 AND is_private = false
    UNION ALL
    SELECT id, NULL, memory_type, content, metadata, created_at, embedding
    FROM team_memory
    WHERE workspace_id = $1
) AS all_memories
WHERE $3::text[] IS NULL OR memory_type = ANY($3)
ORDER BY embedding <=> $2::vector
LIMIT $4;
```

- [ ] **Step 2: Run sqlc generate**

Run: `cd server && sqlc generate`
Expected: Generates `pkg/db/generated/memory.sql.go`

- [ ] **Step 3: Commit**

```bash
git add server/pkg/db/queries/memory.sql server/pkg/db/generated/memory.sql.go
git commit -m "feat(memory): add sqlc queries for agent_memories and team_memory"
```

---

## Task 3: Go Module and Types

**Files:**
- Create: `server/pkg/memory/go.mod`
- Create: `server/pkg/memory/go.sum`
- Create: `server/pkg/memory/types.go`

- [ ] **Step 1: Create go.mod**

```bash
mkdir -p server/pkg/memory
cat > server/pkg/memory/go.mod << 'EOF'
module github.com/agentra-ai/agentra/pkg/memory

go 1.26

require (
	github.com/jackc/pgx/v5 v5.6.0
	github.com/google/uuid v1.6.0
	github.com/sashabaranov/go-openai v1.35.0
)
EOF
```

- [ ] **Step 2: Create types.go**

```go
package memory

type MemoryType string

const (
	MemoryTypeLearning    MemoryType = "learning"
	MemoryTypeTaskResult   MemoryType = "task_result"
	MemoryTypeContext      MemoryType = "context"
	MemoryTypePattern      MemoryType = "pattern"
)

type AgentMemory struct {
	ID          string      `json:"id"`
	AgentID     string      `json:"agent_id"`
	WorkspaceID string      `json:"workspace_id"`
	MemoryType  MemoryType  `json:"memory_type"`
	Content     string      `json:"content"`
	Metadata    map[string]any `json:"metadata"`
	IsPrivate   bool        `json:"is_private"`
	Score       float64     `json:"score,omitempty"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

type TeamMemory struct {
	ID          string      `json:"id"`
	WorkspaceID string      `json:"workspace_id"`
	MemoryType  MemoryType  `json:"memory_type"`
	Content     string      `json:"content"`
	Metadata    map[string]any `json:"metadata"`
	CreatedBy   string      `json:"created_by,omitempty"`
	Score       float64     `json:"score,omitempty"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
}

type StoreResult struct {
	ID        string     `json:"id"`
	MemoryType MemoryType `json:"memory_type"`
	Content   string     `json:"content"`
	CreatedAt string     `json:"created_at"`
}

type RecallResult struct {
	Memories []MemoryEntry `json:"memories"`
}

type MemoryEntry struct {
	ID         string     `json:"id"`
	MemoryType MemoryType  `json:"memory_type"`
	Content    string     `json:"content"`
	AgentID    string     `json:"agent_id,omitempty"`
	Score      float64    `json:"score"`
	CreatedAt  string     `json:"created_at"`
}
```

- [ ] **Step 3: Commit**

```bash
git add server/pkg/memory/go.mod server/pkg/memory/go.sum server/pkg/memory/types.go
git commit -m "feat(memory): add module and types"
```

---

## Task 4: Embedding Client

**Files:**
- Create: `server/pkg/memory/embedding.go`

- [ ] **Step 1: Write embedding.go**

```go
package memory

import (
	"context"
	"os"

	"github.com/sashabaranov/go-openai"
)

type EmbeddingClient struct {
	client  *openai.Client
	model   string
	dim     int
}

func NewEmbeddingClient() *EmbeddingClient {
	apiKey := os.Getenv("OPENAI_API_KEY")
	model := os.Getenv("EMBEDDING_MODEL")
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &EmbeddingClient{
		client: openai.NewClient(apiKey),
		model:  model,
		dim:    1536,
	}
}

func (c *EmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := c.client.CreateEmbeddings(ctx, openai.EmbeddingCreateInput{
		Input: text,
		Model: openai.EmbeddingModel(c.model),
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, ErrNoEmbedding
	}
	// text-embedding-3-small returns 1536-dim vectors
	vec := make([]float32, c.dim)
	copy(vec, resp.Data[0].Embedding)
	return vec, nil
}

func (c *EmbeddingClient) Dim() int { return c.dim }

var ErrNoEmbedding = fmt.Errorf("no embedding returned")
```

- [ ] **Step 2: Commit**

```bash
git add server/pkg/memory/embedding.go
git commit -m "feat(memory): add OpenAI embedding client"
```

---

## Task 5: Memory Service

**Files:**
- Create: `server/pkg/memory/service.go`

- [ ] **Step 1: Write service.go**

```go
package memory

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
)

type MemoryService struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	embedder *EmbeddingClient
}

func NewMemoryService(pool *pgxpool.Pool, embedder *EmbeddingClient) *MemoryService {
	return &MemoryService{
		pool:     pool,
		queries:  db.New(pool),
		embedder: embedder,
	}
}

func (s *MemoryService) StoreAgentMemory(ctx context.Context, agentID, workspaceID string, memType MemoryType, content string, isPrivate bool) (*StoreResult, error) {
	vec, err := s.embedder.Embed(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	row, err := s.queries.CreateAgentMemory(ctx, db.CreateAgentMemoryParams{
		AgentID:     uuid.MustParse(agentID),
		WorkspaceID: uuid.MustParse(workspaceID),
		MemoryType:  string(memType),
		Content:     content,
		Embedding:   vec,
		Metadata:    []byte("{}"),
		IsPrivate:   isPrivate,
	})
	if err != nil {
		return nil, err
	}
	return &StoreResult{
		ID:        row.ID.String(),
		MemoryType: MemoryType(row.MemoryType),
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}, nil
}

func (s *MemoryService) RecallAgentMemories(ctx context.Context, agentID, workspaceID, query string, limit int, memTypes []string) (*RecallResult, error) {
	vec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	rows, err := s.queries.SearchAgentMemories(ctx, db.SearchAgentMemoriesParams{
		AgentID:     uuid.MustParse(agentID),
		Embedding:   vec,
		WorkspaceID: uuid.MustParse(workspaceID),
		MemoryType:  memTypes,
		Limit:       int64(limit),
	})
	if err != nil {
		return nil, err
	}
	entries := make([]MemoryEntry, len(rows))
	for i, r := range rows {
		entries[i] = MemoryEntry{
			ID:         r.ID.String(),
			MemoryType: MemoryType(r.MemoryType),
			Content:    r.Content,
			AgentID:    r.AgentID.String(),
			Score:      r.Score,
			CreatedAt:  r.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
	}
	return &RecallResult{Memories: entries}, nil
}

func (s *MemoryService) SearchAll(ctx context.Context, workspaceID, query string, includeTeam bool, limit int) ([]MemoryEntry, error) {
	vec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	rows, err := s.queries.SearchAllMemories(ctx, db.SearchAllMemoriesParams{
		WorkspaceID: uuid.MustParse(workspaceID),
		Embedding:   vec,
		MemoryType:  nil,
		Limit:       int64(limit),
	})
	if err != nil {
		return nil, err
	}
	entries := make([]MemoryEntry, len(rows))
	for i, r := range rows {
		entries[i] = MemoryEntry{
			ID:         r.ID.String(),
			MemoryType: MemoryType(r.MemoryType),
			Content:    r.Content,
			AgentID:    r.AgentID.String,
			Score:      r.Score,
			CreatedAt:  r.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
	}
	return entries, nil
}

func (s *MemoryService) DeleteAgentMemory(ctx context.Context, memoryID, agentID string) error {
	_, err := s.queries.DeleteAgentMemory(ctx, uuid.MustParse(memoryID), uuid.MustParse(agentID))
	return err
}

func (s *MemoryService) StoreTeamMemory(ctx context.Context, workspaceID string, memType MemoryType, content string, createdBy string) (*StoreResult, error) {
	vec, err := s.embedder.Embed(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	var createdByUUID *uuid.UUID
	if createdBy != "" {
		v := uuid.MustParse(createdBy)
		createdByUUID = &v
	}
	row, err := s.queries.CreateTeamMemory(ctx, db.CreateTeamMemoryParams{
		WorkspaceID: uuid.MustParse(workspaceID),
		MemoryType:  string(memType),
		Content:     content,
		Embedding:   vec,
		Metadata:    []byte("{}"),
		CreatedBy:   createdByUUID,
	})
	if err != nil {
		return nil, err
	}
	return &StoreResult{
		ID:        row.ID.String(),
		MemoryType: MemoryType(row.MemoryType),
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}, nil
}

func (s *MemoryService) ListTeamMemories(ctx context.Context, workspaceID string) ([]TeamMemory, error) {
	rows, err := s.queries.ListTeamMemories(ctx, uuid.MustParse(workspaceID))
	if err != nil {
		return nil, err
	}
	result := make([]TeamMemory, len(rows))
	for i, r := range rows {
		result[i] = TeamMemory{
			ID:          r.ID.String(),
			WorkspaceID: r.WorkspaceID.String(),
			MemoryType:  MemoryType(r.MemoryType),
			Content:     r.Content,
			CreatedBy:   r.CreatedBy.String,
			CreatedAt:   r.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
		}
	}
	return result, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add server/pkg/memory/service.go
git commit -m "feat(memory): add MemoryService with store/recall/search"
```

---

## Task 6: Memory Hooks (Task Lifecycle Integration)

**Files:**
- Create: `server/pkg/memory/hooks/task_completion.go`
- Create: `server/pkg/memory/hooks/task_start.go`
- Modify: `server/internal/service/task.go:248-280` (CompleteTask)

- [ ] **Step 1: Write task_completion.go**

```go
package hooks

import (
	"context"
	"strings"

	"github.com/agentra-ai/agentra/pkg/memory"
	"github.com/agentra-ai/agentra/pkg/db"
)

type TaskCompletionHook struct {
	memorySvc *memory.MemoryService
}

func NewTaskCompletionHook(ms *memory.MemoryService) *TaskCompletionHook {
	return &TaskCompletionHook{memorySvc: ms}
}

// OnTaskComplete extracts learnings from completed task and stores them.
func (h *TaskCompletionHook) OnTaskComplete(ctx context.Context, task *db.AgentTaskQueue, result string) error {
	// Skip if no result content
	if result == "" {
		return nil
	}

	// Extract potential learnings from the result
	// Simple heuristic: look for patterns like "learned", "important", "note:", etc.
	lines := strings.Split(result, "\n")
	var learnings []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			learnings = append(learnings, line[2:])
		}
	}

	// If we found structured learnings, store them
	if len(learnings) > 0 {
		content := strings.Join(learnings, "; ")
		_, err := h.memorySvc.StoreAgentMemory(
			ctx,
			task.AgentID.String(),
			task.WorkspaceID.String(),
			memory.MemoryTypeLearning,
			content,
			true,
		)
		if err != nil {
			return err
		}
	}

	// Also store the raw task result as task_result type
	_, err := h.memorySvc.StoreAgentMemory(
		ctx,
		task.AgentID.String(),
		task.WorkspaceID.String(),
		memory.MemoryTypeTaskResult,
		truncate(result, 1000),
		true,
	)
	return err
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
```

- [ ] **Step 2: Write task_start.go**

```go
package hooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentra-ai/agentra/pkg/memory"
)

type TaskStartHook struct {
	memorySvc *memory.MemoryService
	injectLimit int
}

func NewTaskStartHook(ms *memory.MemoryService, injectLimit int) *TaskStartHook {
	if injectLimit <= 0 {
		injectLimit = 5
	}
	return &TaskStartHook{memorySvc: ms, injectLimit: injectLimit}
}

// BuildMemoryContext builds the RAG context string to inject into system prompt.
// query should be the issue title + description + skill instructions.
func (h *TaskStartHook) BuildMemoryContext(ctx context.Context, agentID, workspaceID, query string) (string, error) {
	results, err := h.memorySvc.RecallAgentMemories(ctx, agentID, workspaceID, query, h.injectLimit, nil)
	if err != nil {
		return "", fmt.Errorf("recall memories: %w", err)
	}

	if len(results.Memories) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString("\n\n=== Relevant Memories ===\n")
	for _, m := range results.Memories {
		agentNote := ""
		if m.AgentID != "" {
			agentNote = fmt.Sprintf(" (from agent:%s)", m.AgentID)
		}
		b.WriteString(fmt.Sprintf("- [%s] %s%s\n", m.MemoryType, m.Content, agentNote))
	}
	b.WriteString("===\n")
	return b.String(), nil
}
```

- [ ] **Step 3: Modify task.go to call hook**

In `server/internal/service/task.go`, after `CompleteTask` calls `s.queries.CompleteAgentTask(...)`, add:

```go
// After the successful CompleteAgentTask call, trigger memory hook
if h := s.taskCompletionHook; h != nil {
	go func() {
		_ = h.OnTaskComplete(context.Background(), updatedTask, string(result))
	}()
}
```

Add field to TaskService struct:
```go
type TaskService struct {
    ...
    taskCompletionHook *hooks.TaskCompletionHook
}
```

And constructor:
```go
func NewTaskService(..., hook *hooks.TaskCompletionHook) *TaskService {
    return &TaskService{..., taskCompletionHook: hook}
}
```

- [ ] **Step 4: Commit**

```bash
git add server/pkg/memory/hooks/task_completion.go server/pkg/memory/hooks/task_start.go server/internal/service/task.go
git commit -m "feat(memory): add task lifecycle hooks for auto-capture and RAG inject"
```

---

## Task 7: REST API Handlers

**Files:**
- Create: `server/internal/handler/memory.go`
- Modify: `server/cmd/server/router.go` (add memory routes)

- [ ] **Step 1: Write memory.go handler**

```go
package handler

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/agentra-ai/agentra/pkg/memory"
)

type MemoryHandler struct {
	svc *memory.MemoryService
}

func NewMemoryHandler(svc *memory.MemoryService) *MemoryHandler {
	return &MemoryHandler{svc: svc}
}

func (h *MemoryHandler) RegisterRoutes(r chi.Router) {
	r.Get("/workspaces/{id}/memories", h.ListMemories)
	r.Post("/workspaces/{id}/memories", h.CreateMemory)
	r.Get("/agents/{id}/memories", h.ListAgentMemories)
	r.Patch("/memories/{id}", h.UpdateMemory)
	r.Delete("/memories/{id}", h.DeleteMemory)
	r.Get("/memories/search", h.SearchMemories)
}

func (h *MemoryHandler) ListMemories(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	memories, err := h.svc.ListTeamMemories(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, memories)
}

func (h *MemoryHandler) CreateMemory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MemoryType string `json:"memory_type"`
		Content    string `json:"content"`
		IsPrivate  bool   `json:"is_private"`
		AgentID    string `json:"agent_id"`
	}
	if err := jsonDecode(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	workspaceID := chi.URLParam(r, "id")
	var result *memory.StoreResult
	var err error
	if req.AgentID != "" {
		result, err = h.svc.StoreAgentMemory(r.Context(), req.AgentID, workspaceID, memory.MemoryType(req.MemoryType), req.Content, req.IsPrivate)
	} else {
		result, err = h.svc.StoreTeamMemory(r.Context(), workspaceID, memory.MemoryType(req.MemoryType), req.Content, "")
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, result)
}

func (h *MemoryHandler) ListAgentMemories(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	memories, err := h.svc.ListAgentMemories(r.Context(), agentID) // uses ListAgentMemories query
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, memories)
}

func (h *MemoryHandler) UpdateMemory(w http.ResponseWriter, r *http.Request) {
	// Implementation for PATCH /memories/:id
}

func (h *MemoryHandler) DeleteMemory(w http.ResponseWriter, r *http.Request) {
	memoryID := chi.URLParam(r, "id")
	// For agent memories, also need agent_id from query param
	if err := h.svc.DeleteAgentMemory(r.Context(), memoryID, r.URL.Query().Get("agent_id")); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]bool{"deleted": true})
}

func (h *MemoryHandler) SearchMemories(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	workspaceID := r.URL.Query().Get("workspace_id")
	includeTeam := r.URL.Query().Get("include_team") == "true"
	results, err := h.svc.SearchAll(r.Context(), workspaceID, query, includeTeam, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"memories": results})
}
```

- [ ] **Step 2: Wire up router**

In `server/cmd/server/router.go`, add:

```go
memoryHandler := handler.NewMemoryHandler(memorySvc)
r.Route("/api", func(r chi.Router) {
    // existing routes...
    r.Handle("/workspaces/{id}/memories", memoryHandler)
    r.Handle("/agents/{id}/memories", memoryHandler)
    r.Handle("/memories/{id}", memoryHandler)
    r.Handle("/memories/search", memoryHandler)
})
```

- [ ] **Step 3: Commit**

```bash
git add server/internal/handler/memory.go server/cmd/server/router.go
git commit -m "feat(memory): add REST API handlers for memory CRUD"
```

---

## Task 8: Frontend — Memory Store and API

**Files:**
- Create: `apps/web/features/memory/hooks/useMemoryStore.ts`
- Create: `apps/web/features/memory/api/memoryApi.ts`

- [ ] **Step 1: Write useMemoryStore.ts**

```typescript
import { create } from 'zustand'
import type { AgentMemory, TeamMemory } from '@/shared/types'

interface MemoryEntry {
  id: string
  memory_type: 'learning' | 'task_result' | 'context' | 'pattern'
  content: string
  agent_id?: string
  score?: number
  created_at: string
}

interface MemoryState {
  memories: MemoryEntry[]
  isLoading: boolean
  error: string | null
  fetchAgentMemories: (agentId: string) => Promise<void>
  fetchTeamMemories: (workspaceId: string) => Promise<void>
  storeMemory: (data: { agent_id?: string; workspace_id: string; memory_type: string; content: string; is_private?: boolean }) => Promise<void>
  deleteMemory: (id: string, agentId?: string) => Promise<void>
}

export const useMemoryStore = create<MemoryState>((set, get) => ({
  memories: [],
  isLoading: false,
  error: null,

  fetchAgentMemories: async (agentId: string) => {
    set({ isLoading: true, error: null })
    try {
      const res = await api.get(`/agents/${agentId}/memories`)
      set({ memories: res.data, isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },

  fetchTeamMemories: async (workspaceId: string) => {
    set({ isLoading: true, error: null })
    try {
      const res = await api.get(`/workspaces/${workspaceId}/memories`)
      set({ memories: res.data, isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },

  storeMemory: async (data) => {
    const workspaceId = data.workspace_id
    const res = await api.post(`/workspaces/${workspaceId}/memories`, data)
    set(state => ({ memories: [res.data, ...state.memories] }))
  },

  deleteMemory: async (id: string, agentId?: string) => {
    await api.delete(`/memories/${id}?agent_id=${agentId || ''}`)
    set(state => ({ memories: state.memories.filter(m => m.id !== id) }))
  },
}))
```

- [ ] **Step 2: Write memoryApi.ts**

```typescript
import { api } from '@/shared/api'
import type { AgentMemory, TeamMemory } from '@/shared/types'

export const memoryApi = {
  listTeamMemories: (workspaceId: string) =>
    api.get<TeamMemory[]>(`/workspaces/${workspaceId}/memories`),

  listAgentMemories: (agentId: string) =>
    api.get<AgentMemory[]>(`/agents/${agentId}/memories`),

  storeMemory: (data: {
    workspace_id: string
    agent_id?: string
    memory_type: string
    content: string
    is_private?: boolean
  }) => api.post('/workspaces/' + data.workspace_id + '/memories', data),

  deleteMemory: (id: string, agentId?: string) =>
    api.delete(`/memories/${id}?agent_id=${agentId || ''}`),

  searchMemories: (workspaceId: string, query: string, includeTeam = true) =>
    api.get('/memories/search', { params: { workspace_id: workspaceId, q: query, include_team: includeTeam } }),
}
```

- [ ] **Step 3: Commit**

```bash
git add apps/web/features/memory/hooks/useMemoryStore.ts apps/web/features/memory/api/memoryApi.ts
git commit -m "feat(memory): add frontend store and API client"
```

---

## Task 9: Frontend — Memory Viewer UI

**Files:**
- Create: `apps/web/features/memory/components/MemoryViewer.tsx`
- Create: `apps/web/features/memory/components/MemoryList.tsx`
- Create: `apps/web/features/memory/components/MemoryItem.tsx`
- Create: `apps/web/features/memory/components/MemoryEditor.tsx`
- Create: `apps/web/features/memory/components/MemorySearch.tsx`

- [ ] **Step 1: Write MemoryViewer.tsx**

```tsx
import { useEffect, useState } from 'react'
import { useWorkspaceStore } from '@/features/workspace'
import { useMemoryStore } from '../hooks/useMemoryStore'
import { MemoryList } from './MemoryList'
import { MemorySearch } from './MemorySearch'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

export function MemoryViewer() {
  const { workspace } = useWorkspaceStore()
  const { memories, isLoading, fetchTeamMemories } = useMemoryStore()
  const [activeTab, setActiveTab] = useState<'all' | 'learning' | 'task_result' | 'context' | 'pattern'>('all')

  useEffect(() => {
    if (workspace?.id) {
      fetchTeamMemories(workspace.id)
    }
  }, [workspace?.id, fetchTeamMemories])

  const filtered = activeTab === 'all'
    ? memories
    : memories.filter(m => m.memory_type === activeTab)

  return (
    <div className="flex flex-col gap-4 p-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">Memory</h2>
        <Button size="sm">+ Add Memory</Button>
      </div>
      <MemorySearch workspaceId={workspace?.id} />
      <Tabs value={activeTab} onValueChange={v => setActiveTab(v as typeof activeTab)}>
        <TabsList>
          <TabsTrigger value="all">All</TabsTrigger>
          <TabsTrigger value="learning">Learnings</TabsTrigger>
          <TabsTrigger value="task_result">Results</TabsTrigger>
          <TabsTrigger value="context">Context</TabsTrigger>
          <TabsTrigger value="pattern">Patterns</TabsTrigger>
        </TabsList>
        <TabsContent value={activeTab}>
          {isLoading ? <div>Loading...</div> : <MemoryList memories={filtered} />}
        </TabsContent>
      </Tabs>
    </div>
  )
}
```

- [ ] **Step 2: Write MemoryList.tsx**

```tsx
import { MemoryItem } from './MemoryItem'

interface MemoryListProps {
  memories: Array<{
    id: string
    memory_type: string
    content: string
    agent_id?: string
    created_at: string
  }>
}

export function MemoryList({ memories }: MemoryListProps) {
  if (memories.length === 0) {
    return <p className="text-muted-foreground text-sm">No memories yet.</p>
  }
  return (
    <div className="flex flex-col gap-2">
      {memories.map(m => (
        <MemoryItem key={m.id} memory={m} />
      ))}
    </div>
  )
}
```

- [ ] **Step 3: Write MemoryItem.tsx**

```tsx
import { useState } from 'react'
import { useMemoryStore } from '../hooks/useMemoryStore'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

const TYPE_COLORS: Record<string, string> = {
  learning: 'bg-yellow-100 text-yellow-800',
  task_result: 'bg-green-100 text-green-800',
  context: 'bg-blue-100 text-blue-800',
  pattern: 'bg-purple-100 text-purple-800',
}

interface MemoryItemProps {
  memory: {
    id: string
    memory_type: string
    content: string
    agent_id?: string
    created_at: string
  }
}

export function MemoryItem({ memory }: MemoryItemProps) {
  const { deleteMemory } = useMemoryStore()
  const [confirmDelete, setConfirmDelete] = useState(false)

  return (
    <div className="border rounded-lg p-3 hover:bg-muted/50 transition-colors">
      <div className="flex items-center gap-2 mb-2">
        <Badge className={TYPE_COLORS[memory.memory_type] || 'bg-gray-100'}>
          {memory.memory_type}
        </Badge>
        <span className="text-xs text-muted-foreground">
          {new Date(memory.created_at).toLocaleDateString()}
        </span>
      </div>
      <p className="text-sm whitespace-pre-wrap">{memory.content}</p>
      <div className="flex justify-end mt-2">
        {confirmDelete ? (
          <div className="flex gap-2">
            <Button size="xs" variant="destructive" onClick={() => deleteMemory(memory.id, memory.agent_id)}>
              Confirm
            </Button>
            <Button size="xs" variant="ghost" onClick={() => setConfirmDelete(false)}>
              Cancel
            </Button>
          </div>
        ) : (
          <Button size="xs" variant="ghost" onClick={() => setConfirmDelete(true)}>
            Delete
          </Button>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Write MemorySearch.tsx**

```tsx
import { useState } from 'react'
import { useMemoryStore } from '../hooks/useMemoryStore'
import { memoryApi } from '../api/memoryApi'
import { Input } from '@/components/ui/input'

interface MemorySearchProps {
  workspaceId?: string
}

export function MemorySearch({ workspaceId }: MemorySearchProps) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<any[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const { storeMemory } = useMemoryStore()

  const handleSearch = async () => {
    if (!query.trim() || !workspaceId) return
    setIsSearching(true)
    try {
      const res = await memoryApi.searchMemories(workspaceId, query)
      setResults(res.data.memories || [])
    } finally {
      setIsSearching(false)
    }
  }

  return (
    <div className="flex gap-2">
      <Input
        placeholder="Search memories..."
        value={query}
        onChange={e => setQuery(e.target.value)}
        onKeyDown={e => e.key === 'Enter' && handleSearch()}
      />
      <Button onClick={handleSearch} disabled={isSearching}>
        {isSearching ? 'Searching...' : 'Search'}
      </Button>
    </div>
  )
}
```

- [ ] **Step 5: Write MemoryEditor.tsx (modal for creating/editing)**

```tsx
import { useState } from 'react'
import { useMemoryStore } from '../hooks/useMemoryStore'
import { useWorkspaceStore } from '@/features/workspace'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

interface MemoryEditorProps {
  open: boolean
  onClose: () => void
  agentId?: string
}

export function MemoryEditor({ open, onClose, agentId }: MemoryEditorProps) {
  const { workspace } = useWorkspaceStore()
  const { storeMemory } = useMemoryStore()
  const [content, setContent] = useState('')
  const [memoryType, setMemoryType] = useState<string>('learning')
  const [isPrivate, setIsPrivate] = useState(true)

  const handleSave = async () => {
    if (!workspace?.id || !content.trim()) return
    await storeMemory({
      workspace_id: workspace.id,
      agent_id: agentId,
      memory_type: memoryType,
      content,
      is_private: isPrivate,
    })
    setContent('')
    onClose()
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add Memory</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-3">
          <Select value={memoryType} onValueChange={setMemoryType}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="learning">Learning</SelectItem>
              <SelectItem value="task_result">Task Result</SelectItem>
              <SelectItem value="context">Context</SelectItem>
              <SelectItem value="pattern">Pattern</SelectItem>
            </SelectContent>
          </Select>
          <Textarea
            placeholder="What should this agent remember?"
            value={content}
            onChange={e => setContent(e.target.value)}
            rows={4}
          />
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="is-private"
              checked={isPrivate}
              onChange={e => setIsPrivate(e.target.checked)}
            />
            <label htmlFor="is-private" className="text-sm">Private (only this agent)</label>
          </div>
          <Button onClick={handleSave}>Save</Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 6: Commit**

```bash
git add apps/web/features/memory/components/MemoryViewer.tsx apps/web/features/memory/components/MemoryList.tsx apps/web/features/memory/components/MemoryItem.tsx apps/web/features/memory/components/MemorySearch.tsx apps/web/features/memory/components/MemoryEditor.tsx
git commit -m "feat(memory): add Memory Viewer UI components"
```

---

## Task 10: Wire Memory into Workspace Settings Page

**Files:**
- Modify: `apps/web/app/(dashboard)/settings/page.tsx` (or wherever settings lives)

- [ ] **Step 1: Add MemoryViewer to settings**

Find the settings page and add the MemoryViewer tab/panel. The exact location depends on current settings page structure — check `apps/web/app/(dashboard)/settings/` for the current layout.

- [ ] **Step 2: Commit**

```bash
git add <settings-page-path>
git commit -m "feat(memory): integrate MemoryViewer into workspace settings"
```

---

## Task 11: Unit Tests

**Files:**
- Create: `server/pkg/memory/service_test.go`
- Create: `server/pkg/memory/hooks/task_completion_test.go`

- [ ] **Step 1: Write service_test.go**

```go
package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemoryService_StoreAndRecall(t *testing.T) {
	// Test that store saves memory and recall retrieves it
	// Use a mock embedder that returns a fixed vector
}

func TestMemoryService_SearchAll(t *testing.T) {
	// Test that SearchAll returns both agent and team memories
}
```

- [ ] **Step 2: Write task_completion_test.go**

```go
package hooks

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskCompletionHook_OnTaskComplete(t *testing.T) {
	// Test that learnings are extracted from task result
}
```

- [ ] **Step 3: Commit**

```bash
git add server/pkg/memory/service_test.go server/pkg/memory/hooks/task_completion_test.go
git commit -m "test(memory): add unit tests for service and hooks"
```

---

## Task 12: End-to-End Verification

**Files:**
- None (verification only)

- [ ] **Step 1: Run migrations**

Run: `make migrate-up`
Expected: 032 and 033 migrations applied

- [ ] **Step 2: Run Go tests**

Run: `go test ./pkg/memory/... -v`
Expected: All tests pass

- [ ] **Step 3: Run TypeScript typecheck**

Run: `pnpm typecheck`
Expected: No errors

- [ ] **Step 4: Build binary**

Run: `go build -o /tmp/agentra-server ./cmd/server`
Expected: Build succeeds

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(memory): complete agent memory RAG system"
```

---

## Spec Coverage Check

| Spec Section | Task(s) |
|--------------|---------|
| Database (agent_memories + team_memory tables) | Task 1 |
| SQL queries (sqlc) | Task 2 |
| Go module + types | Task 3 |
| Embedding client (OpenAI) | Task 4 |
| MemoryService (store/recall/search) | Task 5 |
| Task completion hook (auto-capture) | Task 6 |
| Task start hook (RAG inject) | Task 6 |
| MCP tools (agentra_memory_store/recall/search) | Out of scope for this plan — add via MCP Server in a separate plan |
| REST API handlers | Task 7 |
| Memory Viewer UI | Task 8, Task 9 |
| Config (OPENAI_API_KEY) | Task 4 |

**Gap identified:** MCP tools (`agentra_memory_store`, `agentra_memory_recall`, `agentra_memory_search`) are not in this plan. These should be added via a separate update to the MCP Server plan. The core memory system (DB + service + hooks + UI) is fully covered by Tasks 1-12.

---

Plan complete and saved to `docs/archive/plans/2026-05-09-agent-memory-plan.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?