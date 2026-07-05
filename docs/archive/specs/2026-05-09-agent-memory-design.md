# Agent Memory (RAG) 设计规格

**日期**: 2026-05-09
**状态**: Approved
**目标**: 为每个 agent 提供持久化记忆存储，支持 RAG 自动注入，让 agent 在任务开始时能获取相关历史记忆

---

## 1. 概述

### 1.1 目标

构建基于 pgvector 的 agent 记忆系统，支持：
- Per-agent 私有记忆 + workspace 共享记忆
- RAG 自动注入：任务开始时自动检索相关记忆
- 显式存储：agent 可通过 tool 调用保存重要信息
- Memory viewer UI：用户可浏览、搜索、编辑记忆

### 1.2 关键设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 存储 | pgvector (PostgreSQL) | 已在使用，简化部署 |
| Embedding | OpenAI text-embedding-3-small | 高质量，成本低 |
| 作用域 | Hybrid (per-agent + shared) | 平衡隐私和协作 |
| 内容类型 | 全支持 | learnings, task results, context, patterns |
| 捕获方式 | Hybrid | 自动 + 显式结合 |
| 检索方式 | Both | 自动注入 + on-demand tool |

---

## 2. 架构

### 2.1 系统架构

```
┌─────────────────────────────────────────────────────────┐
│  AI Agent (Claude Code, Codex, etc.)                     │
│  ├── Automatic: Task start → RAG inject memories         │
│  └── Explicit: agentra_memory_store/recall tools        │
└─────────────────┬───────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────┐
│  Agentra Backend                                        │
│  ├── MemoryService       # store/recall/search          │
│  ├── TaskCompletionHook  # auto-extract learnings        │
│  └── TaskStartHook       # auto-inject relevant memories │
└─────────────────┬───────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────┐
│  PostgreSQL (pgvector)                                   │
│  ├── agent_memories    # per-agent private memories      │
│  └── team_memory       # workspace-shared memories       │
└─────────────────────────────────────────────────────────┘
```

### 2.2 目录结构

```
server/
├── pkg/
│   └── memory/
│       ├── go.mod
│       ├── service.go         # MemoryService
│       ├── embedding.go       # OpenAI embedding client
│       ├── search.go          # pgvector similarity search
│       └── hooks/
│           ├── task_completion.go  # auto-extract on task done
│           └── task_start.go       # auto-inject on task start
│
migrations/
├── 032_agent_memory.up.sql    # agent_memories table
└── 033_team_memory.up.sql     # team_memory table

apps/web/
├── features/
│   └── memory/
│       ├── components/        # MemoryViewer, MemoryEditor
│       ├── hooks/             # useMemoryStore
│       └── api/               # memory API client
```

---

## 3. 数据库设计

### 3.1 agent_memories 表

```sql
CREATE TABLE agent_memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    memory_type TEXT NOT NULL CHECK (memory_type IN ('learning', 'task_result', 'context', 'pattern')),
    content TEXT NOT NULL,
    embedding vector(1536),  -- text-embedding-3-small dimension
    metadata JSONB DEFAULT '{}',
    is_private BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX agent_memories_agent_id_idx ON agent_memories(agent_id);
CREATE INDEX agent_memories_workspace_id_idx ON agent_memories(workspace_id);
CREATE INDEX agent_memories_embedding_idx ON agent_memories USING ivfflat (embedding vector_cosine_ops);
```

### 3.2 team_memory 表

```sql
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
CREATE INDEX team_memory_embedding_idx ON team_memory USING ivfflat (embedding vector_cosine_ops);
```

---

## 4. 功能定义

### 4.1 MCP Tools

#### agentra_memory_store

Agent 显式保存记忆。

**参数**:
```json
{
  "workspace_id": "uuid (required)",
  "agent_id": "uuid (required)",
  "memory_type": "learning|task_result|context|pattern (required)",
  "content": "string (required)",
  "is_private": true
}
```

**返回值**:
```json
{
  "id": "uuid",
  "memory_type": "learning",
  "content": "string",
  "created_at": "ISO8601"
}
```

#### agentra_memory_recall

基于当前 context 检索相关记忆。

**参数**:
```json
{
  "agent_id": "uuid (required)",
  "workspace_id": "uuid (required)",
  "query": "string (required)",
  "limit": 5,
  "memory_types": ["learning", "context"] // optional filter
}
```

**返回值**:
```json
{
  "memories": [
    {
      "id": "uuid",
      "memory_type": "learning",
      "content": "string",
      "score": 0.85,
      "agent_id": "uuid|null",
      "created_at": "ISO8601"
    }
  ]
}
```

#### agentra_memory_search

全站搜索记忆（workspace 内）。

**参数**:
```json
{
  "workspace_id": "uuid (required)",
  "query": "string (required)",
  "include_team": true,
  "limit": 20
}
```

### 4.2 自动捕获 (Task Completion Hook)

任务完成时自动提取关键信息：

1. 从 task result 中提取最终的解决方案
2. 从操作过程中提取关键的 learnings
3. 保存为对应类型的 memory entry

触发时机：`TaskService.Complete()` 时

### 4.3 自动注入 (Task Start Hook)

任务开始时自动注入相关记忆：

1. 构建 query: issue title + description + skill instructions
2. 检索 top-k 相关记忆（per-agent + team）
3. 注入到 agent system prompt 的 memory context 段

注入格式：
```
=== Relevant Memories ===
- [learning] bcrypt rounds should be 12+ (from task #123, 2026-05-01)
- [pattern] Always validate input before DB query (from agent:claude-1)
===
```

---

## 5. Embedding 服务

### 5.1 OpenAI Embeddings

```go
type EmbeddingClient struct {
    apiKey string
    model  string // "text-embedding-3-small"
}

func (c *EmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error)
```

环境变量：`OPENAI_API_KEY`

### 5.2 向量维度

`text-embedding-3-small`: 1536 dimensions

---

## 6. Memory Viewer UI

### 6.1 位置

Workspace Settings → Memory tab

### 6.2 功能

- **Browse**: 按 agent、type、date 筛选记忆
- **Search**: 全文 + 向量混合搜索
- **Edit**: 修改 content 和 type
- **Delete**: 删除单条记忆
- **Stats**: 记忆数量、类型分布、最近活跃

### 6.3 组件

```
apps/web/features/memory/
├── components/
│   ├── MemoryViewer.tsx      # 主视图
│   ├── MemoryList.tsx        # 记忆列表
│   ├── MemoryItem.tsx       # 单条记忆
│   ├── MemoryEditor.tsx      # 编辑弹窗
│   └── MemorySearch.tsx      # 搜索栏
├── hooks/
│   └── useMemoryStore.ts    # Zustand store
└── api/
    └── memoryApi.ts         # API client
```

---

## 7. API 端点

| Method | Path | 描述 |
|--------|------|------|
| GET | /api/workspaces/:id/memories | 列出 workspace 记忆 |
| POST | /api/workspaces/:id/memories | 创建记忆 |
| GET | /api/agents/:id/memories | 列出 agent 私有记忆 |
| PATCH | /api/memories/:id | 更新记忆 |
| DELETE | /api/memories/:id | 删除记忆 |
| GET | /api/memories/search | 搜索记忆 |

---

## 8. 配置

| 环境变量 | 必填 | 描述 |
|----------|------|------|
| `OPENAI_API_KEY` | Yes | OpenAI API key for embeddings |
| `EMBEDDING_MODEL` | No | 模型名称 (默认 text-embedding-3-small) |
| `MEMORY_INJECT_LIMIT` | No | 任务开始时注入的记忆条数 (默认 5) |

---

## 9. 依赖

```go
module github.com/agentra-ai/agentra/pkg/memory

go 1.26

require (
    github.com/jackc/pgx/v5 v5.6.0
    github.com/google/uuid v1.6.0
    github.com/sashabaranov/go-openai v1.35.0
)
```

---

## 10. 测试策略

### 10.1 单元测试

- `service_test.go`: store/recall/search 逻辑
- `embedding_test.go`: embedding client mock

### 10.2 集成测试

- 使用真实 pgvector (testcontainers)
- 验证向量检索准确性

---

## 11. 未来扩展

| 功能 | 描述 |
|------|------|
| Memory syntheses | 自动合并多个 learnings 为 team conventions |
| Memory versioning | 记忆编辑历史 |
| Memory permissions | 细粒度访问控制 |