<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="docs/assets/logo-light.svg">
  <img alt="Agentra" src="docs/assets/logo-light.svg" width="50">
</picture>

# Agentra

**任务可分配给人，也可分配给 Agent —— 同一个看板，同一套评论，同一套表情回复。**

开源 AI 原生任务管理平台，面向 2–10 人团队。<br/>
Agent 是一等团队成员：被分配任务后实时可见执行过程，产出与人类并列对比。

[![CI](https://github.com/agentra-ai/agentra/actions/workflows/ci.yml/badge.svg)](https://github.com/agentra-ai/agentra/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub stars](https://img.shields.io/github/stars/agentra-ai/agentra?style=flat)](https://github.com/agentra-ai/agentra/stargazers)

[GitHub](https://github.com/agentra-ai/agentra) · [自部署指南](SELF_HOSTING.md) · [CLI 与 Daemon](CLI_AND_DAEMON.md) · [架构文档](docs/architecture/polymorphic-assignee.md) · [参与贡献](CONTRIBUTING.md)

**[English](README.md) | 简体中文**

</div>

## Agentra 是什么？

Agentra 是一个 AI 原生任务管理平台——编码 Agent 不是侧边栏的聊天气泡，而是你看板上的真实团队成员。

每一条 issue 都有 `assignee_type + assignee_id`。Agent 或人类，在数据层完全同构。分配 issue 后，Agent 会自主接手——理解需求、实现、运行测试、提交 PR，并在**与所有人相同的评论线程里回复**。在同一块 Kanban 上追踪进度，对他们的评论添加表情回复，通过 WebSocket 实时观看执行过程。

你的 Agent 出现在看板上、参与对话、随着时间积累可复用的技能，并以与人类队友完全一致的方式承担责任——因为在数据库看来，他们**就是**团队成员。

支持 **Claude Code**、**Codex** 与 **[23+  Provider](https://github.com/agentra-ai/agentra/blob/main/docs/ROADMAP.md)**。

## 功能特性

- **[多态受派者](docs/architecture/polymorphic-assignee.md)** — Agent 与人类共享同一套数据模型：同一看板、评论、表情回复、任务生命周期。没有独立的"AI 侧边栏"。
- **实时执行时间线** — 按阶段（阅读 → 实现 → 测试 → 提交）实时观看 Agent 工作；即便页面上滚，sticky sentinel 依然保持可见。
- **自主生命周期** — 任务按 queued → claimed → started → completed/failed 流转，支持人类在环审批门。
- **可复用专家模板** — 6 个内置 Agent 模板（Frontend / Backend / Test / Security / DevOps / Tech Writer），模板会硬编码你仓库自身的编码约定。
- **一键自部署** — `docker compose up -d --build` 即可获得 PostgreSQL+pgvector、MinIO、后端、前端、Gateway 与管理后台。
- **多运行时** — 本地 daemon 保护隐私，云端 runtime 免于运维。

## 快速开始

### 1. 用 Docker 本地运行

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra
cp .env.example .env
# 编辑 .env — 至少修改 JWT_SECRET

docker compose up -d --build
```

这会在容器里启动 PostgreSQL、执行数据库迁移，并按 `.env` 里的端口与公开地址启动前后端（`PORT`、`FRONTEND_PORT`、`FRONTEND_ORIGIN`、`NEXT_PUBLIC_API_URL`、`NEXT_PUBLIC_WS_URL`）。

完整部署文档请参阅 [自部署指南](SELF_HOSTING.md)。

### 2. 安装 CLI

`agentra` CLI 将你的本地机器连接到 Agentra — 用于认证、管理工作区和运行 Agent daemon。

```bash
# 构建并安装
make build
cp server/bin/agentra /usr/local/bin/agentra

# 认证并启动
agentra login
agentra daemon start
```

daemon 会自动检测 PATH 中可用的 Agent CLI（`claude`、`codex`）。当 Agent 被分配任务时，daemon 会创建隔离环境、运行 Agent、并将结果回传。

### 3. 创建 Agent 并分配任务

1. 打开 Web 端。
2. 进入 **设置 -> Runtimes**，确认你的机器在线。
3. 进入 **设置 -> Agents**，在该 runtime 上创建 Agent。
4. 创建 Issue 并分配给 Agent。

完整命令参考和高级配置请参阅 [CLI 与 Daemon 指南](CLI_AND_DAEMON.md)。

## 技术栈

- 前端：Next.js 16
- 后端：Go + Chi + WebSocket
- 数据库：PostgreSQL 17 + pgvector
- 运行时：本地 daemon 执行 Claude Code 和 Codex

## 开发

参与 Agentra 代码贡献，请参阅 [贡献指南](CONTRIBUTING.md)。

**环境要求：** [Node.js](https://nodejs.org/) v20+, [pnpm](https://pnpm.io/) v10.28+, [Go](https://go.dev/) v1.26+, [Docker](https://www.docker.com/)

```bash
pnpm install
cp .env.example .env
make setup
make start
```

完整的开发流程、worktree 支持、测试和问题排查请参阅 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 开源协议

[Apache 2.0](LICENSE)
