# 竞品研究与项目强化分析

**日期**: 2026-05-09
**版本**: v0.2

---

## 一、ROADMAP 项目完成情况

### Phase 1 (Q2-Q3 2025) - Solid Foundation - IN PROGRESS

| 功能 | 状态 | 说明 |
|------|------|------|
| Issue Management | ✅ Done | 完整 CRUD，状态/优先级/分配 |
| Agent Assignment | ✅ Done | 支持 Claude/Codex/OpenCode |
| Task Lifecycle | ✅ Done | queued→claimed→started→completed/failed |
| Local Daemon Runtime | ✅ Done | 自动发现 CLI，轮询任务 |
| Skills System | ✅ Done | 可复用工作流模板 |
| Comments & Mentions | ✅ Done | @mention 解析 |
| Real-time Sync | ✅ Done | WebSocket 广播 |
| Cloud Runtime | 🚧 In Progress | Gateway/容器执行已实现 |
| GitHub Integration | ❌ Not Started | OAuth App/PR linking |
| CLI Installer | ❌ Not Started | one-liner setup |
| Onboarding Wizard | ❌ Not Started | 引导流程 |
| Execution Traces | ❌ Not Started | 结构性日志 |
| Human-in-the-Loop | ❌ Not Started | 审批门 |
| Token Cost Tracking | ❌ Not Started | 实时成本估算 |

### Phase 2 (Q4 2025) - Agent Intelligence - PLANNED

| 功能 | 状态 | 说明 |
|------|------|------|
| Agent Memory Store | ❌ Not Started | pgvector-backed |
| RAG Context Injection | ❌ Not Started | 自动检索 |
| Agentra MCP Server | ❌ Not Started | 暴露为 MCP tools |
| External MCP Registry | ❌ Not Started | GitHub/Slack |
| Sub-Task Trees | ❌ Not Started | 任务分解 |
| Multi-Agent Planner | ❌ Not Started | 规划 agent |
| Task Graph Visualization | ❌ Not Started | DAG view |
| Analytics Dashboard | ❌ Not Started | 统计图表 |

### Phase 3-4 - 未开始

---

## 二、GitHub 竞品收集

### 2.1 核心竞品 (Agent-native Task Management)

| 项目 | Stars | 技术栈 | 核心定位 |
|------|-------|--------|----------|
| **[Overseer](https://github.com/dmmulroy/overseer)** | 223 | Rust | MCP server + SQLite + 原生 VCS 集成 |
| **[Tasuku](https://github.com/iheanyi/tasuku)** | 63 | Go | Markdown 文件存储 + MCP tools |
| **[Beekeeper](https://github.com/i-am-bee/beekeeper)** | 52 | Python | Supervisor agent + 多 agent 编排 |
| **[Moo Tasks](https://github.com/dizlexic/moo-tasks)** | 32 | Nuxt4/MySQL | Kanban UI + MCP server |
| **[AgentsBoard](https://github.com/Justmalhar/AgentsBoard)** | 11 | - | AI Agent 看板 |

### 2.2 相关项目 (Multi-agent Systems)

| 项目 | Stars | 技术栈 | 核心定位 |
|------|-------|--------|----------|
| **[AI-teams-controller](https://github.com/hungson175/AI-teams-controller-public)** | 7 | Claude Code | 多 agent tmux teams + 记忆系统 |
| **[Forge](https://github.com/idkuday/forge)** | 2 | - | AI 软件开发系统 + agent teams |
| **[Overseer](https://github.com/dmmulroy/overseer)** | 223 | Rust | **已在上表** |

---

## 三、技术实现对比

### 3.1 架构对比

| 维度 | Agentra | Overseer | Tasuku | Beekeeper | Moo Tasks |
|------|---------|----------|--------|-----------|-----------|
| **存储** | PostgreSQL | SQLite | Markdown 文件 | 内存/文件 | MySQL |
| **Agent 接口** | CLI Backend | MCP Server | MCP Server | BeeAI Framework | MCP Server |
| **多 agent 协调** | Daemon Poll | 原生 VCS | per-file locking | Supervisor 编排 | 板级隔离 |
| **前端** | Next.js | Web UI (单页) | TUI + CLI | 交互式 UI | Nuxt4 Web |
| **实时性** | WebSocket | 轮询 | 轮询 | 实时 | 轮询 |
| **部署方式** | Docker Compose | npm | go install | mise | Docker |

### 3.2 核心功能矩阵

| 功能 | Agentra | Overseer | Tasuku | Beekeeper | Moo Tasks |
|------|---------|----------|--------|-----------|-----------|
| Issue/Task 管理 | ✅ | ✅ milestone/task/subtask | ✅ | ✅ hierarchical | ✅ Kanban |
| Agent 分配 | ✅ | ✅ (via codemode) | ✅ | ✅ (supervisor) | ✅ |
| 实时状态 | ✅ WS | ❌ | ❌ | ✅ | ❌ |
| 任务分解 | ❌ | ❌ | ❌ | ✅ | ❌ |
| 记忆系统 | ❌ | ✅ (learnings bubble) | ✅ (learnings) | ❌ | ❌ |
| MCP 暴露 | ❌ | ✅ | ✅ | ❌ | ✅ |
| 自托管 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 云运行时 | ✅ (Phase 1) | ❌ | ❌ | ❌ | ❌ |
| GitHub 集成 | ❌ | ❌ | ❌ | ❌ | ❌ |

### 3.3 Overseer 深度分析 (最强大竞品)

**架构**:
```
┌─────────────────────────────────────┐
│     Overseer MCP (Node.js)          │
│  - Single "execute" tool (codemode) │
│  - VM sandbox with tasks/learnings  │
└─────────────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────┐
│         os CLI (Rust)               │
│  - SQLite storage                   │
│  - jj-lib (primary VCS)             │
│  - gix (git fallback)               │
└─────────────────────────────────────┘
```

**优势**:
1. **VCS 原生集成**: `tasks.start()` 创建 bookmark, `tasks.complete()` 提交代码
2. **Learnings 冒泡**: 子任务完成时 learnings 传递到父任务
3. **渐进上下文**: 任务继承祖先的 context
4. **codemode 模式**: agent 写 JS 代码在 VM 中执行

**劣势**:
1. archived 项目 - 不再维护
2. 无 WebSocket 实时性
3. 无多 workspace 支持
4. 无 GitHub/PR 集成

### 3.4 Tasuku 深度分析 (最相似定位)

**架构**:
- Markdown 文件存储 (`.tasuku/tasks/*.md`)
- MCP tools (40+)
- Hooks 系统 (session 启动/结束)

**优势**:
1. **Git-friendly**: 一个 task 一个文件，diff 清晰
2. **per-file locking**: 多 agent 并行安全
3. **轻量**: go install 一键安装
4. **多 editor 支持**: Claude Code, Cursor, Copilot, Codex, OpenCode

**劣势**:
1. 无实时同步
2. 无中央协调
3. learnings 无结构性存储

### 3.5 Beekeeper 深度分析 (多 agent 编排)

**架构**:
- Supervisor agent (中央协调)
- Agent registry
- Task management (分解子任务)

**优势**:
1. **Supervisor 编排**: 自动分解任务给 specialized agents
2. **Interactive + Autonomous 模式**
3. **Workspace persistence**: 配置可复用

**劣势**:
1. 无持久化任务存储
2. 依赖 BeeAI 框架
3. 无标准 MCP 接口

---

## 四、产品化对比

### 4.1 商业模式

| 产品 | 商业模式 | 定价 |
|------|----------|------|
| Agentra | 开源 + 云服务 | 待定 |
| Linear | SaaS | $8/seat/mo |
| Jira | SaaS | $7.75/seat/mo |
| Overseer | 开源 | 免费 |
| Moo Tasks | 开源 | 免费 |

### 4.2 开发者生态

| 产品 | MCP 支持 | VS Code/Cursor | CLI 工具 | Editor 插件 |
|------|----------|----------------|----------|-------------|
| Agentra | ❌ | ❌ | ✅ | ❌ |
| Overseer | ✅ | ✅ | ✅ | ❌ |
| Tasuku | ✅ | ✅ | ✅ | ✅ (slash commands) |
| Moo Tasks | ✅ | ✅ | ❌ | ❌ |
| Beekeeper | ❌ | ❌ | ✅ | ❌ |

---

## 五、强化方向建议

### 5.1 高优先级 (P0)

#### 1. **MCP Server 暴露** ⭐⭐⭐⭐⭐

Overseer 和 Moo Tasks 都通过 MCP 暴露 task management 能力。Agentra 应该:
- 暴露 issues, skills, memory 作为 MCP tools
- 支持 external MCP registry (GitHub, Slack)
- 参考: Overseer codemode pattern

**为什么重要**: MCP 是 AI agent 的事实标准，暴露 MCP server 可以让任何 MCP-compatible agent 直接使用 Agentra

#### 2. **Agent Memory (RAG)** ⭐⭐⭐⭐⭐

竞品分析显示记忆系统是核心差异点:
- Overseer: learnings bubble up
- Tasuku: 轻量 learnings capture
- AI-teams-controller: self-improving memory

**建议**: pgvector-backed per-agent memory store (ROADMAP Phase 2)

#### 3. **Execution Traces** ⭐⭐⭐⭐

Agent 工作流分析报告 (docs/agent-workflow-analysis.md) 显示当前缺乏结构性日志:
- 记录每个 task 的 steps, tools, tokens, cost
- 可视化执行链路
- 失败时快速定位问题

### 5.2 中优先级 (P1)

#### 4. **强制 PR 流程** ⭐⭐⭐⭐

Agent 跳过 PR 的问题需要从系统层面解决:
- skill 指令中明确要求 PR
- branch 命名规范
- CI + Code Review 门控

#### 5. **GitHub 深度集成** ⭐⭐⭐⭐

竞品均无 GitHub 集成，但这是企业必需功能:
- OAuth GitHub App
- Issue ↔ PR ↔ commit linking
- PR status badge on issue cards

#### 6. **实时性增强** ⭐⭐⭐

Agentra 有 WebSocket 实时同步，但竞品多无此功能:
- 保持领先优势
- 增加 message replay
- 增加 heartbeat pings

### 5.3 低优先级 (P2)

#### 7. **Task 分解支持** ⭐⭐⭐

Beekeeper 的 supervisor 编排模式值得借鉴:
- Planner agent role
- 子任务树模型
- 并行执行独立子任务

#### 8. **Analytics Dashboard** ⭐⭐

可视化:
- Agent 成功率
- Cycle time
- Cost dashboard

---

## 六、竞品关键技术亮点

### 6.1 Overseer 的 VCS 原生集成

```javascript
await tasks.start(login.id);  // VCS: creates bookmark
// ... work ...
await tasks.complete(login.id, {  // VCS: commits
  result: "Implemented",
  learnings: ["bcrypt rounds should be 12+"]
});
```

### 6.2 Tasuku 的 per-file locking

```
.tasuku/
├── tasks/task-id.md      # One file per task
├── archive/              # Archived tasks
├── context/
│   ├── learnings.md
│   └── decisions.md
└── index.json
```

### 6.3 Beekeeper 的 Supervisor 编排

```
User Input → Supervisor Agent → Agent Registry
                                ├── Operator Agent 1
                                ├── Operator Agent 2
                                └── ...
```

---

## 七、结论

### 7.1 Agentra 的独特优势

1. **实时同步**: 唯一有 WebSocket 实时广播的产品
2. **完整任务生命周期**: queued→claimed→started→completed
3. **多 agent 后端**: Claude/Codex/OpenCode 统一接口
4. **云运行时**: Phase 1 即将完成
5. **企业级**: PostgreSQL + 多 workspace + JWT

### 7.2 需要追赶的差距

1. **MCP 暴露**: Overseer/Moo Tasks 已实现
2. **记忆系统**: Overseer/Tasuku 已实现
3. **GitHub 集成**: 所有竞品均无，但 Agentra 尚未实现

### 7.3 建议的差异化方向

1. **Real-time + MCP**: 唯一同时有实时同步和 MCP 的产品
2. **Cloud Runtime + Memory**: 云运行时 + RAG 记忆
3. **GitHub-first**: 深度 GitHub 集成

---

### 7.4 新竞品发现 (2026-05)

| 项目 | Stars | 关键特性 |
|------|-------|----------|
| [swarmclaw](https://github.com/swarmclawai/swarmclaw) | 471 | 自托管 runtime, MCP tools, agent memory, 23+ LLM providers |
| [hindsight](https://github.com/vectorize-io/hindsight) | 12k | Agent Memory That Learns |
| [dispatch](https://github.com/rezzedai/dispatch) | - | MCP task server for Claude Code |
| [mem0-mcp](https://github.com/coleam00/mcp-mem0) | - | MCP server for long term agent memory |
| [agentmemo-mcp](https://github.com/andrewpetecoleman-cloud/agentmemo-mcp) | - | MCP + approval gateway |

**swarmclaw 是最强竞品** - 与 Agentra 高度重叠 (self-hosted, multi-agent, MCP tools)，且已有 agent memory 内置，支持 23+ LLM providers。

### 7.5 建议的改进方向

| 优先级 | 改进 | 理由 |
|--------|------|------|
| P0 | **Agent-to-Agent Handoff** | 竞品普遍缺失，任务分解是 multi-agent 的核心 |
| P0 | **23+ LLM Providers** | swarmclaw 已实现，Agentra 目前仅支持 Claude/Codex/OpenCode |
| P1 | **Execution Traces** | 结构性日志，调试和成本分析的基础 |
| P1 | **Memory → MCP 暴露** | Memory tools 通过 MCP Server 暴露给 agents |

---

## 八、参考资料

- [Agentra ROADMAP.md](ROADMAP.md)
- [Agent Workflow Analysis](agent-workflow-analysis.md)
- [Overseer - GitHub](https://github.com/dmmulroy/overseer)
- [Tasuku - GitHub](https://github.com/iheanyi/tasuku)
- [Beekeeper - GitHub](https://github.com/i-am-bee/beekeeper)
- [Moo Tasks - GitHub](https://github.com/dizlexic/moo-tasks)
- [AI-teams-controller - GitHub](https://github.com/hungson175/AI-teams-controller-public)
- [swarmclaw - GitHub](https://github.com/swarmclawai/swarmclaw)
- [hindsight - GitHub](https://github.com/vectorize-io/hindsight)
- [dispatch - GitHub](https://github.com/rezzedai/dispatch)
