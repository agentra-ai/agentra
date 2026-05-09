# Agent-to-Agent Handoff 设计规格

**日期**: 2026-05-09
**状态**: Approved
**目标**: 构建 Task Graph 系统，支持多 agent 任务分解、DAG 依赖执行、和 agent-to-agent handoff

---

## 1. 概述

### 1.1 目标

Task Graph 系统让复杂任务可以被 Planner agent 自动分解为子任务，由不同的 specialist agents 并行或顺序执行，通过 handoff protocol 传递 context 和 artifacts。

### 1.2 关键设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 触发方式 | Both | Planner 自动拆解 + 人类手动分配 |
| Handoff context | Full context | context + results + artifacts |
| 执行模型 | DAG with dependencies | 无依赖并行，有依赖顺序执行 |
| UI | Subtask tree + DAG view | 列表 + 可视化 |
| 架构 | Task Graph tables | 独立 node/edge 表，灵活支持复杂 DAG |

---

## 2. 架构

### 2.1 系统架构

```
┌──────────────────────────────────────────────────────────┐
│  Planner Agent (Claude Code)                              │
│  └─ 分析 issue → 生成 task graph → 分配 specialist agents │
└─────────────────────┬────────────────────────────────────┘
                      │ CreateTaskGraph API
                      ▼
┌──────────────────────────────────────────────────────────┐
│  Task Graph Engine                                        │
│  ├── GraphScheduler    # 管理节点状态转换和依赖检查        │
│  ├── HandoffProtocol   # 构造和传递 handoff context        │
│  └── GraphStore        # node/edge CRUD                   │
└─────────────────────┬────────────────────────────────────┘
                      │
                      ▼
┌──────────────────────────────────────────────────────────┐
│  Agent Task Queue (existing)                              │
│  └─ 每个 executable node 创建一个 task                    │
└──────────────────────────────────────────────────────────┘
```

### 2.2 目录结构

```
server/
├── pkg/
│   └── taskgraph/
│       ├── go.mod
│       ├── types.go           # GraphNode, GraphEdge types
│       ├── store.go           # GraphStore: node/edge CRUD
│       ├── scheduler.go       # GraphScheduler: 状态转换
│       └── handoff.go         # HandoffProtocol: context 构造

server/migrations/
├── 034_task_graph.up.sql      # task_graph_nodes + task_graph_edges
└── 034_task_graph.down.sql

server/pkg/db/queries/
├── taskgraph.sql               # sqlc queries for node/edge

server/internal/handler/
├── taskgraph.go                # REST API handlers

apps/web/features/
├── taskgraph/
│   ├── components/
│   │   ├── SubtaskTree.tsx      # Issue 页面上的子任务树
│   │   ├── GraphView.tsx        # DAG 可视化 tab
│   │   └── NodeCard.tsx         # 节点卡片
│   ├── hooks/
│   │   └── useTaskGraph.ts
│   └── api/
│       └── taskGraphApi.ts
```

---

## 3. 数据库设计

### 3.1 task_graph_nodes 表

```sql
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
```

### 3.2 task_graph_edges 表

```sql
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

---

## 4. 核心组件

### 4.1 GraphScheduler

```go
type GraphScheduler struct {
    store *GraphStore
}

// GetReadyNodes 返回所有依赖已满足的 pending 节点
func (s *GraphScheduler) GetReadyNodes(ctx context.Context, graphID string) ([]GraphNode, error)

// TransitionNode 原子转换节点状态
func (s *GraphScheduler) TransitionNode(ctx context.Context, nodeID string, status string) error

// IsGraphComplete 检查所有节点是否完成
func (s *GraphScheduler) IsGraphComplete(ctx context.Context, graphID string) (bool, error)
```

### 4.2 HandoffProtocol

```go
type HandoffProtocol struct {
    store       *GraphStore
    memorySvc   *memory.MemoryService
}

// BuildHandoffContext 为指定节点构造 handoff context
func (h *HandoffProtocol) BuildHandoffContext(ctx context.Context, nodeID string) (map[string]any, error)
// → 包含: parent issue info, completed sibling results, relevant memories, artifacts
```

### 4.3 Handoff Context Structure

```json
{
  "parent_issue": {
    "id": "uuid",
    "title": "Build API + Frontend for auth",
    "description": "..."
  },
  "completed_siblings": [
    {
      "node_id": "uuid",
      "agent_name": "backend-agent",
      "node_type": "executor",
      "result": { "summary": "API endpoints built", "files": ["api/auth.go"] }
    }
  ],
  "relevant_memories": [
    { "type": "learning", "content": "bcrypt rounds should be 12+" }
  ],
  "artifacts": [
    { "type": "file", "path": "/workspace/api/auth.go" },
    { "type": "memory", "id": "uuid" }
  ],
  "instructions": "Frontend component should connect to /api/auth endpoint built by backend-agent"
}
```

---

## 5. API 端点

| Method | Path | 描述 |
|--------|------|------|
| POST | /api/issues/:id/graph | 创建 task graph (Planner 调用) |
| GET | /api/issues/:id/graph | 获取 issue 的完整 graph |
| PATCH | /api/graph/nodes/:id | 更新节点 (状态, agent, context) |
| DELETE | /api/graph/nodes/:id | 删除节点 |
| POST | /api/graph/edges | 添加边 |

---

## 6. UI 设计

### 6.1 Subtask Tree (Issue 页面)

- 在 issue 详情页显示 child nodes 列表
- 展开/收起、drag 排序
- 显示每个 node 的 type badge, status, assigned agent
- "+" 按钮添加手动子任务

### 6.2 Graph View (独立 Tab)

- DAG 可视化，使用 react-flow 或 dagre
- 节点颜色按类型: root=gray, planner=blue, executor=green, synthesis=yellow
- 边的类型: depends_on=实线, handoff=虚线, triggers=点线
- 点击节点显示详情 panel

### 6.3 组件

```
apps/web/features/taskgraph/components/
├── SubtaskTree.tsx      # 子任务树
├── GraphView.tsx        # DAG 可视化 tab
└── NodeCard.tsx         # 节点详情卡片
```

---

## 7. MCP Tools (后续扩展)

| Tool | 描述 |
|------|------|
| agentra_graph_create | 创建 task graph |
| agentra_graph_get | 获取 graph 状态 |
| agentra_node_update | 更新节点状态和结果 |
| agentra_handoff_context | 获取 handoff context |

---

## 8. 测试策略

- 单元测试: store, scheduler, handoff protocol
- 集成测试: 端到端 DAG 执行 (planner → executors → synthesis)

---

## 9. 未来扩展

| 功能 | 描述 |
|------|------|
| Graph templates | 预设常用 task graph 模板 |
| Real-time graph updates | WebSocket 推送 graph 状态变化 |
| Execution replay | 回放整个 graph 执行过程 |