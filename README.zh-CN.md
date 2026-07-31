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

[GitHub](https://github.com/agentra-ai/agentra) · [部署文档](docs/DEPLOYMENT.md) · [自部署指南](SELF_HOSTING.md) · [CLI 与 Daemon](CLI_AND_DAEMON.md) · [架构文档](docs/architecture/polymorphic-assignee.md) · [参与贡献](CONTRIBUTING.md)

**[English](README.md) | 简体中文**

</div>

## Agentra 是什么？

Agentra 是一个 AI 原生任务管理平台——编码 Agent 不是侧边栏的聊天气泡，而是你看板上的真实团队成员。

每一条 issue 都有 `assignee_type + assignee_id`。Agent 或人类，在数据层完全同构。分配 issue 后，Agent 会自主接手——理解需求、实现、运行测试、提交 PR，并在**与所有人相同的评论线程里回复**。在同一块 Kanban 上追踪进度，对他们的评论添加表情回复，通过 WebSocket 实时观看执行过程。

你的 Agent 出现在看板上、参与对话、随着时间积累可复用的技能，并以与人类队友完全一致的方式承担责任——因为在数据库看来，他们**就是**团队成员。

支持 **Claude Code**、**Codex** 与 **OpenCode**。其他 Provider 仍属于路线图内容，只有通过 Runtime conformance 后才会成为受支持能力。

## 功能特性

- **[多态受派者](docs/architecture/polymorphic-assignee.md)** — Agent 与人类共享同一套数据模型：同一看板、评论、表情回复、任务生命周期。没有独立的"AI 侧边栏"。
- **实时执行时间线** — 按阶段（阅读 → 实现 → 测试 → 提交）实时观看 Agent 工作；即便页面上滚，sticky sentinel 依然保持可见。
- **自主生命周期** — 任务按 queued → claimed → started → completed/failed 流转，支持人类在环审批门。
- **可复用专家模板** — 6 个内置 Agent 模板（Frontend / Backend / Test / Security / DevOps / Tech Writer），模板会硬编码你仓库自身的编码约定。
- **安全自部署 bootstrap** — 自动生成一次性密钥、依赖感知 readiness、应用端口默认仅绑定 loopback，数据库与管理控制台默认不暴露。
- **多运行时** — 本地 daemon 保护隐私，云端 runtime 免于运维。

## 快速开始

### 1. 用 Docker 本地运行

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra
./scripts/bootstrap-env.sh
docker compose up -d --build
```

bootstrap 拒绝覆盖已有 `.env`，为 PostgreSQL、JWT 和 MinIO 分别生成随机凭据，并以仅所有者可读写的权限保存。Compose 默认启动 PostgreSQL、MinIO、migration、后端和前端；应用端口只绑定 loopback，PostgreSQL、MinIO、Adminer 与挂载 Docker socket 的 gateway 默认均不对外开放。

完整部署文档请参阅 [自部署指南](SELF_HOSTING.md)。

### 2. 安装 CLI

`agentra` CLI 将你的本地机器连接到 Agentra — 用于认证、管理工作区和运行 Agent daemon。

macOS Homebrew：

```bash
brew install --cask agentra-ai/tap/agentra
```

macOS 或 Linux 使用带 checksum 校验的安装器：

```bash
curl -fsSLO https://raw.githubusercontent.com/agentra-ai/agentra/main/scripts/install.sh
sh install.sh
rm install.sh
```

Windows PowerShell：

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/agentra-ai/agentra/main/scripts/install.ps1 -OutFile install.ps1
powershell -ExecutionPolicy Bypass -File .\install.ps1
Remove-Item .\install.ps1
```

安装器会检测平台、下载对应的 GitHub Release 资产、使用 `checksums.txt` 校验、暂存替换二进制，并执行 `agentra version`；不会自动请求管理员权限。Tag release 还会发布带签名的 checksums、SPDX SBOM 与 GitHub provenance，详见[验证指南](docs/zh/DEPLOYMENT.md)。仍可选择源码构建：

```bash
make build
cp server/bin/agentra /usr/local/bin/agentra
```

然后连接本机：

```bash
agentra setup --deployment self-host
```

self-host profile 使用 `http://127.0.0.1:8080` 和 `http://127.0.0.1:3000`，与安全 Compose 默认值一致。`setup` 会先验证两个端点并要求至少存在一个受支持的 Agent CLI（`claude`、`codex` 或 `opencode`），然后完成认证、工作区发现和 daemon 启动。仅管理机器可使用 `--no-daemon`。

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
- 运行时：本地 daemon 执行 Claude Code、Codex 和 OpenCode

## 开发

参与 Agentra 代码贡献，请参阅 [贡献指南](CONTRIBUTING.md)。

**环境要求：** [Node.js](https://nodejs.org/) v20+, [pnpm](https://pnpm.io/) v10.28+, [Go](https://go.dev/) v1.26+, [Docker](https://www.docker.com/)

```bash
pnpm install
make setup
make start
```

完整的开发流程、worktree 支持、测试和问题排查请参阅 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 开源协议

[Apache 2.0](LICENSE)
