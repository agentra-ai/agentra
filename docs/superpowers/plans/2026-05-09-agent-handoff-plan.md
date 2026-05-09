# Agent-to-Agent Handoff Implementation Plan

> **For agentic workers:** Use subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Task Graph system for multi-agent task decomposition with DAG dependency execution and agent-to-agent handoff protocol.

**Architecture:** Standalone `server/pkg/taskgraph` Go module with node/edge tables. Task graph nodes connect to issues via `issue_id` and agents via `agent_id`. Scheduler manages state transitions and dependency checking. Handoff protocol constructs full context bundles.

**Tech Stack:** Go 1.26, pgx/v5, PostgreSQL, Next.js 16, react-flow (for DAG visualization)

---

## File Structure

```
server/
├── pkg/
│   └── taskgraph/
│       ├── go.mod
│       ├── types.go           # GraphNode, GraphEdge, NodeType, EdgeType
│       ├── store.go           # GraphStore: CRUD for nodes/edges
│       ├── scheduler.go       # GraphScheduler: state machine + dependency check
│       └── handoff.go         # HandoffProtocol: build context bundles
│
migrations/
├── 034_task_graph.up.sql
└── 034_task_graph.down.sql
│
server/pkg/db/queries/
├── taskgraph.sql              # sqlc queries
│
server/internal/handler/
├── taskgraph.go               # REST API handlers
│
apps/web/features/
├── taskgraph/
│   ├── components/
│   │   ├── SubtaskTree.tsx
│   │   ├── GraphView.tsx
│   │   └── NodeCard.tsx
│   ├── hooks/
│   │   └── useTaskGraph.ts
│   └── api/
│       └── taskGraphApi.ts
```

---

## Task 1: Database Migrations

**Files:**
- Create: `server/migrations/034_task_graph.up.sql`
- Create: `server/migrations/034_task_graph.down.sql`

- [ ] **Step 1: Create migration up**

```sql
-- 034_task_graph.up.sql
CREATE TABLE task_graph_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id),
    node_type TEXT NOT NULL CHECK (node_type IN ('root','planner','executor','synthesis')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','running','completed','failed','blocked')),
    context JSONB DEFAULT '{}',
    result JSONB,
    position_x FLOAT,
    position_y FLOAT,
    depth INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX task_graph_nodes_issue_id_idx ON task_graph_nodes(issue_id);
CREATE INDEX task_graph_nodes_workspace_id_idx ON task_graph_nodes(workspace_id);

CREATE TABLE task_graph_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_node_id UUID NOT NULL REFERENCES task_graph_nodes(id) ON DELETE CASCADE,
    to_node_id UUID NOT NULL REFERENCES task_graph_nodes(id) ON DELETE CASCADE,
    edge_type TEXT NOT NULL DEFAULT 'depends_on'
        CHECK (edge_type IN ('depends_on','handoff','triggers')),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX task_graph_edges_from_id_idx ON task_graph_edges(from_node_id);
CREATE INDEX task_graph_edges_to_id_idx ON task_graph_edges(to_node_id);
```

- [ ] **Step 2: Create migration down**

```sql
-- 034_task_graph.down.sql
DROP INDEX IF EXISTS task_graph_edges_to_id_idx;
DROP INDEX IF EXISTS task_graph_edges_from_id_idx;
DROP TABLE IF EXISTS task_graph_edges;
DROP INDEX IF EXISTS task_graph_nodes_workspace_id_idx;
DROP INDEX IF EXISTS task_graph_nodes_issue_id_idx;
DROP TABLE IF EXISTS task_graph_nodes;
```

- [ ] **Step 3: Commit**

```bash
git add server/migrations/034_task_graph.up.sql server/migrations/034_task_graph.down.sql
git commit -m "feat(taskgraph): add task_graph_nodes and task_graph_edges tables"
```

---

## Task 2: SQL Queries (sqlc)

**Files:**
- Create: `server/pkg/db/queries/taskgraph.sql`
- Regenerate: `server/pkg/db/generated/taskgraph.sql.go`

- [ ] **Step 1: Write taskgraph.sql**

```sql
-- name: CreateTaskNode :one
INSERT INTO task_graph_nodes (workspace_id, issue_id, agent_id, node_type, status, context, depth)
VALUES ($1, $2, $3, $4, 'pending', $5, $6)
RETURNING *;

-- name: GetTaskNode :one
SELECT * FROM task_graph_nodes WHERE id = $1;

-- name: ListNodesByIssue :many
SELECT * FROM task_graph_nodes WHERE issue_id = $1 ORDER BY depth, created_at;

-- name: UpdateTaskNode :one
UPDATE task_graph_nodes SET
    agent_id = COALESCE(sqlc.narg('agent_id'), agent_id),
    status = COALESCE(sqlc.narg('status'), status),
    context = COALESCE(sqlc.narg('context'), context),
    result = COALESCE(sqlc.narg('result'), result),
    position_x = COALESCE(sqlc.narg('position_x'), position_x),
    position_y = COALESCE(sqlc.narg('position_y'), position_y),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetReadyNodes :many
-- Returns pending nodes whose all dependencies (from_edges) are completed
SELECT n.* FROM task_graph_nodes n
WHERE n.issue_id = $1 AND n.status = 'pending'
  AND NOT EXISTS (
    SELECT 1 FROM task_graph_edges e
    JOIN task_graph_nodes dep ON dep.id = e.from_node_id
    WHERE e.to_node_id = n.id
      AND e.edge_type = 'depends_on'
      AND dep.status NOT IN ('completed')
  );

-- name: CreateTaskEdge :one
INSERT INTO task_graph_edges (from_node_id, to_node_id, edge_type, metadata)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListEdgesByIssue :many
SELECT e.* FROM task_graph_edges e
JOIN task_graph_nodes n ON n.id = e.from_node_id OR n.id = e.to_node_id
WHERE n.issue_id = $1
ORDER BY e.created_at;

-- name: DeleteTaskNode :one
DELETE FROM task_graph_nodes WHERE id = $1 RETURNING *;

-- name: DeleteTaskEdge :one
DELETE FROM task_graph_edges WHERE id = $1 RETURNING *;
```

- [ ] **Step 2: Regenerate** — `cd server && sqlc generate`

- [ ] **Step 3: Commit**

```bash
git add server/pkg/db/queries/taskgraph.sql server/pkg/db/generated/taskgraph.sql.go
git commit -m "feat(taskgraph): add sqlc queries for task graph"
```

---

## Task 3: Go Module and Types

**Files:**
- Create: `server/pkg/taskgraph/go.mod`
- Create: `server/pkg/taskgraph/types.go`

- [ ] **Step 1: Create go.mod**

```
module github.com/agentra-ai/agentra/pkg/taskgraph

go 1.26

require (
    github.com/jackc/pgx/v5 v5.6.0
    github.com/google/uuid v1.6.0
)
```

- [ ] **Step 2: Create types.go**

```go
package taskgraph

type NodeType string
const (
    NodeTypeRoot      NodeType = "root"
    NodeTypePlanner   NodeType = "planner"
    NodeTypeExecutor  NodeType = "executor"
    NodeTypeSynthesis NodeType = "synthesis"
)

type NodeStatus string
const (
    StatusPending   NodeStatus = "pending"
    StatusRunning   NodeStatus = "running"
    StatusCompleted NodeStatus = "completed"
    StatusFailed    NodeStatus = "failed"
    StatusBlocked   NodeStatus = "blocked"
)

type EdgeType string
const (
    EdgeDependsOn EdgeType = "depends_on"
    EdgeHandoff   EdgeType = "handoff"
    EdgeTriggers  EdgeType = "triggers"
)

type GraphNode struct {
    ID          string            `json:"id"`
    WorkspaceID string            `json:"workspace_id"`
    IssueID     string            `json:"issue_id"`
    AgentID     string            `json:"agent_id,omitempty"`
    NodeType    NodeType          `json:"node_type"`
    Status      NodeStatus        `json:"status"`
    Context     map[string]any    `json:"context"`
    Result      map[string]any    `json:"result,omitempty"`
    PositionX   float64           `json:"position_x"`
    PositionY   float64           `json:"position_y"`
    Depth       int               `json:"depth"`
    CreatedAt   string            `json:"created_at"`
}

type GraphEdge struct {
    ID         string         `json:"id"`
    FromNodeID string         `json:"from_node_id"`
    ToNodeID   string         `json:"to_node_id"`
    EdgeType   EdgeType       `json:"edge_type"`
    Metadata   map[string]any `json:"metadata"`
}
```

- [ ] **Step 3: Commit**

```bash
git add server/pkg/taskgraph/go.mod server/pkg/taskgraph/types.go
git commit -m "feat(taskgraph): add module and types"
```

---

## Task 4: Graph Store

**Files:**
- Create: `server/pkg/taskgraph/store.go`

```go
package taskgraph

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/google/uuid"
    db "github.com/agentra-ai/agentra/pkg/db/generated"
)

type GraphStore struct {
    pool    *pgxpool.Pool
    queries *db.Queries
}

func NewGraphStore(pool *pgxpool.Pool) *GraphStore {
    return &GraphStore{pool: pool, queries: db.New(pool)}
}

func (s *GraphStore) CreateNode(ctx context.Context, workspaceID, issueID string, nodeType NodeType, depth int) (*GraphNode, error) {
    // implementation using s.queries.CreateTaskNode
}

func (s *GraphStore) GetNode(ctx context.Context, id string) (*GraphNode, error) {}

func (s *GraphStore) ListNodesByIssue(ctx context.Context, issueID string) ([]GraphNode, error) {}

func (s *GraphStore) UpdateNode(ctx context.Context, id string, agentID *string, status *string, result []byte) (*GraphNode, error) {}

func (s *GraphStore) GetReadyNodes(ctx context.Context, issueID string) ([]GraphNode, error) {}

func (s *GraphStore) CreateEdge(ctx context.Context, from, to string, edgeType EdgeType) (*GraphEdge, error) {}

func (s *GraphStore) DeleteNode(ctx context.Context, id string) error {}

func (s *GraphStore) DeleteEdge(ctx context.Context, id string) error {}
```

- [ ] **Step 1: Write full store.go with all implementations**
- [ ] **Step 2: Commit**

```bash
git add server/pkg/taskgraph/store.go
git commit -m "feat(taskgraph): add GraphStore with node/edge CRUD"
```

---

## Task 5: Graph Scheduler

**Files:**
- Create: `server/pkg/taskgraph/scheduler.go`

```go
package taskgraph

type GraphScheduler struct {
    store *GraphStore
}

func NewGraphScheduler(store *GraphStore) *GraphScheduler {
    return &GraphScheduler{store: store}
}

// GetReadyNodes returns pending nodes with all dependencies completed
func (s *GraphScheduler) GetReadyNodes(ctx context.Context, issueID string) ([]GraphNode, error) {
    return s.store.GetReadyNodes(ctx, issueID)
}

// TransitionNode atomically transitions a node's status
func (s *GraphScheduler) TransitionNode(ctx context.Context, nodeID string, toStatus NodeStatus) error {
    statusStr := string(toStatus)
    _, err := s.store.UpdateNode(ctx, nodeID, nil, &statusStr, nil)
    return err
}

// IsGraphComplete checks if all executor/synthesis nodes are completed
func (s *GraphScheduler) IsGraphComplete(ctx context.Context, issueID string) (bool, error) {
    nodes, err := s.store.ListNodesByIssue(ctx, issueID)
    if err != nil { return false, err }
    for _, n := range nodes {
        if n.NodeType == NodeTypeExecutor || n.NodeType == NodeTypeSynthesis {
            if n.Status != StatusCompleted { return false, nil }
        }
    }
    return true, nil
}
```

- [ ] **Step 1: Write scheduler.go**
- [ ] **Step 2: Commit**

```bash
git add server/pkg/taskgraph/scheduler.go
git commit -m "feat(taskgraph): add GraphScheduler with state machine"
```

---

## Task 6: Handoff Protocol

**Files:**
- Create: `server/pkg/taskgraph/handoff.go`

```go
package taskgraph

type HandoffContext struct {
    ParentIssue       map[string]any   `json:"parent_issue"`
    CompletedSiblings []HandoffSibling `json:"completed_siblings"`
    RelevantMemories  []any            `json:"relevant_memories,omitempty"`
    Artifacts         []HandoffArtifact `json:"artifacts"`
    Instructions      string           `json:"instructions"`
}

type HandoffSibling struct {
    NodeID    string         `json:"node_id"`
    AgentName string         `json:"agent_name"`
    NodeType  string         `json:"node_type"`
    Result    map[string]any `json:"result"`
}

type HandoffArtifact struct {
    Type string `json:"type"` // "file", "memory"
    Path string `json:"path,omitempty"`
    ID   string `json:"id,omitempty"`
}

type HandoffProtocol struct {
    store     *GraphStore
    memorySvc any // optional, for injecting memories
}

func NewHandoffProtocol(store *GraphStore, memorySvc any) *HandoffProtocol {
    return &HandoffProtocol{store: store, memorySvc: memorySvc}
}

// BuildHandoffContext constructs the full context bundle for a node
func (h *HandoffProtocol) BuildHandoffContext(ctx context.Context, nodeID string) (*HandoffContext, error) {
    // 1. Get the node
    // 2. Get parent issue info
    // 3. Get completed sibling nodes and their results
    // 4. Get artifacts from sibling results
    // 5. Optionally inject relevant memories
    // 6. Return HandoffContext
}
```

- [ ] **Step 1: Write full handoff.go with implementation**
- [ ] **Step 2: Commit**

```bash
git add server/pkg/taskgraph/handoff.go
git commit -m "feat(taskgraph): add HandoffProtocol with context builder"
```

---

## Task 7: REST API Handlers

**Files:**
- Create: `server/internal/handler/taskgraph.go`

- [ ] **Step 1: Write handler**

```go
package handler

type TaskGraphHandler struct {
    store     *taskgraph.GraphStore
    scheduler *taskgraph.GraphScheduler
    handoff   *taskgraph.HandoffProtocol
}

func (h *TaskGraphHandler) RegisterRoutes(r chi.Router) {
    r.Post("/issues/{id}/graph", h.CreateGraph)
    r.Get("/issues/{id}/graph", h.GetGraph)
    r.Patch("/graph/nodes/{id}", h.UpdateNode)
    r.Delete("/graph/nodes/{id}", h.DeleteNode)
    r.Post("/graph/edges", h.CreateEdge)
}

func (h *TaskGraphHandler) CreateGraph(w http.ResponseWriter, r *http.Request) {}
func (h *TaskGraphHandler) GetGraph(w http.ResponseWriter, r *http.Request) {
    // Returns { nodes: [...], edges: [...] } for issue
}
```

- [ ] **Step 2: Commit**

```bash
git add server/internal/handler/taskgraph.go
git commit -m "feat(taskgraph): add REST API handlers"
```

---

## Task 8: Frontend — Store and API

**Files:**
- Create: `apps/web/features/taskgraph/hooks/useTaskGraph.ts`
- Create: `apps/web/features/taskgraph/api/taskGraphApi.ts`

```typescript
// useTaskGraph.ts
import { create } from 'zustand'

interface TaskGraphState {
    nodes: GraphNode[]
    edges: GraphEdge[]
    isLoading: boolean
    fetchGraph: (issueId: string) => Promise<void>
    updateNode: (id: string, data: Partial<GraphNode>) => Promise<void>
}

export const useTaskGraph = create<TaskGraphState>(...)
```

- [ ] **Step 1: Create store and API**
- [ ] **Step 2: Commit**

---

## Task 9: Frontend — SubtaskTree and GraphView

**Files:**
- Create: `apps/web/features/taskgraph/components/SubtaskTree.tsx`
- Create: `apps/web/features/taskgraph/components/GraphView.tsx`
- Create: `apps/web/features/taskgraph/components/NodeCard.tsx`

- [ ] **Step 1: Write SubtaskTree** — collapsible tree list on issue page
- [ ] **Step 2: Write NodeCard** — single node detail card
- [ ] **Step 3: Write GraphView** — DAG visualization using react-flow
- [ ] **Step 4: Commit**

---

## Task 10: Integrate into Issue Page

**Files:**
- Modify: `apps/web/app/(dashboard)/issues/[id]/page.tsx`

- [ ] **Step 1: Add SubtaskTree below issue description**
- [ ] **Step 2: Add "Graph" tab for DAG visualization**
- [ ] **Step 3: Commit**

---

## Task 11: Unit Tests

**Files:**
- Create: `server/pkg/taskgraph/store_test.go`
- Create: `server/pkg/taskgraph/scheduler_test.go`
- Create: `server/pkg/taskgraph/handoff_test.go`

- [ ] **Step 1: Write tests**
- [ ] **Step 2: Commit**

---

## Task 12: End-to-End Verification

- [ ] **Step 1: Run migrations** — `make migrate-up`
- [ ] **Step 2: Run Go tests** — `go test ./pkg/taskgraph/...`
- [ ] **Step 3: Run TS typecheck** — `pnpm typecheck`
- [ ] **Step 4: Commit**

---

## Spec Coverage Check

| Spec Section | Task(s) |
|--------------|---------|
| Database (nodes + edges) | Task 1 |
| SQL queries | Task 2 |
| Go module + types | Task 3 |
| GraphStore (CRUD) | Task 4 |
| GraphScheduler (state machine) | Task 5 |
| HandoffProtocol (context builder) | Task 6 |
| REST API | Task 7 |
| Frontend store + API | Task 8 |
| SubtaskTree + GraphView UI | Task 9 |
| Issue page integration | Task 10 |
| Tests | Task 11 |
| E2E verification | Task 12 |
