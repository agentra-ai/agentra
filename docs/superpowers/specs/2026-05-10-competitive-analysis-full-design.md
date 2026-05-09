# 竞品全面对比分析报告

**日期**: 2026-05-10
**版本**: v0.3
**分析范围**: 技术架构、核心原理、实现方案、交互设计、产品定位

---

## 一、竞品全景图

### 1.1 核心竞品总览

| 项目 | Stars | 技术栈 | 定位 | 核心差异点 |
|------|-------|--------|------|-----------|
| **Agentra (本案)** | - | Go + Next.js | AI-native task management | 实时 WebSocket + 多 agent 后端 + 云运行时 |
| **swarmclaw** | 472 | Node.js (Electron + Next.js) | Self-hosted agent runtime + multi-agent | 23+ LLM providers + MCP native + 桌面 app |
| **hindsight** | 12,763 | Python/Go + PostgreSQL | Agent memory that learns | SOTA memory benchmark + biomimetic memory |
| **Overseer** | 223 | Rust + Node.js MCP | VCS-native task management | 原生 jj/git 集成 + learnings bubble |
| **Tasuku** | 63 | Go + Markdown | Git-friendly MCP tasks | per-file locking + 轻量 |
| **Beekeeper** | 52 | Python | Supervisor agent orchestration | 中央编排 + multi-agent |
| **Moo Tasks** | 32 | Nuxt4 + MySQL | Kanban + MCP server | Web UI + MCP 集成 |
| **dispatch** | 0 | - | MCP task server for Claude Code | minimal |

### 1.2 新发现竞品 (2026-05)

| 项目 | Stars | 关键特性 |
|------|-------|----------|
| [mem0-mcp](https://github.com/coleam00/mcp-mem0) | - | MCP server + long term agent memory |
| [agentmemo-mcp](https://github.com/andrewpetecoleman-cloud/agentmemo-mcp) | - | MCP + approval gateway |
| [AI-teams-controller](https://github.com/hungson175/AI-teams-controller-public) | 7 | 多 agent tmux teams + 记忆系统 |

---

## 二、技术架构深度对比

### 2.1 架构模式对比

```
┌─────────────────────────────────────────────────────────────────┐
│                        Agentra                                   │
│  ┌─────────┐    ┌──────────────┐    ┌────────────────────────┐  │
│  │ Next.js │◄──►│  Go Backend  │◄──►│  PostgreSQL + pgvector  │  │
│  │ Frontend│    │  (Chi+WS)    │    │                        │  │
│  └─────────┘    └──────┬───────┘    └────────────────────────┘  │
│                        │                                         │
│              ┌─────────┴─────────┐                               │
│              ▼                   ▼                               │
│        ┌──────────┐        ┌──────────┐                          │
│        │  Daemon  │        │  Cloud   │                          │
│        │ (Local)  │        │ Runtime  │                          │
│        └────┬─────┘        └────┬─────┘                          │
│             │  Claude/Codex/OpenCode CLI                          │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                        swarmclaw                                 │
│  ┌───────────────┐    ┌────────────────────────────────────┐    │
│  │ Electron App │◄──►│  Next.js (Turbopack) + Node.js      │    │
│  │  (Desktop)   │    │  - Provider health                  │    │
│  └───────────────┘    │  - MCP gateway runtime             │    │
│                       │  - Agent orchestration             │    │
│                       │  - Memory system                   │    │
│                       └──────────────────┬───────────────────┘    │
│                                          │                       │
│                       ┌──────────────────┴───────────────────┐  │
│                       │  Local storage + SQLite + File system │  │
│                       └────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                        hindsight                                 │
│  ┌─────────────┐    ┌─────────────────┐    ┌────────────────┐  │
│  │  Clients    │◄──►│  Hindsight API   │◄──►│  PostgreSQL     │  │
│  │ (Python/JS) │    │  (Go/Python)     │    │  + pgvector    │  │
│  └─────────────┘    └────────┬─────────┘    └────────────────┘  │
│                             │                                    │
│                    ┌────────▼─────────┐                        │
│                    │  Biomimetic Memory │                        │
│                    │  - World facts     │                        │
│                    │  - Experiences    │                        │
│                    │  - Mental models   │                        │
│                    └────────────────────┘                        │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 技术栈矩阵

| 维度 | Agentra | swarmclaw | hindsight | Overseer | Tasuku |
|------|---------|-----------|-----------|----------|--------|
| **前端** | Next.js 16 | Next.js + Electron | API-only | Web UI | TUI + CLI |
| **后端** | Go | Node.js | Go/Python | Rust | Go |
| **数据库** | PostgreSQL + pgvector | SQLite + 文件 | PostgreSQL + pgvector | SQLite | Markdown |
| **实时** | WebSocket | WebSocket | 轮询 | 轮询 | 轮询 |
| **Agent 接口** | CLI Backend | CLI + API + OpenClaw | API wrapper | MCP Server | MCP Server |
| **部署** | Docker | npm + Desktop app | Docker | npm | go install |

### 2.3 LLM Provider 支持

| Provider | Agentra | swarmclaw | hindsight |
|----------|---------|-----------|-----------|
| Claude Code | ✅ | ✅ | ❌ |
| Codex | ✅ | ✅ | ❌ |
| OpenCode | ✅ | ✅ | ❌ |
| Gemini CLI | ❌ | ✅ | ❌ |
| Copilot CLI | ❌ | ✅ | ❌ |
| Anthropic API | ❌ | ✅ | ✅ |
| OpenAI GPT | ❌ | ✅ | ✅ |
| Google Gemini | ❌ | ✅ | ✅ |
| OpenRouter | ❌ | ✅ | ❌ |
| Ollama | ❌ | ✅ | ✅ |
| DeepSeek | ❌ | ✅ | ❌ |
| Groq | ❌ | ✅ | ✅ |
| Together | ❌ | ✅ | ❌ |
| Mistral | ❌ | ✅ | ❌ |
| xAI | ❌ | ✅ | ❌ |
| Fireworks | ❌ | ✅ | ❌ |
| Nebius | ❌ | ✅ | ❌ |
| DeepInfra | ❌ | ✅ | ❌ |
| LM Studio | ❌ | ✅ | ✅ |
| **Total** | **3** | **23+** | **7** |

---

## 三、核心原理深度分析

### 3.1 多 agent 协调模式

| 模式 | 代表项目 | 架构特点 |
|------|----------|----------|
| **Daemon Poll** | Agentra | 中央轮询器 + 任务队列 + CLI backend |
| **Orchestrator** | swarmclaw | 母 agent 委派给 subagents |
| **VCS Native** | Overseer | 任务 start/complete 映射到 VCS bookmarks |
| **Supervisor** | Beekeeper | 中央 supervisor 分解任务给 operators |
| **Peer-to-Peer** | ChatDev | agents 通过文档传递信息 |
| **Graph-based** | Agentra (Task Graph) | DAG 执行 + 依赖检查 |

### 3.2 记忆系统架构

#### Agentra 的记忆系统 (设计中)

```
Task Start → RAG Query → Inject Relevant Memories
                ↓
         pgvector similarity search
                ↓
    per-agent private + workspace shared

Task Complete → Extract Learnings → Store Memories
```

#### swarmclaw 的记忆系统

```
- Hybrid recall (semantic + graph + keyword)
- Journaling + durable documents
- Project-scoped context
- Automatic reflection memory
- Communication preferences
- Profile and boundary memory
```

#### hindsight 的 biomimetic memory (SOTA)

```
Retain → LLM extraction → World facts / Experiences
                ↓
        Entity + Relationship + Time series
                ↓
Recall → 4 策略并行:
  - Semantic (vector similarity)
  - Keyword (BM25)
  - Graph (entity/temporal/causal)
  - Temporal (time range)
        ↓
  Reciprocal Rank Fusion + Cross-encoder reranking

Reflect → 生成新的 mental models 和 insights
```

### 3.3 MCP 集成对比

| 项目 | MCP 实现 | MCP Tools |
|------|----------|-----------|
| Agentra | pkg/mcp (stdio) | issues, skills, agents, comments, inbox, memory |
| swarmclaw | 原生 MCP gateway | 任何 MCP server 可连接 |
| Overseer | Node.js MCP server | tasks, execute (codemode) |
| Tasuku | Go MCP server | 40+ tools |
| Moo Tasks | MCP server | tasks |
| dispatch | MCP task server | create, get, claim, complete |

---

## 四、实现方案对比

### 4.1 任务生命周期

| 项目 | 任务状态 | 特点 |
|------|----------|------|
| Agentra | queued → claimed → started → completed/failed | 完整生命周期 + WebSocket 广播 |
| swarmclaw | pending → running → completed/failed + heartbeat | 持久化 + 心脏跳动 + 后台任务 |
| Overseer | start → working → complete | VCS bookmark 映射 |
| Tasuku | pending → in_progress → completed | per-file locking |
| hindsight | (无任务管理) | 专注记忆，不做任务管理 |

### 4.2 Agent 委派模式

```
Agentra:
  Issue → Daemon poll → Claude Code/Codex/OpenCode CLI → Execute

swarmclaw:
  Agent → delegate to Claude Code CLI
        → delegate to OpenCode CLI
        → delegate to native subagent

Overseer:
  MCP tool → codemode VM → execute
```

### 4.3 技能/记忆系统

| 项目 | 技能系统 | 记忆系统 |
|------|----------|----------|
| Agentra | SKILL.md 模板 + 应用到 issue | pgvector RAG (设计中) |
| swarmclaw | runtime skills + SKILL.md import + conversation-to-skill | hybrid recall + journaling + reflection |
| Overseer | learnings bubble | learnings 自动冒泡 |
| Tasuku | 轻量 learnings | markdown 文件存储 |
| hindsight | 无 | SOTA biomimetic memory |

---

## 五、交互设计对比

### 5.1 UI/UX 模式

| 项目 | 主要界面 | 交互特点 |
|------|----------|----------|
| Agentra | Next.js Web | Issue cards + WebSocket 实时 + 即将有 DAG view |
| swarmclaw | Electron Desktop + Web | Org chart + Agent chat + 桌面 app |
| Overseer | Web UI | minimal 单页 |
| Tasuku | TUI + CLI | terminal-first |
| hindsight | Web UI + API | API-first + 管理 UI |

### 5.2 实时性实现

| 项目 | 实时机制 | 恢复能力 |
|------|----------|----------|
| Agentra | WebSocket + Hub broadcast | 设计中 (reconnection backoff) |
| swarmclaw | WebSocket | 有心跳 |
| Overseer | 轮询 | 无 |
| Tasuku | 轮询 | 无 |

### 5.3 部署模式

| 项目 | 部署方式 | 门槛 |
|------|----------|------|
| Agentra | Docker Compose | 中等 (需要配置) |
| swarmclaw | npm global + Desktop app | 低 (一键安装) |
| hindsight | Docker | 低 (容器化) |
| Overseer | npm | 低 |
| Tasuku | go install | 最低 |

---

## 六、产品定位对比

### 6.1 目标用户

| 项目 | 目标用户 | 团队规模 |
|------|----------|----------|
| Agentra | AI-native teams | 2-10 |
| swarmclaw | OpenClaw operators | 任意 |
| hindsight | Enterprise + AI startups | 任意 |
| Overseer | 个人开发者 | 个人 |
| Linear | 所有团队 | 任意 |

### 6.2 商业模式

| 项目 | 商业模型 | 定价 |
|------|----------|------|
| Agentra | 开源 + 云服务 | 待定 |
| swarmclaw | 开源 + Desktop app | 免费 |
| hindsight | 云服务 + API | $299/mo+ |
| Linear | SaaS | $8/seat/mo |

### 6.3 差异化定位

| 项目 | 核心差异化 |
|------|------------|
| Agentra | **实时 WebSocket + 多 agent 后端 + 云运行时** |
| swarmclaw | **23+ providers + MCP native + 桌面 app** |
| hindsight | **SOTA memory benchmark + biomimetic** |
| Overseer | **VCS 原生集成 + learnings bubble** |
| Tasuku | **Git-friendly + 轻量** |

---

## 七、强化方向建议

### 7.1 高优先级 (P0)

#### 1. **23+ LLM Provider 支持** (学习 swarmclaw)

Agentra 目前仅支持 3 个 provider (Claude/Codex/OpenCode)，swarmclaw 支持 23+。

**建议方案**:
```go
// 扩展 Provider 接口，支持更多 backend
type Provider interface {
    Execute(ctx context.Context, prompt string, opts Options) (*Session, error)
    Name() string
    Supports(model Model) bool
}

// 添加 provider 配置
providers:
  - type: anthropic
    models: ["claude-opus-4", "claude-sonnet-4"]
  - type: openai
    models: ["gpt-4o", "gpt-4o-mini"]
  - type: openrouter
    models: ["openai/gpt-4o-mini", "anthropic/claude-3-haiku"]
  - type: ollama
    base_url: "http://localhost:11434"
    models: ["llama3", "codellama"]
```

#### 2. **Agent-to-Agent Handoff** (竞品普遍缺失)

Task Graph 系统已设计，当前需要完成实现。

#### 3. **Execution Traces** (结构性日志)

**swarmclaw 的 structured execution**:
- Branching + repeat loops
- Parallel branches + explicit joins
- Restart-safe run state

**建议**:
```go
type ExecutionTrace struct {
    TaskID     string
    Steps      []TraceStep
    Tools      []ToolCall
    Tokens     TokenUsage
    Cost       float64
    StartTime  time.Time
    EndTime    time.Time
}

type TraceStep struct {
    Step   int
    Action string
    Result string
    // ...
}
```

### 7.2 中优先级 (P1)

#### 4. **Memory System 强化** (学习 hindsight)

hindsight 的 biomimetic memory 是 SOTA benchmark，Agentra 可以借鉴:

**多策略检索**:
- Semantic (vector similarity) ✅ 设计中
- Keyword (BM25) ❌ 缺失
- Graph (entity/temporal) ❌ 缺失
- Temporal (time range) ❌ 缺失

**Reflect 操作**: 生成新的 mental models

#### 5. **MCP Native 支持** (学习 swarmclaw)

swarmclaw 的 MCP gateway 可以连接任何 MCP server。

**建议**:
```go
// MCP Gateway 支持
type MCPGateway struct {
    servers []MCPServer
    pool    *ConnectionPool
}

// 支持 stdio, SSE, streamable HTTP
```

#### 6. **桌面 App** (学习 swarmclaw)

swarmclaw 有 Electron Desktop app，降低使用门槛。

**建议**: Phase 3 可考虑 Electron wrapper

### 7.3 低优先级 (P2)

#### 7. **Skills Marketplace** (已规划)

#### 8. **Analytics Dashboard** (已规划)

---

## 八、竞争总结

### 8.1 Agentra 独特优势

1. **实时 WebSocket** - 唯一有实时广播的任务管理平台
2. **完整任务生命周期** - queued→claimed→started→completed/failed
3. **多 agent 后端** - Claude/Codex/OpenCode 统一接口
4. **云运行时** - Phase 1 即将完成
5. **企业级** - PostgreSQL + 多 workspace + JWT

### 8.2 需要追赶的差距

| 差距 | 竞品 | 优先级 |
|------|------|--------|
| 23+ LLM Providers | swarmclaw | P0 |
| MCP Native | swarmclaw/Overseer | P0 |
| 记忆系统 benchmark | hindsight | P1 |
| 桌面 App | swarmclaw | P2 |
| VCS 集成 | Overseer | P2 |

### 8.3 建议的差异化方向

1. **Real-time + MCP** - 唯一同时有实时同步和 MCP 的产品
2. **Cloud Runtime + Memory** - 云运行时 + RAG 记忆
3. **Task Graph** - DAG 多 agent 编排 (竞品普遍缺失)

---

## 九、参考资料

- [Agentra ROADMAP.md](../ROADMAP.md)
- [Agent Workflow Analysis](../agent-workflow-analysis.md)
- [swarmclaw - GitHub](https://github.com/swarmclawai/swarmclaw) (472 stars)
- [hindsight - GitHub](https://github.com/vectorize-io/hindsight) (12,763 stars)
- [Overseer - GitHub](https://github.com/dmmulroy/overseer) (223 stars)
- [Tasuku - GitHub](https://github.com/iheanyi/tasuku) (63 stars)
- [Beekeeper - GitHub](https://github.com/i-am-bee/beekeeper) (52 stars)