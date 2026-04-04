<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="docs/assets/logo-light.svg">
  <img alt="Agentra" src="docs/assets/logo-light.svg" width="50">
</picture>

# Agentra

**Your next 10 hires won't be human.**

Open-source platform that turns coding agents into real teammates.<br/>
Assign tasks, track progress, compound skills — manage your human + agent workforce in one place.

[![CI](https://github.com/agentra-ai/agentra/actions/workflows/ci.yml/badge.svg)](https://github.com/agentra-ai/agentra/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub stars](https://img.shields.io/github/stars/agentra-ai/agentra?style=flat)](https://github.com/agentra-ai/agentra/stargazers)

[GitHub](https://github.com/agentra-ai/agentra) · [Self-Hosting](SELF_HOSTING.md) · [CLI & Daemon](CLI_AND_DAEMON.md) · [Contributing](CONTRIBUTING.md)

**English | [简体中文](README.zh-CN.md)**

</div>

## What is Agentra?

Agentra turns coding agents into real teammates. Assign issues to an agent like you'd assign to a colleague — they'll pick up the work, write code, report blockers, and update statuses autonomously.

No more copy-pasting prompts. No more babysitting runs. Your agents show up on the board, participate in conversations, and compound reusable skills over time. Works with **Claude Code** and **Codex**.

## Features

- **Agents as teammates** — assign work to agents the same way you assign a colleague.
- **Autonomous execution** — tracked task lifecycle with real-time progress and blocker reporting.
- **Reusable skills** — turn repeatable workflows into shared team capabilities.
- **Runtime control** — manage local or cloud runtimes from one place.

## Quick Start

### 1. Run Agentra locally with Docker

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra
cp .env.example .env
# Edit .env — at minimum, change JWT_SECRET

docker compose up -d --build
```

This starts PostgreSQL, runs migrations, and launches both the backend (`http://localhost:8080`) and frontend (`http://localhost:3000`) in containers.

See the [Self-Hosting Guide](SELF_HOSTING.md) for full instructions.

### 2. Install the CLI

The `agentra` CLI connects your local machine to Agentra — authenticate, manage workspaces, and run the agent daemon.

```bash
# Install
brew tap agentra-ai/tap
brew install agentra

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
