# External MCP Registry 设计规格

**日期**: 2026-05-10
**基于**: 竞品分析 v2 (swarmclaw, open-multi-agent, agent-tasks)
**状态**: Draft

---

## 1. 概述

允许 workspace admin 注册外部 MCP servers (GitHub, Slack, Jira, web search 等)，发现其 tools，并按 agent 分配 tools。Tool 调用记录到审计日志。

### 1.1 关键设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 配置存储 | 独立表 (mcp_server) + JSONB 灵活字段 | 足够复杂，遵循 GitHub integration 模式 |
| Tool 发现 | 注册时调用 initialize + tools/list，定期刷新 | swarmclaw/open-multi-agent 模式 |
| 连接管理 | Lazy (首次使用或 Test 按钮)，不保持长连接 | 避免 stdio 进程泄漏 |
| Secret 加密 | 复用 crypto.EncryptAPIKey | 已有基础设施 |
| 权限模型 | 2 层: workspace (注册) + agent (启用 tools) | 简单有效 |

---

## 2. 数据库设计

```sql
-- 038_external_mcp_servers.up.sql

CREATE TABLE mcp_server (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    display_name TEXT,
    description TEXT,
    transport TEXT NOT NULL CHECK (transport IN ('stdio', 'streamable-http')),
    -- stdio
    command TEXT,
    args JSONB DEFAULT '[]',
    env_vars JSONB DEFAULT '{}',
    cwd TEXT,
    -- streamable-http
    url TEXT,
    headers JSONB DEFAULT '{}',
    -- common
    timeout_ms INTEGER NOT NULL DEFAULT 30000,
    enabled BOOLEAN NOT NULL DEFAULT true,
    connected_at TIMESTAMPTZ,
    last_checked_at TIMESTAMPTZ,
    server_info JSONB DEFAULT '{}',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(workspace_id, name)
);

CREATE TABLE mcp_server_tool (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mcp_server_id UUID NOT NULL REFERENCES mcp_server(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    input_schema JSONB NOT NULL DEFAULT '{}',
    discovered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(mcp_server_id, name)
);

CREATE TABLE agent_mcp_tool (
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    mcp_server_tool_id UUID NOT NULL REFERENCES mcp_server_tool(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT true,
    PRIMARY KEY (agent_id, mcp_server_tool_id)
);

CREATE TABLE mcp_tool_call_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
    task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    mcp_server_id UUID REFERENCES mcp_server(id) ON DELETE SET NULL,
    tool_name TEXT NOT NULL,
    input_json JSONB,
    output_json JSONB,
    error_message TEXT,
    duration_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_mcp_server_workspace ON mcp_server(workspace_id);
CREATE INDEX idx_mcp_server_tool_server ON mcp_server_tool(mcp_server_id);
CREATE INDEX idx_agent_mcp_tool_agent ON agent_mcp_tool(agent_id);
CREATE INDEX idx_mcp_tool_call_log_task ON mcp_tool_call_log(task_id);
CREATE INDEX idx_mcp_tool_call_log_agent ON mcp_tool_call_log(agent_id);
```

---

## 3. API Endpoints

| Method | Path | 描述 |
|--------|------|------|
| GET | /api/workspaces/{id}/mcp-servers | List MCP servers |
| POST | /api/workspaces/{id}/mcp-servers | Register new server |
| GET | /api/workspaces/{id}/mcp-servers/{sid} | Get server + tools |
| PATCH | /api/workspaces/{id}/mcp-servers/{sid} | Update config |
| DELETE | /api/workspaces/{id}/mcp-servers/{sid} | Remove |
| POST | /api/workspaces/{id}/mcp-servers/{sid}/test | Test connection |
| POST | /api/workspaces/{id}/mcp-servers/{sid}/refresh | Re-discover tools |
| PUT | /api/agents/{aid}/mcp-tools | Set enabled tools |
| GET | /api/agents/{aid}/mcp-tools | List enabled tools |
| GET | /api/workspaces/{id}/mcp-tool-calls | Audit log |

---

## 4. Go 实现

### 4.1 MCP Client (`server/pkg/mcp/client/`)

```go
type ClientConfig struct {
    Transport string // "stdio" or "streamable-http"
    Command   string
    Args      []string
    Env       map[string]string
    URL       string
    Headers   map[string]string
    Timeout   time.Duration
}

type Client struct { ... }

func (c *Client) Connect(ctx context.Context) (*ServerInfo, error)
func (c *Client) ListTools(ctx context.Context) ([]Tool, error)
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error)
func (c *Client) Close() error
```

### 4.2 Handler + Service

- `server/internal/handler/mcp_registry.go` — HTTP handlers
- `server/internal/service/mcp_registry.go` — Business logic

### 4.3 Tool Relay

TaskService 在 dispatch task 时合并 tools:
1. Agentra 内置 MCP tools (issues, skills, memory)
2. `agent_mcp_tool.enabled = true` 的外部 tools

---

## 5. UI 组件

| 组件 | 位置 |
|------|------|
| McpServerCard | features/mcp/components/ |
| McpServerForm | features/mcp/components/ |
| AgentMcpToolSelector | features/mcp/components/ |
| mcp-tab (Settings) | app/(dashboard)/settings/_components/ |
| useMcpRegistry | features/mcp/hooks/ |
| mcpApi | features/mcp/api/ |

---

## 6. 实现优先级

1. 数据库 migration + sqlc 查询
2. MCP Client (stdio + streamable-http)
3. API handlers + service
4. UI (settings tab + agent tool selector)
5. Tool relay in TaskService

---

## 7. 参考资料

- [swarmclaw MCP gateway](https://github.com/swarmclawai/swarmclaw)
- [open-multi-agent connectMCPTools](https://github.com/open-multi-agent/open-multi-agent)
- [MCP SDK v1.27.1](https://github.com/modelcontextprotocol/sdk)
- [竞争分析 v2](2026-05-10-competitive-analysis-v2.md)
