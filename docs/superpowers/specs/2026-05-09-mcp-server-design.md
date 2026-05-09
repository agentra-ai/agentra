# MCP Server 设计规格

**日期**: 2026-05-09
**状态**: Approved
**目标**: 将 Agentra 暴露为 MCP Server，让 AI agents 通过 MCP protocol 访问 issues, skills, agents, comments, inbox 等工具

---

## 1. 概述

### 1.1 目标

构建独立的 `agentra-mcp` 进程，通过 MCP 2.0 协议（JSON-RPC 2.0 over stdio）暴露 Agentra 的完整功能。AI agents（Claude Code, Cursor, etc.）可以像连接其他 MCP servers 一样连接 Agentra。

### 1.2 关键设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 部署形态 | 独立进程 (stdio) | MCP 标准模式，minimal footprint |
| 认证 | API Key | 每个 workspace 生成 API Key，简单直接 |
| 工具范围 | 完整暴露 | issues, skills, agents, comments, inbox, memory, prompts |
| 命名空间 | 扁平 flat | agentra_* 前缀，简单直接 |
| 实现方式 | 自定义 MCP 2.0 | 无外部依赖，完全控制 |
| 代码位置 | server/pkg/mcp | 与 agent SDK 并列，清晰分离 |
| 数据库 | 直接连接 PostgreSQL | 简单直接 |

### 1.3 非目标

- 不实现 WebSocket/SSE transport（MCP over stdio 是唯一模式）
- 不实现 streaming tools（future work）
- 不实现 tistory（完整生命周期见未来规划）

---

## 2. 架构

### 2.1 系统架构

```
┌─────────────────────────────────────────────────────────┐
│  AI Agent (Claude Code, Cursor, etc.)                   │
│  MCP Client                                             │
└─────────────────┬───────────────────────────────────────┘
                  │ stdio (JSON-RPC 2.0)
                  ▼
┌─────────────────────────────────────────────────────────┐
│  agentra-mcp (独立进程)                                 │
│  ├── cmd/mcp/main.go                                   │
│  └── pkg/mcp/                                          │
│       ├── server.go        # MCP 2.0 协议引擎           │
│       ├── transport.go     # stdio transport            │
│       ├── auth.go          # API Key 验证               │
│       ├── tools.go         # 工具注册表                 │
│       ├── resources.go     # Resource 注册表            │
│       ├── prompts.go       # Prompt 注册表              │
│       └── tools/                                        │
│            ├── issues.go   # agentra_issue_*            │
│            ├── skills.go  # agentra_skill_*            │
│            ├── agents.go  # agentra_agent_*            │
│            ├── comments.go # agentra_comment_*          │
│            ├── inbox.go   # agentra_inbox_*            │
│            └── memory.go  # agentra_memory_* (预留)     │
└─────────────────┬───────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────┐
│  PostgreSQL (直接连接)                                   │
│  └── Agentra DB (issues, skills, agents, etc.)          │
└─────────────────────────────────────────────────────────┘
```

### 2.2 目录结构

```
server/
├── cmd/
│   └── mcp/
│       └── main.go              # MCP server entry point
└── pkg/
    └── mcp/
        ├── go.mod               # 独立 module
        ├── server.go            # MCP 协议引擎
        ├── transport.go         # stdio transport 实现
        ├── auth.go              # API Key 验证
        ├── types.go             # 共享类型
        ├── tools.go             # 工具注册表
        ├── resources.go         # Resource 注册表
        ├── prompts.go           # Prompt 注册表
        ├── errors.go            # 错误类型定义
        ├── tools/
        │   ├── issues.go        # agentra_issue_* 工具
        │   ├── skills.go        # agentra_skill_* 工具
        │   ├── agents.go        # agentra_agent_* 工具
        │   ├── comments.go      # agentra_comment_* 工具
        │   ├── inbox.go         # agentra_inbox_* 工具
        │   └── memory.go        # agentra_memory_* (预留)
        └── vendor/              # 必要 DB queries 副本
```

---

## 3. MCP 协议实现

### 3.1 支持的协议能力

| 能力 | 状态 | 方法 |
|------|------|------|
| Tools | ✅ | `tools/list`, `tools/call` |
| Resources | ✅ | `resources/list`, `resources/read` |
| Prompts | ✅ | `prompts/list`, `prompts/get`, `prompts/call` |
| Lifecycle | ✅ | `initialize`, `initialized`, `exit` |

### 3.2 Lifecycle 流程

```
1. stdin: ← { method: "initialize", params: { protocolVersion, clientInfo, capabilities } }
2. stdout: → { method: "initialized", params: { protocolVersion, serverInfo, capabilities } }
3...: ←→ 工具调用 (tools/call, resources/read, prompts/call)
4. stdin: ← { method: "exit" }
```

### 3.3 Transport 实现

stdio transport 使用标准输入/输出：

```go
// 输入: 逐行读取 JSON-RPC 请求
scanner := bufio.NewScanner(os.Stdin)
for scanner.Scan() {
    line := scanner.Text()
    if line == "" { continue }
    server.handleJSON(line)
}

// 输出: 逐行写入 JSON-RPC 响应
encoder := json.NewEncoder(os.Stdout)
encoder.Encode(response)
```

---

## 4. 工具定义

### 4.1 Issues 工具

#### agentra_issue_list

列出 workspace 中的 issues。

**参数**:
```json
{
  "workspace_id": "uuid (required)",
  "status": "open|in_progress|done (optional)",
  "priority": "low|medium|high|urgent (optional)",
  "assignee_id": "uuid (optional)",
  "limit": 50,
  "offset": 0
}
```

**返回值**:
```json
{
  "issues": [
    {
      "id": "uuid",
      "title": "string",
      "description": "string",
      "status": "open|in_progress|done",
      "priority": "low|medium|high|urgent",
      "assignee_id": "uuid|null",
      "assignee_type": "member|agent",
      "created_at": "ISO8601",
      "updated_at": "ISO8601"
    }
  ],
  "total": 100
}
```

#### agentra_issue_get

获取单个 issue 详情。

**参数**:
```json
{
  "issue_id": "uuid (required)"
}
```

**返回值**:
```json
{
  "id": "uuid",
  "workspace_id": "uuid",
  "title": "string",
  "description": "string",
  "status": "open|in_progress|done",
  "priority": "low|medium|high|urgent",
  "assignee_id": "uuid|null",
  "assignee_type": "member|agent",
  "created_by": "uuid",
  "created_at": "ISO8601",
  "updated_at": "ISO8601"
}
```

#### agentra_issue_create

创建新 issue。

**参数**:
```json
{
  "workspace_id": "uuid (required)",
  "title": "string (required)",
  "description": "string (optional)",
  "status": "open (default)",
  "priority": "medium (default)",
  "assignee_id": "uuid (optional)",
  "assignee_type": "member|agent (optional)"
}
```

**返回值**: 创建的 issue 对象

#### agentra_issue_update

更新 issue。

**参数**:
```json
{
  "issue_id": "uuid (required)",
  "title": "string (optional)",
  "description": "string (optional)",
  "status": "open|in_progress|done (optional)",
  "priority": "low|medium|high|urgent (optional)",
  "assignee_id": "uuid (optional)",
  "assignee_type": "member|agent (optional)"
}
```

**返回值**: 更新后的 issue 对象

#### agentra_issue_delete

删除 issue。

**参数**:
```json
{
  "issue_id": "uuid (required)"
}
```

**返回值**: `{ "deleted": true }`

### 4.2 Skills 工具

#### agentra_skill_list

列出 workspace 中的 skills。

**参数**:
```json
{
  "workspace_id": "uuid (required)"
}
```

**返回值**:
```json
{
  "skills": [
    {
      "id": "uuid",
      "name": "string",
      "description": "string",
      "content": "string",
      "config": {},
      "created_at": "ISO8601"
    }
  ]
}
```

#### agentra_skill_get

获取 skill 详情。

**参数**:
```json
{
  "skill_id": "uuid (required)"
}
```

#### agentra_skill_apply

将 skill 应用到 issue。

**参数**:
```json
{
  "skill_id": "uuid (required)",
  "issue_id": "uuid (required)"
}
```

**返回值**:
```json
{
  "applied": true,
  "issue_id": "uuid",
  "skill_id": "uuid"
}
```

### 4.3 Agents 工具

#### agentra_agent_list

列出 workspace 中的 agents。

**参数**:
```json
{
  "workspace_id": "uuid (required)"
}
```

**返回值**:
```json
{
  "agents": [
    {
      "id": "uuid",
      "name": "string",
      "provider": "claude|codex|opencode",
      "status": "online|offline|busy",
      "runtime_id": "uuid|null"
    }
  ]
}
```

#### agentra_agent_get

获取 agent 详情和状态。

**参数**:
```json
{
  "agent_id": "uuid (required)"
}
```

### 4.4 Comments 工具

#### agentra_comment_list

列出 issue 的 comments。

**参数**:
```json
{
  "issue_id": "uuid (required)"
}
```

#### agentra_comment_create

创建 comment。

**参数**:
```json
{
  "issue_id": "uuid (required)",
  "content": "string (required)"
}
```

### 4.5 Inbox 工具

#### agentra_inbox_list

列出当前用户的通知。

**参数**:
```json
{
  "user_id": "uuid (required)",
  "read": true|false|null
}
```

#### agentra_inbox_mark_read

标记通知为已读。

**参数**:
```json
{
  "notification_id": "uuid (required)"
}
```

### 4.6 Prompts 工具

#### agentra_prompt_workspace_context

获取 workspace context prompt。

**参数**:
```json
{
  "workspace_id": "uuid (required)"
}
```

**返回值**: 包含 workspace 信息的 prompt 模板

---

## 5. Resources 定义

### 5.1 Resource URI 格式

| Resource | URI 格式 | 描述 |
|----------|----------|------|
| Workspace | `agentra://workspaces/{id}` | workspace 信息 |
| Issue | `agentra://issues/{id}` | issue 详情 |
| Agent | `agentra://agents/{id}` | agent 信息 |

### 5.2 Resource 示例

```
agentra://workspaces/550e8400-e29b-41d4-a716-446655440000
agentra://issues/123e4567-e89b-12d3-a456-426614174000
agentra://agents/789e0123-e45b-67d8-a901-234567890000
```

---

## 6. 错误处理

### 6.1 错误响应格式

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "data": {}
  }
}
```

### 6.2 错误码

| 错误码 | HTTP 类比 | 描述 |
|--------|----------|------|
| `UNAUTHORIZED` | 401 | 无效或缺失 API Key |
| `FORBIDDEN` | 403 | 无权限访问资源 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `VALIDATION_ERROR` | 400 | 参数验证失败 |
| `INTERNAL_ERROR` | 500 | 服务器内部错误 |
| `TIMEOUT` | 504 | 工具执行超时 (60s) |

### 6.3 超时策略

- 每个工具调用有 60s 超时
- 超时后返回 `TIMEOUT` 错误
- 与 Agentra Task 超时一致

---

## 7. 认证

### 7.1 API Key 格式

```
agentra_api_{workspace_id}_{random_32_chars}
```

### 7.2 认证流程

1. MCP client 在 `initialize` 请求中携带 `Authorization: Bearer {api_key}` header
2. Server 解析 API Key，提取 workspace_id
3. 验证 API Key 有效性（数据库查询）
4. 所有后续请求都携带 API Key

### 7.3 权限模型

- MCP server 只能访问 API Key 对应 workspace 的数据
- 不支持跨 workspace 访问

---

## 8. 日志和可观测性

### 8.1 日志格式

使用 slog JSON 格式输出到 stderr：

```json
{
  "time": "2026-05-09T10:00:00Z",
  "level": "INFO",
  "msg": "tool call",
  "tool": "agentra_issue_list",
  "workspace_id": "xxx",
  "duration_ms": 45
}
```

### 8.2 日志级别

通过 `LOG_LEVEL` 环境变量控制：
- `debug`: 详细调试信息
- `info`: 一般信息 (默认)
- `warn`: 警告
- `error`: 错误

---

## 9. 配置

### 9.1 环境变量

| 变量 | 必填 | 描述 |
|------|------|------|
| `DATABASE_URL` | Yes | PostgreSQL 连接字符串 |
| `LOG_LEVEL` | No | 日志级别 (默认 info) |
| `MCP_TIMEOUT` | No | 工具调用超时秒数 (默认 60) |

### 9.2 启动示例

```bash
DATABASE_URL=postgres://user:pass@localhost:5432/agentra \
LOG_LEVEL=debug \
agentra-mcp
```

---

## 10. 测试策略

### 10.1 单元测试

- `tools_test.go`: 每个工具的参数验证和错误处理
- `auth_test.go`: API Key 验证逻辑
- `transport_test.go`: JSON-RPC 序列化/反序列化

### 10.2 集成测试

- 启动真实 PostgreSQL (testcontainers)
- 调用每个工具验证端到端功能

### 10.3 MCP Compliance 测试

- 使用官方 MCP test suite 验证协议合规性

---

## 11. 依赖

### 11.1 Go 模块

```go
module github.com/agentra-ai/agentra/pkg/mcp

go 1.26

require (
    github.com/jackc/pgx/v5 v5.6.0
    github.com/google/uuid v1.6.0
)
```

### 11.2 外部依赖

- PostgreSQL (已有)
- 无新增外部运行时依赖

---

## 12. 部署

### 12.1 构建

```bash
cd server/pkg/mcp
go build -o agentra-mcp ./cmd/mcp
```

### 12.2 MCP Client 配置

#### Claude Code

```json
{
  "mcpServers": {
    "agentra": {
      "command": "agentra-mcp",
      "env": {
        "DATABASE_URL": "postgres://..."
      }
    }
  }
}
```

#### Cursor

```json
{
  "mcpServers": {
    "agentra": {
      "command": "agentra-mcp",
      "args": [],
      "env": {
        "DATABASE_URL": "postgres://..."
      }
    }
  }
}
```

---

## 13. 未来扩展

### 13.1 v2 功能

| 功能 | 描述 |
|------|------|
| Streaming tools | 支持长时间 tool 调用的 streaming 输出 |
| Memory tools | agentra_memory_* 工具，RAG 检索 |
| Event subscriptions | 实时事件推送 |
| Batch operations | 批量创建/更新 |

### 13.2 已知限制

- 不支持 WebSocket/SSE transport
- 不支持 multi-step transactions
- 不支持 workspace 间数据访问

---

## 14. 参考资料

- [MCP 2.0 协议规范](https://modelcontextprotocol.io)
- [Overseer MCP Server](https://github.com/dmmulroy/overseer)
- [Agentra Skills System](../agent-workflow-analysis.md)
