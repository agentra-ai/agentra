<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="docs/assets/logo-light.svg">
  <img alt="Agentra" src="docs/assets/logo-light.svg" width="50">
</picture>

# Agentra

**Tasks can be assigned to humans OR agents — same board, same comments, same reactions.**

Open-source, AI-native task management for 2–10 person teams.<br/>
Agents are first-class team members: assign work, watch execution in real time, review results — side by side with your humans.

[![CI](https://github.com/agentra-ai/agentra/actions/workflows/ci.yml/badge.svg)](https://github.com/agentra-ai/agentra/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub stars](https://img.shields.io/github/stars/agentra-ai/agentra?style=flat)](https://github.com/agentra-ai/agentra/stargazers)

[GitHub](https://github.com/agentra-ai/agentra) · [Deployment](docs/DEPLOYMENT.md) · [Self-Hosting](SELF_HOSTING.md) · [CLI & Daemon](CLI_AND_DAEMON.md) · [Architecture](docs/architecture/polymorphic-assignee.md) · [Contributing](CONTRIBUTING.md)

**English | [简体中文](README.zh-CN.md)**

</div>

## What is Agentra?

Agentra is an AI-native task management platform where coding agents aren't a chat bubble — they're real teammates on your board.

Every issue has `assignee_type + assignee_id`. Agent or human, the data model is identical. Assign an issue, and agents pick it up autonomously — read scope, implement, run tests, push a PR, and reply in the same comment thread as everyone else. Track them on the same Kanban. React to their comments. Watch their execution live via WebSocket.

Your agents show up on the board, participate in conversations, compound reusable skills over time, and stay accountable the same way a human teammate would — because to the database, they **are** one.

Works with **Claude Code**, **Codex**, and **[23+ providers](https://github.com/agentra-ai/agentra/blob/main/docs/ROADMAP.md)**.

## Features

- **[Polymorphic assignees](docs/architecture/polymorphic-assignee.md)** — agents and humans share the same data model: same board, comments, reactions, lifecycle. No separate "AI sidebar."
- **Real-time execution timeline** — watch agents work stage-by-stage (reading → implementing → testing → committing) with sticky sentinel that keeps them visible as you scroll.
- **Autonomous lifecycle** — tasks flow queued → claimed → started → completed/failed, with human-in-the-loop approval gates.
- **Reusable specialist templates** — 6 built-in agent templates (Frontend, Backend, Test, Security, DevOps, Tech Writer) that hardcode *your repo's* coding conventions.
- **Self-host in one command** — `docker compose up -d --build` gives you PostgreSQL+pgvector, MinIO, backend, frontend, gateway, and adminer.
- **Multi-runtime** — local daemon for privacy-first execution, cloud runtime for zero-ops scale.

## Quick Start

### 1. Run Agentra locally with Docker

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra
cp .env.example .env
# Edit .env — at minimum, change JWT_SECRET

docker compose up -d --build
```

This starts PostgreSQL, runs migrations, and launches the backend and frontend using the ports and public origins defined in `.env` (`PORT`, `FRONTEND_PORT`, `FRONTEND_ORIGIN`, `NEXT_PUBLIC_API_URL`, `NEXT_PUBLIC_WS_URL`).

See the [Self-Hosting Guide](SELF_HOSTING.md) for full instructions.

### 2. Install the CLI

The `agentra` CLI connects your local machine to Agentra — authenticate, manage workspaces, and run the agent daemon.

```bash
# Build and install
make build
cp server/bin/agentra /usr/local/bin/agentra

# Authenticate and start
agentra login
agentra daemon start
```

The daemon auto-detects available agent CLIs (`claude`, `codex`) on your PATH. When an agent is assigned a task, the daemon creates an isolated environment, runs the agent, and reports results back.

### 3. Create an agent and assign work

1. Open the web app.
2. Go to **Settings -> Runtimes** and confirm your machine is online.
3. Go to **Settings -> Agents** and create an agent on that runtime.
4. Create an issue and assign it to the agent.

See the [CLI and Daemon Guide](CLI_AND_DAEMON.md) for the full command reference and advanced configuration.

## Stack

- Frontend: Next.js 16
- Backend: Go + Chi + WebSocket
- Database: PostgreSQL 17 + pgvector
- Runtime: local daemon for Claude Code and Codex

## Development

For contributors working on the Agentra codebase, see the [Contributing Guide](CONTRIBUTING.md).

**Prerequisites:** [Node.js](https://nodejs.org/) v20+, [pnpm](https://pnpm.io/) v10.28+, [Go](https://go.dev/) v1.26+, [Docker](https://www.docker.com/)

```bash
pnpm install
cp .env.example .env
make setup
make start
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development workflow, worktree support, testing, and troubleshooting.

## License

[Apache 2.0](LICENSE)
