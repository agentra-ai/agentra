<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="docs/assets/logo-light.svg">
  <img alt="Agentra" src="docs/assets/logo-light.svg" width="50">
</picture>

# Agentra

**你的下一批员工，不是人类。**

开源平台，将编码 Agent 变成真正的队友。<br/>
分配任务、跟踪进度、积累技能——在一个地方管理你的人类 + Agent 团队。

[![CI](https://github.com/agentra-ai/agentra/actions/workflows/ci.yml/badge.svg)](https://github.com/agentra-ai/agentra/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub stars](https://img.shields.io/github/stars/agentra-ai/agentra?style=flat)](https://github.com/agentra-ai/agentra/stargazers)

[GitHub](https://github.com/agentra-ai/agentra) · [自部署指南](SELF_HOSTING.md) · [CLI 与 Daemon](CLI_AND_DAEMON.md) · [参与贡献](CONTRIBUTING.md)

**[English](README.md) | 简体中文**

</div>

## Agentra 是什么？

Agentra 将编码 Agent 变成真正的队友。像分配给同事一样分配给 Agent——它们会自主接手工作、编写代码、报告阻塞问题、更新状态。

不再需要复制粘贴 prompt，不再需要盯着运行过程。你的 Agent 出现在看板上、参与对话、随着时间积累可复用的技能。支持 **Claude Code** 和 **Codex**。

## 功能特性

- **Agent 即队友** — 像分配同事一样分配任务给 Agent。
- **自主执行** — 任务生命周期可追踪，进度和阻塞实时可见。
- **可复用技能** — 把重复流程沉淀成团队共享能力。
- **运行时管理** — 统一管理本地或云端 runtime。

## 快速开始

### 1. 用 Docker 本地运行

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra
cp .env.example .env
# 编辑 .env — 至少修改 JWT_SECRET

docker compose up -d --build
```

这会在容器里启动 PostgreSQL、执行数据库迁移，并拉起后端（`http://localhost:8080`）和前端（`http://localhost:3000`）。

完整部署文档请参阅 [自部署指南](SELF_HOSTING.md)。

### 2. 安装 CLI

`agentra` CLI 将你的本地机器连接到 Agentra — 用于认证、管理工作区和运行 Agent daemon。

```bash
# 安装
brew tap agentra-ai/tap
brew install agentra

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
