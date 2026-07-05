# Goal-to-DAG Auto-Decomposition Design

**Date**: 2026-05-10
**Status**: Draft
**Based on**: Competitive analysis v2 (open-multi-agent runTeam pattern), existing Task Graph system

---

## 1. Overview

### 1.1 Problem

Agentra's Task Graph system (`pkg/taskgraph/`) has a fully implemented DAG store, scheduler, handoff protocol, and delegation engine -- but nodes and edges must be created manually. The top competitor **open-multi-agent** (6,086 stars) provides `runTeam(team, goal)` -- a single call that auto-decomposes a goal into a task DAG with parallel/sequential execution.

This feature brings that capability to Agentra, leveraging the existing infrastructure.

### 1.2 Key Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Planner backend | Multi-provider facade (`pkg/agent/providers`) | Reuse existing provider abstraction; any LLM can plan |
| Output format | Structured JSON (strict schema) | Parseable by backend, deterministically persisted |
| Prompt strategy | Single-pass planner with JSON schema constraint | Simpler than iterative refinement; matches open-multi-agent approach |
| Human review | Mandatory between decompose and execute | Safety gate; users can edit node assignments, add/remove nodes |
| MCP exposure | `agentra_graph_decompose` tool | Existing MCP server pattern; agents can self-decompose |
| Storage | Existing `task_graph_nodes` + `task_graph_edges` tables | Zero new schema; decompose produces normal graph nodes |
| UI location | Button on issue detail page + review modal | Minimal new UI; extends existing SubtaskTree view |

### 1.3 Non-Goals

- Iterative/incremental decomposition (plan one node at a time) -- future work
- Planner agent as a persistent daemon role -- this is a one-shot API call
- Auto-execution without review -- mandatory human-in-the-loop for v1
- Goal streaming from frontend -- user types the goal in the issue description

---

## 2. Architecture

### 2.1 System Flow

```
User creates issue               User clicks "Auto-Decompose"
  │                                      │
  ▼                                      ▼
┌─────────────────────┐     POST /api/issues/:id/auto-decompose
│ Issue with goal     │         │
│ title + description │         ▼
└─────────────────────┘   ┌──────────────────────────────────┐
                          │ AutoDecomposeHandler             │
                          │  1. Load issue + workspace agents │
                          │  2. Build planner prompt          │
                          │  3. Call Provider.Execute()      │
                          │  4. Parse JSON response           │
                          │  5. Create nodes + edges in DB   │
                          │  6. Return graph to frontend      │
                          └──────────────────────────────────┘
                                      │
                                      ▼
                          ┌──────────────────────────────────┐
                          │ Review Modal (frontend)          │
                          │  - Show DAG visualization        │
                          │  - Edit node descriptions        │
                          │  - Reassign agents               │
                          │  - Add/remove nodes              │
                          │  - Add/remove edges              │
                          │  - [Confirm] or [Cancel]         │
                          └──────────────────────────────────┘
                                      │ Confirm
                                      ▼
                          ┌──────────────────────────────────┐
                          │ Graph persisted. User clicks     │
                          │ "Execute" to start DAG execution │
                          │ (existing DelegationScheduler)   │
                          └──────────────────────────────────┘
```

### 2.2 Integration Points

```
server/
├── cmd/server/router.go              # Add route: POST /api/issues/:id/auto-decompose
├── internal/handler/
│   ├── handler.go                    # Add GraphStore field to Handler struct
│   └── auto_decompose.go             # NEW: handler for auto-decomposition
├── internal/service/
│   └── planner.go                    # NEW: PlannerService (prompt building + LLM call)
├── pkg/taskgraph/                    # EXISTING: store.go, types.go (no changes needed)
├── pkg/agent/providers/             # EXISTING: provider.go (used as-is)
└── pkg/mcp/tools/
    └── graph.go                      # NEW: agentra_graph_decompose tool

apps/web/
├── features/taskgraph/
│   ├── api/taskGraphApi.ts           # ADD: autoDecompose() method
│   ├── components/
│   │   ├── AutoDecomposeButton.tsx   # NEW: button on issue page
│   │   ├── DecomposeReviewModal.tsx  # NEW: review + edit modal
│   │   ├── DAGEditor.tsx            # NEW: editable DAG canvas
│   │   ├── SubtaskTree.tsx          # EXISTING: shows persisted graph
│   │   └── GraphView.tsx            # EXISTING: DAG visualization (enhance)
│   └── hooks/
│       └── useTaskGraph.ts          # EXISTING: add decompose action
```

---

## 3. API Design

### 3.1 Endpoint

```
POST /api/issues/:id/auto-decompose
```

**Auth**: Workspace member (standard middleware chain)
**Content-Type**: `application/json`

**Request Body** (all optional -- falls back to issue data):

```json
{
  "model": "claude-sonnet-4-20250514",
  "provider": "anthropic",
  "max_nodes": 10,
  "additional_context": "Focus on security review of the auth module"
}
```

| Field | Type | Default | Description |
|---|---|---|---|
| `model` | string | workspace default | Model to use for planning |
| `provider` | string | "anthropic" | Provider type (anthropic, openai, openrouter, ollama) |
| `max_nodes` | integer | 10 | Maximum number of nodes in the generated DAG |
| `additional_context` | string | "" | Extra hints for the planner |

**Response** (200):

```json
{
  "graph": {
    "nodes": [
      {
        "id": "uuid-1",
        "node_type": "executor",
        "agent_id": "uuid-agent-architect",
        "status": "pending",
        "context": {
          "description": "Design the API schema for user authentication",
          "suggested_agent": "Architect Agent",
          "estimated_effort": "medium",
          "deliverable": "OpenAPI spec document"
        },
        "depth": 0,
        "position_x": 100,
        "position_y": 50
      }
    ],
    "edges": [
      {
        "id": "uuid-edge-1",
        "from_node_id": "uuid-1",
        "to_node_id": "uuid-2",
        "edge_type": "depends_on",
        "metadata": {
          "reason": "Implementation depends on API design being complete"
        }
      }
    ]
  },
  "plan": "## Execution Plan\n\n1. Architect designs the API schema\n2. Developer implements the endpoints (parallel: tests + code)\n3. Reviewer audits the implementation\n4. Synthesis agent produces the final summary",
  "metadata": {
    "model_used": "claude-sonnet-4-20250514",
    "tokens_used": 2500,
    "decomposition_time_ms": 3200,
    "node_count": 5,
    "edge_count": 4
  }
}
```

**Response** (202) -- async mode (future):

```json
{
  "decomposition_id": "uuid",
  "status": "processing",
  "message": "Decomposition is running. Poll GET /api/decompositions/:id for result."
}
```

**Error Responses**:

| Status | Body | Cause |
|---|---|---|
| 404 | `{"error": "issue not found"}` | Issue ID does not exist |
| 400 | `{"error": "issue has no description"}` | Empty description cannot be decomposed |
| 422 | `{"error": "planner returned invalid JSON"}` | LLM output did not match schema |
| 500 | `{"error": "planner call failed"}` | Provider error or timeout |
| 409 | `{"error": "graph already exists for this issue"}` | Refuse to overwrite (use ?force=true) |

### 3.2 Related Endpoints (Existing)

These already exist and are used by the frontend after decomposition:

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/issues/:id/graph` | Fetch current graph (used by SubtaskTree) |
| POST | `/api/issues/:id/graph` | Create graph manually (used for edits) |
| PATCH | `/api/graph/nodes/:id` | Update a single node (reassign agent, edit description) |
| DELETE | `/api/graph/nodes/:id` | Remove a node from the graph |

---

## 4. Planner Prompt Design

### 4.1 System Prompt Template

```
You are a task decomposition planner for Agentra, an AI-native task management platform.

Your job: Given a goal (issue title + description) and a team of available agents,
produce a Task DAG (Directed Acyclic Graph) that decomposes the goal into executable
subtasks with clear dependencies.

## Rules
1. Each node must be a concrete, independently executable task.
2. Edges represent "depends_on" relationships. Node B depends on Node A if B cannot
   start until A completes.
3. Nodes with no mutual dependencies should be placed at the same depth so they can
   execute in parallel.
4. Assign each node a `node_type`:
   - "executor" -- a work task (coding, writing, analysis, design)
   - "synthesis" -- combines results from multiple upstream nodes
5. Suggest an agent for each node from the available team. Include the `suggested_agent`
   name in the context. Match agent skills to task requirements.
6. Prefer width (parallelism) over depth (long chains). Minimize critical path length.
7. Maximum nodes: {max_nodes}
8. Include a `plan` field with a human-readable execution plan summary in markdown.

## Output Format
Respond with a single JSON object matching this schema:
{
  "plan": "string (markdown summary of the execution plan)",
  "nodes": [
    {
      "node_type": "executor | synthesis",
      "context": {
        "description": "string (what this node does)",
        "suggested_agent": "string (name of recommended agent)",
        "estimated_effort": "low | medium | high",
        "deliverable": "string (concrete output artifact)",
        "acceptance_criteria": ["string"]
      },
      "depth": 0,
      "dependencies": ["index of upstream node in this array, or empty"]
    }
  ]
}

Important:
- The first node in the array is index 0.
- Use array indices in `dependencies` to declare edges.
- Nodes at depth 0 have no dependencies.
- A `synthesis` node typically has multiple dependencies.
- Do NOT include node IDs -- the server assigns them.
- Output ONLY valid JSON, no markdown wrappers, no trailing commas.
```

### 4.2 User Prompt Template (sent to LLM)

```
## Goal

**Title**: {issue.title}
**Description**: {issue.description}

## Available Agents

{formatted_agent_list}

## Additional Context

{additional_context}

Decompose this goal into a task DAG. Return the JSON.
```

### 4.3 Agent List Formatting

Agents are formatted in the prompt as:

```
### {agent.name} ({agent.role})
- Skills: {skill1}, {skill2}, {skill3}
- ID: {agent.id}
- Provider: {agent.provider}
```

The agent list is loaded from `ListAgents` (which includes `ListAgentSkills` join). Only active, non-archived agents are included.

### 4.4 Provider Selection

The planner call uses the **multi-provider facade** (`pkg/agent/providers`):

```go
provider, err := agentproviders.NewProvider(req.Provider, agentproviders.APIConfig{
    APIKey:   resolvedAPIKey,
    Endpoint: resolvedEndpoint,
})
session, err := provider.Execute(ctx, fullPrompt, types.ExecOptions{
    Model:        req.Model,
    SystemPrompt: systemPromptTemplate,
    MaxTurns:     1,  // single-shot, no tool use needed
    Timeout:      60 * time.Second,
})
result := <-session.Result
```

The `MaxTurns: 1` constraint is critical -- this is a single-pass JSON generation, not an interactive agent session. The planner should not use tools; it should just output structured JSON.

---

## 5. JSON Schema for Planner Output

### 5.1 Go Struct

```go
type PlannerOutput struct {
    Plan  string         `json:"plan"`
    Nodes []PlannerNode  `json:"nodes"`
}

type PlannerNode struct {
    NodeType     string            `json:"node_type"`     // "executor" | "synthesis"
    Context      PlannerContext    `json:"context"`
    Depth        int               `json:"depth"`
    Dependencies []int             `json:"dependencies"`  // indices into Nodes array
}

type PlannerContext struct {
    Description       string   `json:"description"`
    SuggestedAgent    string   `json:"suggested_agent"`
    EstimatedEffort   string   `json:"estimated_effort"`   // "low" | "medium" | "high"
    Deliverable       string   `json:"deliverable"`
    AcceptanceCriteria []string `json:"acceptance_criteria"`
}
```

### 5.2 Validation Rules

Server-side validation before persisting:

1. `len(output.Nodes) > 0` -- at least one node
2. `len(output.Nodes) <= maxNodes` -- respect the cap (default 10, max 20)
3. All `node_type` values are `"executor"` or `"synthesis"`
4. All `dependencies` indices are in-bounds and reference earlier nodes (DAG acyclicity: dependencies must point to lower indices)
5. At least one node at `depth == 0` (entry point exists)
6. `context.description` is non-empty for every node
7. `plan` string is non-empty

If validation fails, the handler returns 422 with the specific validation error. The frontend can offer a "Retry" button (re-calls the endpoint; the LLM may produce valid output on the next attempt).

### 5.3 Node-to-DB Mapping

Each `PlannerNode` maps to a single `GraphNode` row:

| Planner Field | DB Column | Notes |
|---|---|---|
| `node_type` | `node_type` | Direct mapping |
| `context.description` | `context.description` | Stored in JSONB |
| `context.suggested_agent` | `context.suggested_agent` | Name match to resolve `agent_id` |
| `context.estimated_effort` | `context.estimated_effort` | Stored in JSONB |
| `context.deliverable` | `context.deliverable` | Stored in JSONB |
| `context.acceptance_criteria` | `context.acceptance_criteria` | Stored in JSONB |
| `depth` | `depth` | Direct mapping |
| `dependencies[i]` | Edge `from_node_id` -> `to_node_id` | Each entry creates one `depends_on` edge |

**Agent ID resolution**: The `suggested_agent` name is matched against the workspace agent list (case-insensitive substring match). If no match is found, `agent_id` is left NULL (unassigned). The user can assign agents during review.

**Position layout**: Nodes are auto-positioned in a layered DAG layout:
- `position_y = depth * 150` (vertical spacing)
- `position_x` computed using a simple left-to-right layout within each depth layer

---

## 6. Backend Implementation

### 6.1 PlannerService (`server/internal/service/planner.go`)

```go
type PlannerService struct {
    queries  *db.Queries
    graphStore *taskgraph.GraphStore
}

func (s *PlannerService) DecomposeIssue(ctx context.Context, workspaceID, issueID string, opts DecomposeOptions) (*DecomposeResult, error)
```

Responsibilities:
1. Load issue by ID (title + description)
2. Load workspace agents with skills (for agent list in prompt)
3. Resolve provider + model from opts or workspace defaults
4. Build prompt (system + user)
5. Call provider
6. Parse and validate JSON response
7. Resolve agent names to agent IDs
8. Batch-create nodes + edges in graph store
9. Return the complete graph

### 6.2 Handler (`server/internal/handler/auto_decompose.go`)

Standard Chi handler following existing handler patterns:

```go
func (h *Handler) AutoDecomposeIssue(w http.ResponseWriter, r *http.Request) {
    issueID := chi.URLParam(r, "id")
    workspaceID := resolveWorkspaceID(r)

    var req AutoDecomposeRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    result, err := h.PlannerService.DecomposeIssue(r.Context(), workspaceID, issueID, DecomposeOptions{
        Model:             req.Model,
        Provider:          req.Provider,
        MaxNodes:          req.MaxNodes,
        AdditionalContext: req.AdditionalContext,
    })
    if err != nil {
        // Map specific errors to status codes
        writeError(w, statusFromErr(err), err.Error())
        return
    }

    writeJSON(w, http.StatusOK, result)
}
```

### 6.3 Handler Struct Changes

Add to `Handler` in `handler.go`:

```go
type Handler struct {
    // ... existing fields ...
    GraphStore      *taskgraph.GraphStore    // NEW
    PlannerService  *service.PlannerService  // NEW
}
```

Wire up in `New()` and in `router.go`.

### 6.4 Router Registration

In `cmd/server/router.go`, add within the issues route group:

```go
r.Post("/{id}/auto-decompose", h.AutoDecomposeIssue)
```

### 6.5 MCP Tool (`server/pkg/mcp/tools/graph.go`)

```go
func GraphDecompose(ctx mcp.ToolContext, params map[string]any) (any, error) {
    // Calls PlannerService.DecomposeIssue internally
}

// Registration:
{
    Name:        "agentra_graph_decompose",
    Description: "Auto-decompose an issue goal into a task DAG. Returns nodes and edges.",
    InputSchema: mcp.ToolInputSchema{
        Type: "object",
        Properties: map[string]mcp.Property{
            "issue_id": {Type: "string", Description: "The issue ID to decompose"},
            "model":    {Type: "string", Description: "Optional: model override for planning"},
            "max_nodes": {Type: "integer", Description: "Optional: max nodes (default 10)"},
        },
        Required: []string{"issue_id"},
    },
}
```

This follows the existing MCP tool pattern (`agentra_issue_*`, `agentra_skill_*`) exactly.

---

## 7. Frontend UI Design

### 7.1 Auto-Decompose Button

**Location**: Issue detail page, in the toolbar area (next to existing actions), visible when:
- The issue has a description (non-empty)
- No graph already exists for the issue (or a "Regenerate" option with confirmation)

**Component**: `AutoDecomposeButton.tsx`

```
┌──────────────────────────────────────────┐
│ [Auto-Decompose ▼]                       │
│   - Use default model (Claude Sonnet)    │
│   - Choose model...                      │
│   - Max nodes: [5] [10] [15] [20]        │
└──────────────────────────────────────────┘
```

States:
- **Idle**: Button visible, clickable
- **Loading**: Spinner + "Decomposing goal..." + provider/model shown
- **Error**: Red outline + error message + "Retry" option
- **Success**: Button changes to "Review Graph (5 nodes)" -- clicking opens review modal

### 7.2 Review Modal

**Component**: `DecomposeReviewModal.tsx`

A full-screen or large modal with three sections:

```
┌─────────────────────────────────────────────────────────────┐
│  Review Task Graph                               [X Close]  │
│                                                             │
│  ┌─────────────────────────┐  ┌───────────────────────────┐ │
│  │                         │  │                           │ │
│  │   DAG Visualization     │  │   Node Details            │ │
│  │   (editable canvas)     │  │                           │ │
│  │                         │  │   Description: [editable] │ │
│  │   [node1]──→[node2]    │  │   Agent: [dropdown]       │ │
│  │   [node1]──→[node3]    │  │   Type: executor          │ │
│  │   [node2]──→[node4]    │  │   Effort: medium          │ │
│  │   [node3]──→[node4]    │  │   Deliverable: [editable] │ │
│  │                         │  │   Acceptance: [editable]  │ │
│  │   (+ Add Node)          │  │                           │ │
│  │                         │  │   [Delete Node]           │ │
│  └─────────────────────────┘  └───────────────────────────┘ │
│                                                             │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Plan Summary (markdown)                                 ││
│  │ ## Execution Plan                                       ││
│  │ 1. Architect designs API schema                         ││
│  │ 2. Developer + Tester work in parallel after step 1     ││
│  │ 3. Reviewer audits after implementation                 ││
│  └─────────────────────────────────────────────────────────┘│
│                                                             │
│  Model: claude-sonnet-4  |  5 nodes  |  4 edges  |  2.5K tokens │
│                                                             │
│            [Cancel]              [Save & Execute Later]     │
│                                 [Execute Now]              │
└─────────────────────────────────────────────────────────────┘
```

**Edit capabilities**:
- **Edit node**: Click a node to populate the detail panel. Edit description, reassign agent, change effort.
- **Add node**: "+" button creates a new unconnected node. Drag to position, then draw edges.
- **Delete node**: Remove a node (and its edges) from the graph.
- **Add edge**: Visual edge-drawing between nodes (click source port, click target port).
- **Reassign agent**: Dropdown populated from workspace agents list.

**Buttons**:
- **Cancel**: Discard the generated graph entirely. Soft-delete nodes from DB.
- **Save & Execute Later**: Persist the (potentially edited) graph. Node statuses remain "pending". User returns to issue page.
- **Execute Now**: Persist + trigger `DelegationScheduler.Schedule()` to begin DAG execution. The first ready nodes transition to "running".

### 7.3 State Flow

```
No Graph ──[Auto-Decompose]──> Generating ──[LLM returns]──> Review Modal
                                                                    │
                                                    ┌───────────────┼───────────────┐
                                                    ▼               ▼               ▼
                                                [Cancel]     [Save]          [Execute]
                                                    │               │               │
                                                    ▼               ▼               ▼
                                              Delete nodes    Nodes saved     Nodes saved
                                              Back to issue   Back to issue   Scheduler runs
                                              (no graph)      (graph shown)   (graph runs)
```

### 7.4 Existing Components to Modify

- **`SubtaskTree.tsx`**: Already renders on the issue detail page (line 680 of issue-detail.tsx). Enhance to show auto-decompose button when graph is empty.
- **`useTaskGraph.ts`**: Add `decomposeGraph(issueId, opts)` action that calls `POST /api/issues/:id/auto-decompose`.
- **`taskGraphApi.ts`**: Add `autoDecompose(issueId, opts)` method.
- **`GraphView.tsx`**: Replace placeholder with actual DAG rendering. Use a lightweight library (reactflow or a simple SVG layout).

---

## 8. Implementation Steps (Prioritized)

### Phase 1: Core Backend (P0)

| Step | Files | Description |
|---|---|---|
| 1.1 | `internal/service/planner.go` | Create `PlannerService` with prompt building + provider call + JSON parsing |
| 1.2 | `internal/handler/auto_decompose.go` | Create handler: parse request, call service, return response |
| 1.3 | `internal/handler/handler.go` | Add `GraphStore` + `PlannerService` to Handler struct |
| 1.4 | `cmd/server/router.go` | Register `POST /api/issues/:id/auto-decompose` route |
| 1.5 | `cmd/server/router.go` | Wire `GraphStore` into Handler construction |
| 1.6 | Tests | `TestAutoDecompose` -- mock provider, validate node/edge creation |

### Phase 2: Frontend (P0)

| Step | Files | Description |
|---|---|---|
| 2.1 | `features/taskgraph/api/taskGraphApi.ts` | Add `autoDecompose(issueId, opts)` |
| 2.2 | `features/taskgraph/hooks/useTaskGraph.ts` | Add `decomposeGraph` action + loading/error states |
| 2.3 | `features/taskgraph/components/AutoDecomposeButton.tsx` | Button with model selector dropdown |
| 2.4 | `features/taskgraph/components/DecomposeReviewModal.tsx` | Full review modal with DAG viz + node editing |
| 2.5 | `features/taskgraph/components/DAGEditor.tsx` | Editable DAG canvas (nodes + edges) |
| 2.6 | `features/issues/components/issue-detail.tsx` | Integrate AutoDecomposeButton into toolbar |

### Phase 3: MCP Integration (P1)

| Step | Files | Description |
|---|---|---|
| 3.1 | `pkg/mcp/tools/graph.go` | Implement `agentra_graph_decompose` tool |
| 3.2 | `cmd/mcp/main.go` / tool registration | Register the new tool in MCP server |

### Phase 4: Polish (P1)

| Step | Description |
|---|---|
| 4.1 | Async decomposition (202 Accepted + poll endpoint) for very large goals |
| 4.2 | "Regenerate" -- allow re-decomposition with different params, replacing existing graph |
| 4.3 | Graph diff view -- show what changed between regenerations |
| 4.4 | Planner quality metrics -- track acceptance rate (save vs. cancel), edit distance |

### Phase 5: Execution Integration (Already Possible)

The existing `DelegationScheduler.Schedule()` works with any graph created by decomposition:
- It reads ready nodes from `GraphStore.GetReadyNodes()`
- It classifies them as parallel/sequential
- It executes via the existing `Executor`
- Handoff context flows through `HandoffProtocol`

No new code needed for execution -- only the "Execute Now" button wiring.

---

## 9. Reference: open-multi-agent's Approach

### 9.1 How `runTeam` Works

open-multi-agent's `runTeam(team, goal)` API:

```typescript
const result = await runTeam(team, "Build a REST API for user authentication");
```

Internal flow:
1. `CoordinatorAgent` receives the goal string
2. Uses Claude Sonnet 4.6 with a system prompt that instructs it to output a structured task plan
3. The plan includes: task name, assigned agent role, dependencies (array of task names)
4. Coordinator spawns sub-agents for each task, respecting the dependency DAG
5. Results flow back through `onProgress` callbacks
6. Final result is a synthesis of all task outputs

### 9.2 Key Differences from Agentra's Design

| Dimension | open-multi-agent | Agentra (this design) |
|---|---|---|
| Persistence | In-memory only | PostgreSQL (durable DAG) |
| Human review | None (auto-executes) | Mandatory review modal |
| Agent assignment | By role string | By actual workspace agent ID |
| DAG visibility | HTML dashboard (post-run) | Real-time WebSocket + persistent UI |
| Edit capability | None | Full CRUD on nodes/edges before execution |
| Multi-tenant | No | Yes (workspace-scoped) |
| MCP integration | `connectMCPTools()` | `agentra_graph_decompose` tool |

### 9.3 What Agentra Adds

open-multi-agent proves the core pattern works: one LLM call can produce a useful task DAG. Agentra's advantage is making that DAG **persistent, reviewable, editable, and integrated with a full task management platform**. The generated graph is not a throwaway artifact -- it becomes part of the issue's permanent record, traceable and auditable.

---

## 10. Edge Cases & Error Handling

| Scenario | Behavior |
|---|---|
| Issue has no description | Return 400: "Issue has no description. Add a goal description first." |
| Issue already has a graph | Return 409 with existing graph data. Frontend shows "Graph already exists. Regenerate?" confirmation. |
| LLM returns invalid JSON | Return 422 with raw output (truncated). Frontend shows retry button. |
| LLM returns valid JSON but no nodes | Return 422: "Planner returned an empty graph. Try a more specific goal description." |
| LLM times out | Return 500. Frontend shows "Planning took too long. Try reducing max_nodes or using a faster model." |
| Agent name match fails | Node created with NULL agent_id. User reassigns during review. |
| Provider not configured | Return 500 with guidance to configure API keys in workspace settings. |
| Rate limiting | Return 429. Frontend shows backoff timer. |
| Workspace has zero agents | Return 400: "No agents available. Create at least one agent first." |

---

## 11. Security Considerations

- The planner prompt is built server-side. User-controlled fields (`additional_context`, `issue.description`) are injected into the prompt but are not used to override the system prompt.
- Provider API keys are resolved from the workspace configuration (encrypted storage pattern from existing `APIConfig`).
- The endpoint uses existing workspace membership middleware -- only workspace members can trigger decomposition.
- Generated node `context` fields are stored as JSONB. No raw LLM output is stored outside the graph node context.
- The planner is called with `MaxTurns: 1` -- it cannot invoke tools, access the filesystem, or make network calls.
