# Agentra Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement 5 major features: Multi-Provider Support, GitHub VCS Integration, Execution Traces, Memory Auto-Learning, and Swarm/Agent Delegation

**Architecture:** Layered module system - each feature is independent module. Execution Traces and GitHub Integration can be parallelized first. Multi-Provider enables Swarm Delegation.

**Tech Stack:** Go 1.26, pgx/v5, PostgreSQL, Next.js 16, Docker SDK, GitHub API

---

## File Structure

```
server/
├── pkg/
│   ├── agent/
│   │   ├── providers/          # EXISTING - multi-provider support
│   │   │   ├── provider.go     # (exists)
│   │   │   ├── anthropic.go     # (exists)
│   │   │   ├── openai.go        # (exists)
│   │   │   ├── openrouter.go   # (exists)
│   │   │   └── ollama.go       # (exists)
│   │   └── backend.go          # NEW - facade for CLI + API providers
│   │
│   ├── traces/                  # NEW - Execution Traces module
│   │   ├── go.mod
│   │   ├── types.go
│   │   ├── recorder.go
│   │   ├── summarizer.go
│   │   └── queries.go          # sqlc queries
│   │
│   ├── github/                  # NEW - GitHub VCS Integration module
│   │   ├── go.mod
│   │   ├── app.go
│   │   ├── client.go
│   │   ├── webhooks.go
│   │   ├── pr.go
│   │   ├── sync.go
│   │   └── queries.go          # sqlc queries
│   │
│   ├── memory/                  # EXISTING - extend with auto-learning
│   │   └── hooks/
│   │       ├── task_completion.go  # (exists - extend)
│   │       ├── continuous.go       # NEW
│   │       └── extractor.go        # NEW
│   │
│   └── taskgraph/               # EXISTING - extend with delegation
│       ├── delegation.go        # NEW
│       ├── executor.go          # NEW
│       └── container.go         # NEW

migrations/
├── 035_trace_tables.up.sql      # NEW
├── 035_trace_tables.down.sql
├── 036_github_tables.up.sql     # NEW
├── 036_github_tables.down.sql
├── 037_agent_delegation.up.sql  # NEW
└── 037_agent_delegation.down.sql

server/pkg/db/queries/
├── traces.sql                   # NEW
└── github.sql                   # NEW

server/internal/handler/
├── traces.go                    # NEW - REST API
└── github.go                    # NEW - REST API

apps/web/features/
├── traces/
│   ├── hooks/useTraces.ts
│   ├── api/tracesApi.ts
│   └── components/
│       ├── TraceList.tsx
│       ├── TraceDetail.tsx
│       ├── TraceTimeline.tsx
│       └── TraceAnalytics.tsx
│
└── github/
    ├── hooks/useGitHub.ts
    ├── api/githubApi.ts
    └── components/
        ├── GitHubConnect.tsx
        ├── RepoSelector.tsx
        └── PRStatusBadge.tsx
```

---

## PARALLEL TRACKS

**Track A: Execution Traces (Task 1, 2, 3)** - Can start immediately
**Track B: Multi-Provider Enhancement (Task 4, 5)** - Can start immediately
**Track C: GitHub Integration (Task 6, 7, 8)** - Can start immediately
**Track D: Memory Auto-Learning (Task 9, 10)** - Can start immediately after Track A
**Track E: Swarm Delegation (Task 11, 12, 13)** - Depends on Track B and Task Graph completion

---

## TRACK A: Execution Traces

### Task 1: Trace Database Schema

**Files:**
- Create: `server/migrations/035_trace_tables.up.sql`
- Create: `server/migrations/035_trace_tables.down.sql`

- [ ] **Step 1: Create migration up**

```sql
-- 035_trace_tables.up.sql

CREATE TABLE task_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id),
    status TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'completed', 'failed', 'cancelled')),
    started_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    duration_ms INT,
    exit_code INT,
    total_steps INT DEFAULT 0,
    total_tokens INT DEFAULT 0,
    total_cost NUMERIC(10,6) DEFAULT 0,
    output TEXT,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE trace_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_run_id UUID NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
    step_number INT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    action TEXT NOT NULL CHECK (action IN ('tool_call', 'output', 'error', 'thinking', 'status')),
    tool TEXT,
    input_text TEXT,
    output_text TEXT,
    tokens_used INT DEFAULT 0,
    duration_ms INT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX task_runs_task_id_idx ON task_runs(task_id);
CREATE INDEX task_runs_agent_id_idx ON task_runs(agent_id);
CREATE INDEX task_runs_created_at_idx ON task_runs(created_at);
CREATE INDEX trace_steps_task_run_id_idx ON trace_steps(task_run_id);
CREATE INDEX trace_steps_timestamp_idx ON trace_steps(timestamp);
```

- [ ] **Step 2: Create migration down**

```sql
-- 035_trace_tables.down.sql
DROP INDEX IF EXISTS trace_steps_timestamp_idx;
DROP INDEX IF EXISTS trace_steps_task_run_id_idx;
DROP INDEX IF EXISTS task_runs_created_at_idx;
DROP INDEX IF EXISTS task_runs_agent_id_idx;
DROP INDEX IF EXISTS task_runs_task_id_idx;
DROP TABLE IF EXISTS trace_steps;
DROP TABLE IF EXISTS task_runs;
```

- [ ] **Step 3: Commit**

```bash
git add server/migrations/035_trace_tables.up.sql server/migrations/035_trace_tables.down.sql
git commit -m "feat(traces): add task_runs and trace_steps tables"
```

---

### Task 2: Trace Go Module and SQL Queries

**Files:**
- Create: `server/pkg/traces/go.mod`
- Create: `server/pkg/traces/types.go`
- Create: `server/pkg/db/queries/traces.sql`
- Modify: `server/pkg/db/generated/traces.sql.go` (generated)

- [ ] **Step 1: Create go.mod**

```
module github.com/agentra-ai/agentra/pkg/traces

go 1.26

require (
    github.com/jackc/pgx/v5 v5.6.0
    github.com/google/uuid v1.6.0
)
```

- [ ] **Step 2: Create types.go**

```go
package traces

type TaskRun struct {
    ID           string    `json:"id"`
    TaskID       string    `json:"task_id"`
    AgentID      string    `json:"agent_id"`
    Status       string    `json:"status"`
    StartedAt    string    `json:"started_at"`
    CompletedAt  string    `json:"completed_at,omitempty"`
    DurationMs   int       `json:"duration_ms"`
    ExitCode     int       `json:"exit_code"`
    TotalSteps   int       `json:"total_steps"`
    TotalTokens  int       `json:"total_tokens"`
    TotalCost    float64   `json:"total_cost"`
    Output       string    `json:"output"`
    Error        string    `json:"error,omitempty"`
    CreatedAt    string    `json:"created_at"`
}

type TraceStep struct {
    ID           string         `json:"id"`
    TaskRunID    string         `json:"task_run_id"`
    StepNumber   int            `json:"step_number"`
    Timestamp    string         `json:"timestamp"`
    Action       string         `json:"action"`
    Tool         string         `json:"tool,omitempty"`
    InputText    string         `json:"input_text,omitempty"`
    OutputText   string         `json:"output_text,omitempty"`
    TokensUsed   int            `json:"tokens_used"`
    DurationMs   int            `json:"duration_ms"`
    Metadata     map[string]any `json:"metadata"`
}

type TaskRunSummary struct {
    TotalSteps   int            `json:"total_steps"`
    TotalTokens  int            `json:"total_tokens"`
    TotalCost    float64        `json:"total_cost"`
    Duration     int64          `json:"duration_ms"`
    ToolUsage    map[string]int `json:"tool_usage"`
    KeyActions   []string       `json:"key_actions"`
}

type TraceRecorder struct {
    pool    *pgxpool.Pool
    taskID  uuid.UUID
    steps   []TraceStep
    runID   uuid.UUID
    mu      sync.Mutex
}

func NewTraceRecorder(pool *pgxpool.Pool, taskID, runID uuid.UUID) *TraceRecorder {
    return &TraceRecorder{pool: pool, taskID: taskID, runID: runID, steps: []TraceStep{}}
}

func (r *TraceRecorder) RecordStep(ctx context.Context, step *TraceStep) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.steps = append(r.steps, *step)
    if len(r.steps) >= 10 {
        return r.flush(ctx)
    }
    return nil
}

func (r *TraceRecorder) flush(ctx context.Context) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO trace_steps (task_run_id, step_number, timestamp, action, tool, input_text, output_text, tokens_used, duration_ms, metadata)
        SELECT $1, unnest($2::int[]), unnest($3::timestamptz[]), unnest($4::text[]), unnest($5::text[]), unnest($6::text[]), unnest($7::text[]), unnest($8::int[]), unnest($9::int[]), unnest($10::jsonb[])
    `, r.runID, stepNumbers, timestamps, actions, tools, inputs, outputs, tokens, durations, metadatas)
    r.steps = r.steps[:0]
    return err
}
```

- [ ] **Step 3: Create traces.sql**

```sql
-- name: CreateTaskRun :one
INSERT INTO task_runs (task_id, agent_id, status, started_at)
VALUES ($1, $2, 'running', NOW())
RETURNING *;

-- name: GetTaskRun :one
SELECT * FROM task_runs WHERE id = $1;

-- name: CompleteTaskRun :one
UPDATE task_runs SET
    status = COALESCE($2, status),
    completed_at = NOW(),
    duration_ms = $3,
    exit_code = $4,
    total_steps = $5,
    total_tokens = $6,
    total_cost = $7,
    output = $8,
    error = $9
WHERE id = $1
RETURNING *;

-- name: ListTaskRuns :many
SELECT * FROM task_runs WHERE agent_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: ListTaskRunsByTask :many
SELECT * FROM task_runs WHERE task_id = $1 ORDER BY created_at DESC;

-- name: RecordTraceSteps :many
INSERT INTO trace_steps (task_run_id, step_number, timestamp, action, tool, input_text, output_text, tokens_used, duration_ms, metadata)
SELECT $1, unnest($2::int[]), unnest($3::timestamptz[]), unnest($4::text[]), unnest($5::text[]), unnest($6::text[]), unnest($7::text[]), unnest($8::int[]), unnest($9::int[]), unnest($10::jsonb[])
RETURNING *;

-- name: ListTraceSteps :many
SELECT * FROM trace_steps WHERE task_run_id = $1 ORDER BY step_number;

-- name: GetTraceAnalytics :one
SELECT
    COUNT(*) as total_runs,
    AVG(duration_ms) as avg_duration,
    AVG(total_tokens) as avg_tokens,
    AVG(total_cost) as avg_cost,
    SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_count,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count
FROM task_runs
WHERE agent_id = $1 AND created_at > NOW() - $2::interval;
```

- [ ] **Step 4: Run sqlc generate**

```bash
cd server && sqlc generate
```

- [ ] **Step 5: Commit**

```bash
git add server/pkg/traces/go.mod server/pkg/traces/types.go server/pkg/db/queries/traces.sql server/pkg/db/generated/traces.sql.go
git commit -m "feat(traces): add trace module with sqlc queries"
```

---

### Task 3: Trace REST API and Frontend

**Files:**
- Create: `server/internal/handler/traces.go`
- Create: `server/pkg/traces/recorder.go`
- Create: `server/pkg/traces/summarizer.go`
- Create: `apps/web/features/traces/hooks/useTraces.ts`
- Create: `apps/web/features/traces/api/tracesApi.ts`
- Create: `apps/web/features/traces/components/TraceList.tsx`
- Create: `apps/web/features/traces/components/TraceDetail.tsx`

- [ ] **Step 1: Create REST handler**

```go
// server/internal/handler/traces.go
package handler

type TraceHandler struct {
    pool *pgxpool.Pool
    queries *db.Queries
}

func (h *TraceHandler) RegisterRoutes(r chi.Router) {
    r.Get("/tasks/{id}/trace", h.GetTaskTrace)
    r.Get("/tasks/{id}/trace/summary", h.GetTaskTraceSummary)
    r.Get("/agents/{id}/traces", h.ListAgentTraces)
    r.Get("/traces/analytics", h.GetTraceAnalytics)
}

func (h *TraceHandler) GetTaskTrace(w http.ResponseWriter, r *http.Request) {
    taskID := chi.URLParam(r, "id")
    runID := r.URL.Query().Get("run_id")

    steps, err := h.queries.ListTraceSteps(r.Context(), uuid.MustParse(taskID))
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    json.NewEncoder(w).Encode(map[string]any{
        "task_id": taskID,
        "run_id": runID,
        "steps": steps,
    })
}

func (h *TraceHandler) ListAgentTraces(w http.ResponseWriter, r *http.Request) {
    agentID := chi.URLParam(r, "id")
    limit := 50

    runs, err := h.queries.ListTaskRuns(r.Context(), uuid.MustParse(agentID), limit)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    json.NewEncoder(w).Encode(runs)
}

func (h *TraceHandler) GetTraceAnalytics(w http.ResponseWriter, r *http.Request) {
    agentID := r.URL.Query().Get("agent_id")
    period := r.URL.Query().Get("period")

    analytics, err := h.queries.GetTraceAnalytics(r.Context(), uuid.MustParse(agentID), period)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    json.NewEncoder(w).Encode(analytics)
}
```

- [ ] **Step 2: Create TraceRecorder integration**

```go
// server/pkg/traces/recorder.go
package traces

import (
    "context"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

func NewTraceRecorder(pool *pgxpool.Pool, taskID, runID uuid.UUID) *TraceRecorder {
    return &TraceRecorder{
        pool:   pool,
        taskID: taskID,
        runID:  runID,
        steps:  []TraceStep{},
    }
}

func (r *TraceRecorder) RecordStep(ctx context.Context, step *TraceStep) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    step.TaskRunID = r.runID.String()
    r.steps = append(r.steps, *step)
    if len(r.steps) >= 10 {
        return r.flush(ctx)
    }
    return nil
}
```

- [ ] **Step 3: Create summarizer**

```go
// server/pkg/traces/summarizer.go
package traces

type Summarizer struct{}

func (s *Summarizer) Summarize(steps []TraceStep) *TaskRunSummary {
    summary := &TaskRunSummary{
        ToolUsage:  make(map[string]int),
        KeyActions: []string{},
    }

    var firstTimestamp, lastTimestamp time.Time
    for i, step := range steps {
        if i == 0 {
            firstTimestamp = step.Timestamp
        }
        lastTimestamp = step.Timestamp

        summary.TotalTokens += step.TokensUsed
        summary.TotalDuration += step.DurationMs

        if step.Tool != "" {
            summary.ToolUsage[step.Tool]++
        }

        if step.Action == "tool_call" {
            summary.KeyActions = append(summary.KeyActions, step.Tool)
        }
    }

    summary.Duration = lastTimestamp.Sub(firstTimestamp).Milliseconds()
    return summary
}
```

- [ ] **Step 4: Create frontend store and API**

```typescript
// apps/web/features/traces/hooks/useTraces.ts
import { create } from 'zustand'

interface TraceStep {
  id: string
  step_number: number
  action: string
  tool?: string
  input_text?: string
  output_text?: string
  timestamp: string
}

interface TaskRun {
  id: string
  task_id: string
  agent_id: string
  status: string
  total_steps: number
  total_tokens: number
  total_cost: number
  duration_ms: number
  created_at: string
}

interface TraceState {
  runs: TaskRun[]
  currentSteps: TraceStep[]
  isLoading: boolean
  error: string | null
  fetchTraces: (agentId: string) => Promise<void>
  fetchTraceDetail: (taskId: string, runId?: string) => Promise<void>
}

export const useTraceStore = create<TraceState>((set) => ({
  runs: [],
  currentSteps: [],
  isLoading: false,
  error: null,

  fetchTraces: async (agentId: string) => {
    set({ isLoading: true, error: null })
    try {
      const res = await fetch(`/api/agents/${agentId}/traces`)
      const data = await res.json()
      set({ runs: Array.isArray(data) ? data : [], isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },

  fetchTraceDetail: async (taskId: string, runId?: string) => {
    set({ isLoading: true, error: null })
    try {
      const url = runId ? `/api/tasks/${taskId}/trace?run_id=${runId}` : `/api/tasks/${taskId}/trace`
      const res = await fetch(url)
      const data = await res.json()
      set({ currentSteps: data.steps || [], isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },
}))
```

- [ ] **Step 5: Create TraceList component**

```typescript
// apps/web/features/traces/components/TraceList.tsx
import { useTraceStore } from '../hooks/useTraces'

export function TraceList({ agentId }: { agentId: string }) {
  const { runs, isLoading, error, fetchTraces } = useTraceStore()

  useEffect(() => {
    fetchTraces(agentId)
  }, [agentId])

  if (isLoading) return <div>Loading traces...</div>
  if (error) return <div>Error: {error}</div>
  if (runs.length === 0) return <div>No traces yet.</div>

  return (
    <div className="space-y-2">
      {runs.map((run) => (
        <div key={run.id} className="border rounded p-3">
          <div className="flex justify-between">
            <span className="font-medium">{run.status}</span>
            <span className="text-muted-foreground text-sm">{run.duration_ms}ms</span>
          </div>
          <div className="text-sm text-muted-foreground">
            {run.total_steps} steps, {run.total_tokens} tokens, ${run.total_cost.toFixed(4)}
          </div>
        </div>
      ))}
    </div>
  )
}
```

- [ ] **Step 6: Commit**

```bash
git add server/internal/handler/traces.go server/pkg/traces/recorder.go server/pkg/traces/summarizer.go
git add apps/web/features/traces/hooks/useTraces.ts apps/web/features/traces/api/tracesApi.ts
git add apps/web/features/traces/components/TraceList.tsx apps/web/features/traces/components/TraceDetail.tsx
git commit -m "feat(traces): add REST API and frontend components"
```

---

## TRACK B: Multi-Provider Enhancement

### Task 4: Provider Facade and Extended Backend Interface

**Files:**
- Create: `server/pkg/agent/backend.go` (replaces current backend interface)

- [ ] **Step 1: Create unified backend facade**

```go
// server/pkg/agent/backend.go
package agent

import (
    "context"
    "github.com/agentra-ai/agentra/server/pkg/agent/providers"
)

// ProviderType identifies the type of backend.
type ProviderType string

const (
    ProviderClaude  ProviderType = "claude"
    ProviderCodex   ProviderType = "codex"
    ProviderOpenCode ProviderType = "opencode"
    ProviderOpenAI   ProviderType = "openai"
    ProviderAnthropic ProviderType = "anthropic"
    ProviderGemini   ProviderType = "gemini"
    ProviderOllama   ProviderType = "ollama"
    ProviderOpenRouter ProviderType = "openrouter"
)

// Backend is the unified interface for executing prompts via coding agents.
type Backend interface {
    Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error)
    ProviderType() ProviderType
    Model() string
    Capabilities() *Capabilities
}

// Capabilities describes what a backend supports.
type Capabilities struct {
    Streaming       bool
    ContextWindow   int
    Tools           bool
    MultiModal      bool
}

// BackendFacade provides a unified interface across all provider types.
type BackendFacade struct {
    cliBackends  map[ProviderType]Backend
    apiProviders map[ProviderType]providers.Provider
    default      ProviderType
}

func NewBackendFacade(defaultProvider ProviderType) *BackendFacade {
    return &BackendFacade{
        cliBackends:  make(map[ProviderType]Backend),
        apiProviders: make(map[ProviderType]providers.Provider),
        default:      defaultProvider,
    }
}

// RegisterCLIBackend registers a CLI-based backend (Claude Code, Codex, OpenCode).
func (f *BackendFacade) RegisterCLIBackend(p ProviderType, b Backend) {
    f.cliBackends[p] = b
}

// RegisterAPIProvider registers an API-based provider (OpenAI, Anthropic, Ollama).
func (f *BackendFacade) RegisterAPIProvider(p ProviderType, provider providers.Provider) {
    f.apiProviders[p] = provider
}

// Execute delegates to the appropriate backend.
func (f *BackendFacade) Execute(ctx context.Context, p ProviderType, prompt string, opts ExecOptions) (*Session, error) {
    if backend, ok := f.cliBackends[p]; ok {
        return backend.Execute(ctx, prompt, opts)
    }
    if provider, ok := f.apiProviders[p]; ok {
        return provider.Execute(ctx, prompt, providers.ExecOptions{
            Cwd:          opts.Cwd,
            Model:        opts.Model,
            SystemPrompt: opts.SystemPrompt,
            MaxTurns:     opts.MaxTurns,
            Timeout:      opts.Timeout,
        })
    }
    // Fallback to default
    if backend, ok := f.cliBackends[f.default]; ok {
        return backend.Execute(ctx, prompt, opts)
    }
    return nil, fmt.Errorf("no backend available for provider %s", p)
}

// ProviderForAgent returns the appropriate provider for an agent's configuration.
func ProviderForAgent(agent *Agent) ProviderType {
    if agent.Provider != "" {
        return ProviderType(agent.Provider)
    }
    return ProviderClaude
}
```

- [ ] **Step 2: Commit**

```bash
git add server/pkg/agent/backend.go
git commit -m "feat(agent): add BackendFacade for unified multi-provider access"
```

---

### Task 5: Agent Provider Configuration in Database

**Files:**
- Modify: `server/migrations/035_agent_provider.up.sql`
- Modify: `server/migrations/035_agent_provider.down.sql`
- Modify: `server/pkg/db/queries/agents.sql`
- Modify: `server/internal/handler/agent.go`

- [ ] **Step 1: Create migration**

```sql
-- 035_agent_provider.up.sql
ALTER TABLE agents ADD COLUMN provider TEXT NOT NULL DEFAULT 'claude';
ALTER TABLE agents ADD COLUMN model_override TEXT;
ALTER TABLE agents ADD COLUMN provider_config JSONB DEFAULT '{}';
```

```sql
-- 035_agent_provider.down.sql
ALTER TABLE agents DROP COLUMN IF EXISTS provider_config;
ALTER TABLE agents DROP COLUMN IF EXISTS model_override;
ALTER TABLE agents DROP COLUMN IF EXISTS provider;
```

- [ ] **Step 2: Update agents sqlc queries**

In `server/pkg/db/queries/agents.sql`, add to CreateAgent and UpdateAgent:

```sql
-- name: CreateAgent :one
INSERT INTO agents (workspace_id, name, agent_type, instructions, visibility, provider, model_override, provider_config)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateAgent :one
UPDATE agents SET
    name = COALESCE($2, name),
    instructions = COALESCE($3, instructions),
    visibility = COALESCE($4, visibility),
    provider = COALESCE(sqlc.narg('provider'), provider),
    model_override = COALESCE(sqlc.narg('model_override'), model_override),
    provider_config = COALESCE(sqlc.narg('provider_config'), provider_config),
    updated_at = NOW()
WHERE id = $1
RETURNING *;
```

- [ ] **Step 3: Update agent handler for provider switch**

```go
// server/internal/handler/agent.go
// Add to UpdateAgent handler
func (h *AgentHandler) UpdateAgent(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...
    var req struct {
        Name           *string `json:"name"`
        Instructions   *string `json:"instructions"`
        Visibility     *string `json:"visibility"`
        Provider       *string `json:"provider"`        // NEW
        ModelOverride  *string `json:"model_override"`  // NEW
        ProviderConfig *string `json:"provider_config"` // NEW - JSON string
    }
    // parse and call UpdateAgent with new fields
}
```

- [ ] **Step 4: Commit**

```bash
git add server/migrations/035_agent_provider.up.sql server/migrations/035_agent_provider.down.sql
git add server/pkg/db/queries/agents.sql
git add server/internal/handler/agent.go
git commit -m "feat(agent): add provider configuration fields to agents table"
```

---

## TRACK C: GitHub Integration

### Task 6: GitHub Tables and SQL Queries

**Files:**
- Create: `server/migrations/036_github_tables.up.sql`
- Create: `server/migrations/036_github_tables.down.sql`
- Create: `server/pkg/db/queries/github.sql`
- Create: `server/pkg/db/generated/github.sql.go` (generated)

- [ ] **Step 1: Create migration**

```sql
-- 036_github_tables.up.sql

CREATE TABLE github_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    installation_id BIGINT NOT NULL,
    account_login TEXT NOT NULL,
    account_type TEXT NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT,
    token_expires_at TIMESTAMPTZ,
    repositories JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE github_issue_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    repository TEXT NOT NULL,
    pr_number INT,
    commit_sha TEXT,
    branch_name TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX github_installations_workspace_idx ON github_installations(workspace_id);
CREATE INDEX github_issue_links_issue_idx ON github_issue_links(issue_id);
```

```sql
-- 036_github_tables.down.sql
DROP INDEX IF EXISTS github_issue_links_issue_idx;
DROP INDEX IF EXISTS github_installations_workspace_idx;
DROP TABLE IF EXISTS github_issue_links;
DROP TABLE IF EXISTS github_installations;
```

- [ ] **Step 2: Create github.sql queries**

```sql
-- name: CreateInstallation :one
INSERT INTO github_installations (workspace_id, installation_id, account_login, account_type, access_token, refresh_token, token_expires_at, repositories)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetInstallation :one
SELECT * FROM github_installations WHERE workspace_id = $1;

-- name: UpdateInstallationToken :one
UPDATE github_installations SET
    access_token = $2,
    refresh_token = COALESCE($3, refresh_token),
    token_expires_at = $4,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteInstallation :exec
DELETE FROM github_installations WHERE id = $1;

-- name: CreateIssueLink :one
INSERT INTO github_issue_links (issue_id, repository, pr_number, commit_sha, branch_name)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetIssueLinks :many
SELECT * FROM github_issue_links WHERE issue_id = $1;

-- name: UpdateIssueLink :one
UPDATE github_issue_links SET
    pr_number = COALESCE($2, pr_number),
    commit_sha = COALESCE($3, commit_sha),
    branch_name = COALESCE($4, branch_name)
WHERE id = $1
RETURNING *;

-- name: DeleteIssueLink :one
DELETE FROM github_issue_links WHERE id = $1 RETURNING *;
```

- [ ] **Step 3: Run sqlc generate**

```bash
cd server && sqlc generate
```

- [ ] **Step 4: Commit**

```bash
git add server/migrations/036_github_tables.up.sql server/migrations/036_github_tables.down.sql
git add server/pkg/db/queries/github.sql server/pkg/db/generated/github.sql.go
git commit -m "feat(github): add GitHub installations and issue links tables"
```

---

### Task 7: GitHub App and Webhook Handler

**Files:**
- Create: `server/pkg/github/go.mod`
- Create: `server/pkg/github/app.go`
- Create: `server/pkg/github/client.go`
- Create: `server/pkg/github/webhooks.go`
- Create: `server/pkg/github/pr.go`
- Create: `server/pkg/github/sync.go`

- [ ] **Step 1: Create go.mod**

```
module github.com/agentra-ai/agentra/pkg/github

go 1.26

require (
    github.com/google/uuid v1.6.0
    github.com/google/go-github/v67 v67.0.0
)
```

- [ ] **Step 2: Create app.go**

```go
package github

import (
    "context"
    "fmt"
    "github.com/google/go-github/v67/github"
)

type App struct {
    appID      int64
    privateKey []byte
    client     *github.Client
}

func NewApp(appID int64, privateKey []byte) *App {
    return &App{
        appID:      appID,
        privateKey: privateKey,
    }
}

// InstallForRepo returns an authenticated client for a specific installation.
func (a *App) InstallForRepo(ctx context.Context, installationID int64) (*github.Client, error) {
    token, _, err := a.client.Apps.ObtainInstallationToken(ctx, installationID, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to obtain installation token: %w", err)
    }

    client := github.NewTokenClient(token.GetToken())
    return client, nil
}

// CreatePR creates a pull request in the specified repository.
func (a *App) CreatePR(ctx context.Context, client *github.Client, owner, repo string, pr *PROptions) (*PR, error) {
    newPR := &github.NewPullRequest{
        Title:               github.String(pr.Title),
        Head:                github.String(pr.Head),
        Base:                github.String(pr.Base),
        Body:                github.String(pr.Body),
        MaintainerCanModify: github.Bool(true),
    }

    prResult, _, err := client.PullRequests.Create(ctx, owner, repo, newPR)
    if err != nil {
        return nil, fmt.Errorf("failed to create PR: %w", err)
    }

    return &PR{
        Number: prResult.GetNumber(),
        URL:    prResult.GetHTMLURL(),
        State:  prResult.GetState(),
    }, nil
}

type PROptions struct {
    Title string
    Head  string
    Base  string
    Body  string
}

type PR struct {
    Number int
    URL    string
    State  string
}
```

- [ ] **Step 3: Create webhooks.go**

```go
package github

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "io"
    "net/http"
)

type WebhookHandler struct {
    secret   []byte
    handler  WebhookEventHandler
}

type WebhookEventHandler interface {
    HandlePREvent(ctx context.Context, pr *PRPayload) error
    HandlePushEvent(ctx context.Context, push *PushPayload) error
    HandleCommentEvent(ctx context.Context, comment *CommentPayload) error
}

func NewWebhookHandler(secret string, h WebhookEventHandler) *WebhookHandler {
    return &WebhookHandler{secret: []byte(secret), handler: h}
}

func (wh *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    payload, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Failed to read body", 500)
        return
    }

    // Verify signature
    signature := r.Header.Get("X-Hub-Signature-256")
    if !wh.verifySignature(payload, signature) {
        http.Error(w, "Invalid signature", 401)
        return
    }

    event := r.Header.Get("X-GitHub-Event")

    switch event {
    case "pull_request":
        var pr PRPayload
        json.Unmarshal(payload, &pr)
        wh.handler.HandlePREvent(r.Context(), &pr)
    case "push":
        var push PushPayload
        json.Unmarshal(payload, &push)
        wh.handler.HandlePushEvent(r.Context(), &push)
    case "issue_comment":
        var comment CommentPayload
        json.Unmarshal(payload, &comment)
        wh.handler.HandleCommentEvent(r.Context(), &comment)
    }

    w.WriteHeader(200)
}

func (wh *WebhookHandler) verifySignature(payload []byte, signature string) bool {
    mac := hmac.New(sha256.New, wh.secret)
    mac.Write(payload)
    expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}

type PRPayload struct {
    Action      string `json:"action"`
    Number      int    `json:"number"`
    PullRequest struct {
        Title string `json:"title"`
        State string `json:"state"`
        HTMLURL string `json:"html_url"`
    } `json:"pull_request"`
    Repository struct {
        FullName string `json:"full_name"`
    } `json:"repository"`
}
```

- [ ] **Step 4: Create sync.go**

```go
package github

import (
    "context"
    "github.com/google/uuid"
)

type SyncService struct {
    queries *db.Queries
}

func NewSyncService(queries *db.Queries) *SyncService {
    return &SyncService{queries: queries}
}

func (s *SyncService) LinkIssueToPR(ctx context.Context, issueID, repo string, prNumber int) error {
    _, err := s.queries.CreateIssueLink(ctx, db.CreateIssueLinkParams{
        IssueID:   uuid.MustParse(issueID),
        Repository: repo,
        PrNumber:   int64(prNumber),
    })
    return err
}

func (s *SyncService) UpdatePRStatusForIssue(ctx context.Context, issueID string, status string) error {
    links, err := s.queries.GetIssueLinks(ctx, uuid.MustParse(issueID))
    if err != nil {
        return err
    }

    // Update each linked PR's status
    for _, link := range links {
        if link.PrNumber != nil {
            // TODO: Call GitHub API to update PR status
            _ = status
        }
    }
    return nil
}
```

- [ ] **Step 5: Commit**

```bash
git add server/pkg/github/go.mod server/pkg/github/app.go server/pkg/github/client.go
git add server/pkg/github/webhooks.go server/pkg/github/pr.go server/pkg/github/sync.go
git commit -m "feat(github): add GitHub App and webhook handler"
```

---

### Task 8: GitHub REST API and Frontend

**Files:**
- Create: `server/internal/handler/github.go`
- Create: `apps/web/features/github/hooks/useGitHub.ts`
- Create: `apps/web/features/github/api/githubApi.ts`
- Create: `apps/web/features/github/components/GitHubConnect.tsx`
- Create: `apps/web/features/github/components/PRStatusBadge.tsx`

- [ ] **Step 1: Create REST handler**

```go
// server/internal/handler/github.go
package handler

type GitHubHandler struct {
    github *github.App
    queries *db.Queries
}

func (h *GitHubHandler) RegisterRoutes(r chi.Router) {
    r.Get("/workspaces/{id}/github/installations", h.ListInstallations)
    r.Post("/workspaces/{id}/github/connect", h.ConnectGitHub)
    r.Delete("/workspaces/{id}/github/disconnect", h.DisconnectGitHub)
}

func (h *GitHubHandler) ListInstallations(w http.ResponseWriter, r *http.Request) {
    workspaceID := chi.URLParam(r, "id")
    inst, err := h.queries.GetInstallation(r.Context(), uuid.MustParse(workspaceID))
    if err != nil {
        http.Error(w, "not found", 404)
        return
    }
    json.NewEncoder(w).Encode(inst)
}

func (h *GitHubHandler) ConnectGitHub(w http.ResponseWriter, r *http.Request) {
    workspaceID := chi.URLParam(r, "id")
    var req struct {
        InstallationID int64  `json:"installation_id"`
        AccountLogin   string `json:"account_login"`
        AccountType    string `json:"account_type"`
        AccessToken    string `json:"access_token"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    inst, err := h.queries.CreateInstallation(r.Context(), db.CreateInstallationParams{
        WorkspaceID:    uuid.MustParse(workspaceID),
        InstallationID: req.InstallationID,
        AccountLogin:   req.AccountLogin,
        AccountType:    req.AccountType,
        AccessToken:    req.AccessToken,
    })
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    json.NewEncoder(w).Encode(inst)
}

func (h *GitHubHandler) DisconnectGitHub(w http.ResponseWriter, r *http.Request) {
    workspaceID := chi.URLParam(r, "id")
    inst, err := h.queries.GetInstallation(r.Context(), uuid.MustParse(workspaceID))
    if err != nil {
        http.Error(w, "not found", 404)
        return
    }
    h.queries.DeleteInstallation(r.Context(), inst.ID)
    w.WriteHeader(204)
}
```

- [ ] **Step 2: Create frontend hooks and API**

```typescript
// apps/web/features/github/hooks/useGitHub.ts
import { create } from 'zustand'

interface GitHubInstallation {
  id: string
  account_login: string
  account_type: string
  repositories: string[]
}

interface GitHubState {
  installation: GitHubInstallation | null
  isLoading: boolean
  error: string | null
  fetchInstallation: (workspaceId: string) => Promise<void>
  connect: (workspaceId: string, installationId: number) => Promise<void>
  disconnect: (workspaceId: string) => Promise<void>
}

export const useGitHubStore = create<GitHubState>((set) => ({
  installation: null,
  isLoading: false,
  error: null,

  fetchInstallation: async (workspaceId: string) => {
    set({ isLoading: true, error: null })
    try {
      const res = await fetch(`/api/workspaces/${workspaceId}/github/installations`)
      const data = await res.json()
      set({ installation: data || null, isLoading: false })
    } catch (e: any) {
      set({ error: e.message, isLoading: false })
    }
  },

  connect: async (workspaceId: string, installationId: number) => {
    const res = await fetch(`/api/workspaces/${workspaceId}/github/connect`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ installation_id: installationId }),
    })
    const data = await res.json()
    set({ installation: data })
  },

  disconnect: async (workspaceId: string) => {
    await fetch(`/api/workspaces/${workspaceId}/github/disconnect`, { method: 'DELETE' })
    set({ installation: null })
  },
}))
```

- [ ] **Step 3: Create GitHubConnect component**

```typescript
// apps/web/features/github/components/GitHubConnect.tsx
import { useGitHubStore } from '../hooks/useGitHub'

export function GitHubConnect({ workspaceId }: { workspaceId: string }) {
  const { installation, isLoading, fetchInstallation, connect, disconnect } = useGitHubStore()

  useEffect(() => {
    fetchInstallation(workspaceId)
  }, [workspaceId])

  if (isLoading) return <div>Loading...</div>

  if (!installation) {
    return (
      <div className="border rounded p-4">
        <h3 className="font-semibold mb-2">Connect GitHub</h3>
        <p className="text-muted-foreground text-sm mb-4">
          Connect your GitHub account to enable PR status sync and automatic commits.
        </p>
        <Button onClick={() => window.location.href = '/api/github/oauth'}>
          Connect GitHub App
        </Button>
      </div>
    )
  }

  return (
    <div className="border rounded p-4">
      <div className="flex justify-between items-center">
        <div>
          <span className="font-medium">{installation.account_login}</span>
          <span className="text-muted-foreground text-sm ml-2">
            ({installation.account_type})
          </span>
        </div>
        <Button variant="destructive" onClick={() => disconnect(workspaceId)}>
          Disconnect
        </Button>
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/github.go
git add apps/web/features/github/hooks/useGitHub.ts apps/web/features/github/api/githubApi.ts
git add apps/web/features/github/components/GitHubConnect.tsx apps/web/features/github/components/PRStatusBadge.tsx
git commit -m "feat(github): add REST API and frontend components"
```

---

## TRACK D: Memory Auto-Learning

### Task 9: Continuous Capture and Configurable Threshold Hooks

**Files:**
- Create: `server/pkg/memory/hooks/continuous.go`
- Create: `server/pkg/memory/hooks/extractor.go`
- Modify: `server/pkg/memory/hooks/task_completion.go` (already exists)

- [ ] **Step 1: Create continuous capture hook**

```go
// server/pkg/memory/hooks/continuous.go
package hooks

import (
    "context"
    "log/slog"

    "github.com/agentra-ai/agentra/server/pkg/memory"
)

// ShouldCapture determines if a trace step contains valuable memory content.
func ShouldCapture(step *TraceStep) bool {
    // Capture tool results that look like learnings
    if step.Action == "tool_result" {
        output := step.OutputText
        // Keywords that suggest valuable information
        keywords := []string{"remember", "note", "important", "use", "avoid", "don't", "always", "never"}
        for _, kw := range keywords {
            if contains(output, kw) {
                return true
            }
        }
        // Long outputs (>500 chars) with code patterns might be useful
        if len(output) > 500 && contains(output, "function") || contains(output, "class") {
            return true
        }
    }
    return false
}

func contains(s, substr string) bool {
    return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
    for i := 0; i <= len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return true
        }
    }
    return false
}

// ContinuousCapture stores valuable insights from ongoing task execution.
func ContinuousCapture(ctx context.Context, svc *memory.MemoryService, step *TraceStep) error {
    if !ShouldCapture(step) {
        return nil
    }

    return svc.Store(ctx, &memory.StoreInput{
        WorkspaceID: step.WorkspaceID,
        AgentID:     step.AgentID,
        Type:        "context",
        Content:     step.OutputText,
        Metadata: map[string]any{
            "step":      step.StepNumber,
            "tool":      step.Tool,
            "timestamp": step.Timestamp,
        },
    })
}
```

- [ ] **Step 2: Create configurable threshold extractor**

```go
// server/pkg/memory/hooks/extractor.go
package hooks

import (
    "context"
    "log/slog"

    "github.com/agentra-ai/agentra/server/pkg/memory"
)

type ExtractionConfig struct {
    TokenThreshold int
    StepThreshold  int
    ExtractOnError bool
}

func DefaultExtractionConfig() *ExtractionConfig {
    return &ExtractionConfig{
        TokenThreshold: 5000,
        StepThreshold:  50,
        ExtractOnError: true,
    }
}

// ConfigurableExtractor checks if task meets threshold and extracts learnings.
func ConfigurableExtractor(ctx context.Context, svc *memory.MemoryService, task *AgentTask, config *ExtractionConfig) error {
    if config == nil {
        config = DefaultExtractionConfig()
    }

    shouldExtract := false

    // Check token threshold
    if task.TotalTokens > config.TokenThreshold {
        shouldExtract = true
    }

    // Check step threshold
    if task.TotalSteps > config.StepThreshold {
        shouldExtract = true
    }

    // Check error condition
    if config.ExtractOnError && task.Error != "" {
        shouldExtract = true
    }

    if !shouldExtract {
        return nil
    }

    // Extract and store learnings
    learnings := ExtractLearnings(task.Output)
    for _, learning := range learnings {
        err := svc.Store(ctx, &memory.StoreInput{
            WorkspaceID: task.WorkspaceID,
            AgentID:     task.AgentID,
            Type:        "pattern",
            Content:     learning,
        })
        if err != nil {
            slog.Error("failed to store learning", "error", err)
        }
    }

    return nil
}
```

- [ ] **Step 3: Update task_completion hook with improved extractor**

```go
// Update server/pkg/memory/hooks/task_completion.go
package hooks

import (
    "context"
    "strings"

    "github.com/agentra-ai/agentra/server/pkg/memory"
)

// ExtractLearnings extracts key learnings from task output.
func ExtractLearnings(output string) []string {
    var learnings []string

    // Simple pattern-based extraction
    // Look for lines starting with "-", "*", or numbered patterns
    lines := strings.Split(output, "\n")
    for _, line := range lines {
        trimmed := strings.TrimSpace(line)
        if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
            learnings = append(learnings, strings.TrimPrefix(trimmed, "- "))
        }
    }

    // If no structured learnings found, extract last paragraph as summary
    if len(learnings) == 0 && len(output) > 100 {
        paragraphs := strings.Split(output, "\n\n")
        if len(paragraphs) > 0 {
            lastParagraph := strings.TrimSpace(paragraphs[len(paragraphs)-1])
            if len(lastParagraph) > 50 && len(lastParagraph) < 500 {
                learnings = append(learnings, lastParagraph)
            }
        }
    }

    return learnings
}

// OnTaskComplete extracts learnings from completed task and stores them.
func OnTaskComplete(ctx context.Context, svc *memory.MemoryService, task *AgentTask, result *TaskResult) error {
    learnings := ExtractLearnings(result.Output)

    for _, learning := range learnings {
        err := svc.Store(ctx, &memory.StoreInput{
            WorkspaceID: task.WorkspaceID,
            AgentID:     task.AgentID,
            Type:        "learning",
            Content:     learning,
        })
        if err != nil {
            return err
        }
    }

    return nil
}
```

- [ ] **Step 4: Commit**

```bash
git add server/pkg/memory/hooks/continuous.go server/pkg/memory/hooks/extractor.go
git add server/pkg/memory/hooks/task_completion.go
git commit -m "feat(memory): add continuous capture and configurable threshold hooks"
```

---

## TRACK E: Swarm Delegation

### Task 10: Delegation Scheduler with Docker Isolation

**Files:**
- Create: `server/pkg/taskgraph/delegation.go`
- Create: `server/pkg/taskgraph/executor.go`
- Create: `server/pkg/taskgraph/container.go`
- Create: `server/migrations/037_agent_delegation.up.sql`
- Create: `server/migrations/037_agent_delegation.down.sql`

- [ ] **Step 1: Create delegation policy migration**

```sql
-- 037_agent_delegation.up.sql

CREATE TABLE agent_delegation_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    from_agent_id UUID REFERENCES agents(id),
    to_agent_type TEXT NOT NULL CHECK (to_agent_type IN ('planner', 'executor', 'synthesis')),
    max_depth INT DEFAULT 3,
    allow_parallel BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX agent_delegation_policies_workspace_idx ON agent_delegation_policies(workspace_id);
```

```sql
-- 037_agent_delegation.down.sql
DROP INDEX IF EXISTS agent_delegation_policies_workspace_idx;
DROP TABLE IF EXISTS agent_delegation_policies;
```

- [ ] **Step 2: Create delegation.go**

```go
package taskgraph

import (
    "context"
    "log/slog"
    "sync"
)

type DelegationScheduler struct {
    store     *GraphStore
    executor  *Executor
    container *ContainerManager
}

func NewDelegationScheduler(store *GraphStore, executor *Executor, container *ContainerManager) *DelegationScheduler {
    return &DelegationScheduler{
        store:     store,
        executor:  executor,
        container: container,
    }
}

// Schedule determines execution strategy for all ready nodes in an issue.
func (s *DelegationScheduler) Schedule(ctx context.Context, issueID string) error {
    readyNodes, err := s.store.GetReadyNodes(ctx, issueID)
    if err != nil {
        return err
    }

    // Classify nodes by dependency
    parallelNodes, sequentialChains := classifyByDependency(readyNodes)

    var wg sync.WaitGroup

    // Execute parallel nodes concurrently
    if len(parallelNodes) > 0 {
        wg.Add(1)
        go func() {
            defer wg.Done()
            s.executeParallel(ctx, parallelNodes)
        }()
    }

    // Execute sequential chains
    for _, chain := range sequentialChains {
        wg.Add(1)
        go func() {
            defer wg.Done()
            s.executeSequential(ctx, chain)
        }()
    }

    wg.Wait()
    return nil
}

func classifyByDependency(nodes []GraphNode) ([]GraphNode, [][]GraphNode) {
    parallel := []GraphNode{}
    sequential := [][]GraphNode{}

    for _, node := range nodes {
        if hasBlockingDependencies(node) {
            // Add to sequential chain
            found := false
            for _, chain := range sequential {
                if canJoinChain(node, chain) {
                    sequential = append(sequential, append(chain, node))
                    found = true
                    break
                }
            }
            if !found {
                sequential = append(sequential, []GraphNode{node})
            }
        } else {
            parallel = append(parallel, node)
        }
    }

    return parallel, sequential
}

func hasBlockingDependencies(node GraphNode) bool {
    // Check if any parent nodes are not completed
    // This would use store.ListEdgesByIssue to find incoming edges
    return false
}

func canJoinChain(node GraphNode, chain []GraphNode) bool {
    // Check if node depends on any node in the chain
    return false
}
```

- [ ] **Step 3: Create executor.go**

```go
package taskgraph

import (
    "context"
    "log/slog"
)

func (s *DelegationScheduler) executeParallel(ctx context.Context, nodes []GraphNode) error {
    var wg sync.WaitGroup
    for _, node := range nodes {
        wg.Add(1)
        go func(n GraphNode) {
            defer wg.Done()
            err := s.executeNode(ctx, &n)
            if err != nil {
                slog.Error("parallel execution failed", "node_id", n.ID, "error", err)
            }
        }(node)
    }
    wg.Wait()
    return nil
}

func (s *DelegationScheduler) executeSequential(ctx context.Context, chain []GraphNode) error {
    for _, node := range chain {
        err := s.executeNode(ctx, &node)
        if err != nil {
            // Stop chain on failure
            s.store.TransitionNode(ctx, node.ID, StatusFailed)
            return err
        }
    }
    return nil
}

func (s *DelegationScheduler) executeNode(ctx context.Context, node *GraphNode) error {
    // Build handoff context
    handoffCtx, err := s.buildHandoffContext(ctx, node.ID)
    if err != nil {
        return err
    }

    // Transition to running
    s.store.TransitionNode(ctx, node.ID, StatusRunning)

    // Execute based on node type
    result, err := s.executor.Execute(ctx, node, handoffCtx)
    if err != nil {
        s.store.TransitionNode(ctx, node.ID, StatusFailed)
        return err
    }

    // Store result and transition to completed
    node.Result = result
    s.store.UpdateNodeResult(ctx, node.ID, result)
    s.store.TransitionNode(ctx, node.ID, StatusCompleted)

    return nil
}

func (s *DelegationScheduler) buildHandoffContext(ctx context.Context, nodeID string) (*HandoffContext, error) {
    // Use the existing HandoffProtocol
    protocol := NewHandoffProtocol(s.store, nil)
    return protocol.BuildHandoffContext(ctx, nodeID)
}
```

- [ ] **Step 4: Create container.go**

```go
package taskgraph

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/client"
)

type ContainerManager struct {
    docker     *client.Client
    image      string
    networkName string
}

func NewContainerManager(image, networkName string) (*ContainerManager, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv)
    if err != nil {
        return nil, err
    }

    return &ContainerManager{
        docker:      cli,
        image:       image,
        networkName: networkName,
    }, nil
}

func (m *ContainerManager) Execute(ctx context.Context, node *GraphNode, prompt string) (*ExecutionResult, error) {
    // Create container
    resp, err := m.docker.ContainerCreate(ctx, &container.Config{
        Image: m.image,
        Cmd:   []string{"agent", "execute", "--prompt", prompt},
        Env:   []string{
            fmt.Sprintf("AGENT_TYPE=%s", node.NodeType),
            fmt.Sprintf("NODE_ID=%s", node.ID),
        },
    }, nil, nil, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create container: %w", err)
    }

    defer m.docker.ContainerRemove(ctx, resp.ID, container.RemoveOptions{})

    // Wait for completion with timeout
    timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
    defer cancel()

    statusCh, errCh := m.docker.ContainerWait(timeoutCtx, resp.ID, container.WaitConditionNotRunning)

    select {
    case result := <-statusCh:
        output, _ := m.docker.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true})

        return &ExecutionResult{
            ExitCode: int(result.StatusCode),
            Output:   parseLogs(output),
        }, nil
    case err := <-errCh:
        return nil, fmt.Errorf("container wait failed: %w", err)
    }
}

type ExecutionResult struct {
    ExitCode int
    Output   string
    Cost     float64
    Duration time.Duration
}

func parseLogs(logs []byte) string {
    // Docker logs have a header format, strip it
    return string(logs)
}
```

- [ ] **Step 5: Commit**

```bash
git add server/migrations/037_agent_delegation.up.sql server/migrations/037_agent_delegation.down.sql
git add server/pkg/taskgraph/delegation.go server/pkg/taskgraph/executor.go server/pkg/taskgraph/container.go
git commit -m "feat(taskgraph): add delegation scheduler with Docker isolation"
```

---

## Integration Tasks

### Task 11: Wire Trace Recording into Task Lifecycle

**Files:**
- Modify: `server/internal/service/task.go` (find where task executes and add recorder)

- [ ] **Step 1: Add trace recording to task execution**

```go
// In server/internal/service/task.go
// Add recorder to task execution flow

func (s *TaskService) ExecuteTask(ctx context.Context, task *AgentTask) error {
    // Create trace recorder
    runID := uuid.New()
    recorder := traces.NewTraceRecorder(s.pool, task.ID, runID)

    // Create task run record
    _, err := s.queries.CreateTaskRun(ctx, db.CreateTaskRunParams{
        TaskID:  task.ID,
        AgentID: task.AgentID,
    })
    if err != nil {
        return err
    }

    // Wrap backend to record steps
    wrappedBackend := traces.NewRecordingBackend(s.backend, recorder)

    // Execute with wrapped backend
    session, err := wrappedBackend.Execute(ctx, task.Prompt, agent.ExecOptions{
        Cwd:       task.WorkingDirectory,
        Timeout:   task.Timeout,
    })
    if err != nil {
        traces.CompleteTaskRun(ctx, s.pool, runID, "failed", err.Error())
        return err
    }

    // Stream results and record
    for msg := range session.Messages {
        recorder.RecordStep(ctx, &traces.TraceStep{
            Action:    string(msg.Type),
            Tool:      msg.Tool,
            InputText: formatInput(msg),
            OutputText: msg.Content,
            Timestamp: time.Now(),
        })
    }

    // Get result
    result := <-session.Result

    // Complete task run
    traces.CompleteTaskRun(ctx, s.pool, runID, result.Status, result.Output)

    return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add server/internal/service/task.go
git commit -m "feat(traces): wire trace recording into task lifecycle"
```

---

### Task 12: Update Roadmap Documentation

**Files:**
- Modify: `docs/ROADMAP.md` (update Phase 2 status and add new features)

- [ ] **Step 1: Update ROADMAP.md**

Update the "Known Gaps" section to reflect completed features and update Phase 2 status.

- [ ] **Step 2: Commit**

```bash
git add docs/ROADMAP.md
git commit -m "docs: update ROADMAP with enhanced feature status"
```

---

## Spec Coverage Check

| Spec Section | Task(s) |
|--------------|---------|
| Multi-Provider Backend Interface | Task 4, 5 |
| GitHub App OAuth + Webhooks | Task 6, 7, 8 |
| Execution Traces (task_runs + trace_steps) | Task 1, 2, 3, 11 |
| Memory Auto-Learning (A+B+C) | Task 9 |
| Swarm Delegation (DAG + Docker) | Task 10 |

All spec sections are covered.

---

## Implementation Order

1. **Task 1** - Trace Database Schema (foundation)
2. **Task 2** - Trace Go Module
3. **Task 3** - Trace REST API + Frontend
4. **Task 6** - GitHub Tables (parallel to Task 1)
5. **Task 7** - GitHub App + Webhooks
6. **Task 8** - GitHub REST API + Frontend
7. **Task 4** - Provider Facade
8. **Task 5** - Agent Provider Config
9. **Task 9** - Memory Auto-Learning Hooks
10. **Task 10** - Swarm Delegation
11. **Task 11** - Wire Traces into Task Lifecycle
12. **Task 12** - Update Roadmap