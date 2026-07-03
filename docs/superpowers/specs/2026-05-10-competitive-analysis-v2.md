# 竞品全面对比分析报告 v2

**日期**: 2026-05-10
**覆盖范围**: 35+ 项目，5 维度对比
**对比维度**: 技术架构、核心原理、实现方案、交互设计、产品定位

---

## 一、竞品全景图 (更新)

### 1.1 核心竞品总览

| 项目 | Stars | 技术栈 | 定位 | 与 Agentra 重叠度 |
|------|-------|--------|------|-------------------|
| **Agentra (本案)** | - | Go + Next.js + pgvector | AI-native task management + agent runtime | - |
| **open-multi-agent** | 6,086 | TypeScript (Node.js) | Goal → Task DAG 自动编排 | ⭐⭐⭐⭐⭐ |
| **wshobson/agents** | 35,093 | Python (Claude Code plugins) | Claude Code 插件生态 | ⭐⭐ |
| **edict** | 15,678 | Python (OpenClaw) | 9-agent 三省六部制编排 | ⭐⭐⭐ |
| **swarmclaw** | 472 | Node.js (Electron + Next.js) | Self-hosted agent runtime | ⭐⭐⭐⭐ |
| **kiwiq** | 1,043 | Python | 企业级 agent 编排 + 多层记忆 | ⭐⭐⭐ |
| **agency-swarm** | 4,340 | Python | 多 agent 编排框架 | ⭐⭐⭐ |
| **agent-tasks** | - | Node.js (TypeScript) | MCP 文件任务管理 | ⭐⭐⭐⭐ |
| **overstory** | 1,284 | TypeScript | 多 agent 编码编排 + tmux 运行时 | ⭐⭐⭐ |
| **hindsight** | 12,763 | Python/Go | SOTA agent memory | ⭐⭐ |

### 1.2 竞品分类矩阵

```
                          ┌─ Task Management ─┐
                          │                    │
         Agentra ─────────┤  open-multi-agent  ├──  edict
                          │  agent-tasks       │
                          │  swarmclaw         │
                          └────────────────────┘
                                    │
                          ┌─────────┴─────────┐
                          │                    │
                  Orchestration         Memory/Context
                          │                    │
               wshobson/agents          hindsight
               agency-swarm             kiwiq
               overstory                Engram
               Shannon
```

---

## 二、技术架构深度对比

### 2.1 架构模式分类

| 模式 | 代表项目 | 特点 | Agentra 对应 |
|------|----------|------|-------------|
| **Goal → DAG** | open-multi-agent | Coordinator 自动分解目标为 DAG | Task Graph (手动创建) |
| **Plugin Ecosystem** | wshobson/agents | 80 插件 + 185 agents + 153 skills | Skills 系统 (基础) |
| **Role-based Teams** | edict | 9 个预定义角色 (三省六部制) | Agent assignees (任意角色) |
| **File-native** | agent-tasks | Markdown + MCP + git hooks | PostgreSQL + WebSocket |
| **Enterprise Memory** | kiwiq | JSON agents + 多层记忆 | pgvector RAG |
| **Runtime Swarm** | swarmclaw | 23+ LLM + MCP gateway + desktop | 3 CLI + API providers |

### 2.2 open-multi-agent 深度分析 (最强新竞品)

**架构**:
```
Coordinator Agent (Claude Sonnet 4.6)
  │
  ├── 接收 goal → 分解为 Task DAG
  │     ├── design-api       (architect agent)
  │     ├── implement        (developer agent) ─┐
  │     └── scaffold-tests   (tester agent)  ──┤ parallel
  │                                            │
  │     └── review-code      (reviewer) ←──────┘ depends on
  │
  └── 合成最终结果
```

**与 Agentra Task Graph 的核心差异**:

| 维度 | Agentra | open-multi-agent |
|------|---------|------------------|
| 触发方式 | 手动创建 issue → 分配 agent | 一句话 goal → 自动 DAG |
| 图构建 | 用户/Planner 手动定义节点 | Coordinator 运行时生成 |
| 执行模型 | 持久化 DAG + 任务队列 | 内存中 DAG + 流式执行 |
| 持久化 | PostgreSQL (所有节点) | 示例中的 HTML dashboard |
| 前端 | Next.js App (CRUD + GraphView) | onProgress 事件 + HTML dashboard |
| 部署 | Docker Compose | npm install + 3 deps |
| MCP | ✅ Agentra MCP Server | ✅ connectMCPTools() |
| Memory | pgvector RAG | 可插拔 MemoryStore |

**关键劣势 vs Agentra**:
- 无持久化任务管理 UI (不是 Linear 替代品)
- 无多人 workspace 协作
- 无 agent 分配/委派生命周期
- 无 WebSocket 实时同步
- 无 cloud runtime

### 2.3 agent-tasks (MCP Task Management)

**架构**:
```
Markdown files (.mcp-tasks/tasks/*.md)
  │
  ├── SQLite 索引 (快速查询)
  │
  └── MCP stdio server (20 tools)
       ├── task_create, task_claim, task_complete
       ├── git hook: prepare-commit-msg
       ├── git hook: post-commit → link-commit
       └── git hook: post-merge → auto-transition
```

**与 Agentra 的差异**:
- 文件存储 vs PostgreSQL — 更轻量但不可扩展
- MCP-only vs Web + MCP — 无 Web UI
- Git-native hooks vs 独立生命周期 — 更适合开发者
- 无 agent 运行时 — 只是任务跟踪
- 20 MCP tools vs Agentra 的 issue/skill/memory MCP

### 2.4 wshobson/agents (Plugin Ecosystem)

**规模**: 80 插件 / 185  agents / 153 技能 / 16 编排器

**关键模式**:
- 渐进式加载 (只加载需要的插件)
- 每个插件隔离的 agents/commands/skills
- 混合编排: Haiku 快速任务、Sonnet 深度分析、Opus 架构

**对 Agentra 的启示**:
- Skills Marketplace 应该遵循此模式 (渐进式披露)
- 每个 skill 应该是独立的、单一用途的
- 插件化 agent 能力可以降低 token 成本

---

## 三、核心原理对比

### 3.1 任务分解策略

| 策略 | 项目 | 实现方式 | Agentra 现状 |
|------|------|----------|-------------|
| **LLM 自动分解** | open-multi-agent | Coordinator agent + prompt | ❌ 需要手动创建 |
| **角色预定义** | edict | 9 固定角色 + 模板 | ✅ 通过 Skills 系统 |
| **模板驱动** | wshobson/agents | 16 工作流编排器 | ✅ Skills 模板 |
| **手动分解** | agent-tasks | 用户手动创建 + MCP | ✅ Issue CRUD |

**关键发现**: Agentra 在任务分解上落后于 open-multi-agent 的 "一句话 goal → 自动 DAG"。

### 3.2 多 Agent 通信

| 模式 | 项目 | 实现 |
|------|------|------|
| **Handoff Protocol** | Agentra | context bundle + artifacts (已实现 ✅) |
| **Shared Memory** | open-multi-agent | MemoryStore 接口 (可插拔) |
| **Delegation** | swarmclaw | 母 agent 委派给 subagent |
| **Artifact Passing** | agent-tasks | Markdown 文件 → 下一个 agent 读取 |
| **Supervisor** | agency-swarm | 中央 supervisor 分解给 workers |

### 3.3 Memory 架构演进

| 层级 | 项目 | 存储 | 检索 |
|------|------|------|------|
| **单层 vector** | Agentra (现状) | pgvector | 余弦相似度 |
| **多层记忆** | kiwiq | JSON + vector + graph | 多层检索 |
| **Biomimetic** | hindsight | World/Experience/Mental Model | 4-strategy + RRF |
| **文件 + SQLite** | agent-tasks | Markdown + SQLite | BM25 + SQL |
| **可插拔** | open-multi-agent | in-memory KV / Redis / PG | 实现 MemoryStore |

**Agentra 记忆系统现状**: ✅ 4-strategy + RRF fusion (本次迭代已实现)

---

## 四、实现方案对比

### 4.1 LLM Provider 支持

| 项目 | Provider 数 | 类型 |
|------|------------|------|
| swarmclaw | 23+ | CLI + API + OpenClaw |
| wshobson/agents | 185 agents (Claude Code plugins) | CLI-based |
| open-multi-agent | 10 | API (Anthropic, OpenAI, Gemini, DeepSeek 等) |
| edict | 多 (OpenClaw-based) | CLI-based |
| **Agentra** | **7** (3 CLI + 4 API) | CLI + API |

### 4.2 任务持久化

| 项目 | 存储 | 优点 | 缺点 |
|------|------|------|------|
| Agentra | PostgreSQL + pgvector | 企业级、多 workspace | 需要 Docker |
| agent-tasks | Markdown + SQLite | Git-friendly、零依赖 | 不可扩展 |
| swarmclaw | SQLite + 文件系统 | 本地、桌面 app | 无多租户 |
| open-multi-agent | 无 (内存) | 零配置 | 无持久化 |
| edict | 文件 + 审计日志 | 审计追踪 | 无查询 |

### 4.3 实时性

| 项目 | 实时机制 | Agentra 优势 |
|------|----------|-------------|
| Agentra | WebSocket + Hub broadcast | ✅ 唯一实时任务管理 |
| open-multi-agent | onProgress 事件 | 单进程内 |
| edict | Dashboard 轮询 | 5-10s 延迟 |
| agent-tasks | MCP stdio (阻塞) | 无实时 |
| swarmclaw | WebSocket | ✅ 有实时 |

---

## 五、交互设计对比

### 5.1 UI/UX 模式

| 项目 | 主要界面 | 交互模型 | Agentra 对比 |
|------|----------|----------|-------------|
| Agentra | Next.js Web App | Issue cards + WS 实时 + DAG view | ✅ 完整 CRUD |
| open-multi-agent | HTML dashboard (post-run) | 事件流 + 可视化 DAG | 更强 (实时) |
| edict | Dashboard + Kanban | 实时 dashboard + 配置面板 | 类似 |
| swarmclaw | Electron Desktop + Web | Org chart + Agent chat | 不同范式 |
| agent-tasks | CLI + MCP | 终端优先 | 缺失 CLI 体验 |

### 5.2 开发者体验

| 项目 | 安装 | 配置 | 上手时间 |
|------|------|------|----------|
| Agentra | Docker Compose | 需要配置 | ~5min |
| open-multi-agent | `npm install` | 设置 API key | ~1min |
| agent-tasks | `npm install -g` | `agent-tasks init` | ~30s |
| edict | 克隆 repo + Python | 配置 OpenClaw | ~3min |
| swarmclaw | `npm install -g` 或 Desktop app | 内置向导 | ~1min |

**Agentra 在上手难度上处于劣势** — 需要 Docker + PostgreSQL + MinIO，而竞品多是单命令安装。

---

## 六、产品定位对比

### 6.1 定位地图

```
                    Human-facing
                         │
              agent-tasks │  Agentra
              (纯工具)    │  (平台)
                         │
    Light ───────────────┼─────────────── Heavy
                         │
     open-multi-agent    │     edict
     (库/框架)           │     (系统)
                         │
                   Agent-facing
```

### 6.2 目标用户

| 项目 | 目标用户 | 团队规模 | 商业模式 |
|------|----------|----------|----------|
| Agentra | AI-native teams | 2-10 | 开源 + 云服务 |
| open-multi-agent | TypeScript 后端开发者 | 任意 | MIT 开源 |
| wshobson/agents | Claude Code 用户 | 个人 | 开源插件 |
| edict | OpenClaw 用户 | 1-5 | 开源 |
| agent-tasks | 使用 agent 的开发者 | 个人 | MIT 开源 |
| kiwiq | 企业 AI 团队 | 10+ | 开源 + 云 |

---

## 七、强化方向建议 (更新)

### 7.1 P0: Goal → Task DAG 自动分解 ⭐⭐⭐⭐⭐

**学习**: open-multi-agent 的 "一句话 goal → 自动 DAG"

**当前状态**: Agentra Task Graph 需要手动创建节点和边

**建议**:
```
POST /api/issues/:id/auto-decompose
  → Planner Agent 分析 issue
  → 自动生成 task graph (nodes + edges)
  → 分配 specialist agents
  → 返回 DAG 结构供用户审核
```

**实现优先于**: open-multi-agent 已有执行能力，但缺少持久化 UI。Agentra 可提供两者。

### 7.2 P0: 一键安装体验 ⭐⭐⭐⭐

**问题**: Agentra 需要 Docker、PostgreSQL、MinIO，竞品都是 `npm install -g`

**建议**: 
- `npx create-agentra` 一键脚手架
- 嵌入式 SQLite 模式 (开发环境)
- 单二进制部署 (Go embed frontend)

### 7.3 P1: Goal-first API ⭐⭐⭐

**学习**: open-multi-agent 的 `runTeam(team, goal)` 模式

**已有基础**: Agentra 已有 Task Graph + Planner agent role

**建议**: 添加高层次 API
```go
// 新 API
POST /api/workspaces/:id/execute
{
  "goal": "Build a REST API for todo list",
  "team": ["architect", "developer", "reviewer"]
}
// → 返回 task graph ID + WebSocket 流式进度
```

### 7.4 P1: Agent-tasks 的 Git-native 模式 ⭐⭐⭐

**学习**: agent-tasks 的 git hooks + 文件存储

**建议**: 
- Agentra Git Hook: `prepare-commit-msg` 自动关联 issue ID
- `post-merge` hook 自动完成 task (参考 agent-tasks)
- 可选 Markdown 导出 (兼容 git diff)

### 7.5 P2: 插件生态 (长期) ⭐⭐

**学习**: wshobson/agents 的 80 插件架构

**已有基础**: Skills 系统

**建议**: 扩展 Skills 为插件格式 (agent + commands + skills 打包)

---

## 八、竞争定位更新

### 8.1 Agentra 的核心护城河

| 优势 | 竞品现状 | 护城河深度 |
|------|----------|-----------|
| WebSocket 实时 | open-multi-agent 有事件、edict 轮询 | 深 |
| 持久化 UI | 竞品无或弱 | 深 |
| Multi-workspace | 无竞品支持 | 中 |
| Cloud Runtime | 无竞品支持 | 中 |
| Task Graph + 手动编辑 | 无竞品有 | 深 |
| MCP Server | agent-tasks 有, open-multi-agent 有 | 浅 |

### 8.2 需要追赶的差距

| 差距 | 竞品 | 紧迫度 |
|------|------|--------|
| Goal → DAG 自动分解 | open-multi-agent | 🔴 P0 |
| 一键安装 | agent-tasks, open-multi-agent | 🔴 P0 |
| TypeScript 原生库 | open-multi-agent | 🟡 P1 |
| Git-native hooks | agent-tasks | 🟡 P2 |
| Plugin ecosystem | wshobson/agents | 🟢 P3 |

### 8.3 建议差异化路径

1. **Goal → DAG + 持久化 UI** — open-multi-agent 自动分解但无持久化。Agentra 两者都提供。
2. **Real-time DAG 可视化** — WebSocket 推送 graph 状态变化 (无竞品)
3. **Human-in-the-loop 审批** — Agent 分解后由人审核再执行

---

## 九、参考资料

- [open-multi-agent](https://github.com/open-multi-agent/open-multi-agent) (6,086★) — Goal → DAG
- [wshobson/agents](https://github.com/wshobson/agents) (35,093★) — Claude Code plugins
- [edict](https://github.com/cft0808/edict) (15,678★) — OpenClaw 三省六部
- [agency-swarm](https://github.com/VRSEN/agency-swarm) (4,340★) — Multi-agent framework
- [overstory](https://github.com/jayminwest/overstory) (1,284★) — Coding agent orchestration
- [kiwiq](https://github.com/rcortx/kiwiq) (1,043★) — Enterprise agent platform
- [agent-tasks](https://github.com/nash-software/mcp-agent-tasks) — MCP task management
- [Agentra ROADMAP.md](../../ROADMAP.md)
- [上一版竞品分析](2026-05-10-competitive-analysis-full-design.md)
