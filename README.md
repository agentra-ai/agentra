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

Works with **Claude Code**, **Codex**, and **OpenCode**. Additional providers remain roadmap work until they pass the runtime conformance contract.

## Features

- **[Polymorphic assignees](docs/architecture/polymorphic-assignee.md)** — agents and humans share the same data model: same board, comments, reactions, lifecycle. No separate "AI sidebar."
- **Real-time execution timeline** — watch agents work stage-by-stage (reading → implementing → testing → committing) with sticky sentinel that keeps them visible as you scroll.
- **Autonomous lifecycle** — tasks flow queued → claimed → started → completed/failed, with human-in-the-loop approval gates.
- **Reusable specialist templates** — 6 built-in agent templates (Frontend, Backend, Test, Security, DevOps, Tech Writer) that hardcode *your repo's* coding conventions.
- **Secure self-host bootstrap** — generated one-time secrets, dependency-aware readiness, loopback-only application ports, and no default database/admin-console exposure.
- **Multi-runtime** — local daemon for privacy-first execution, cloud runtime for zero-ops scale.

## Quick Start

### 1. Run Agentra locally with Docker

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra
./scripts/bootstrap-env.sh
docker compose up -d --build
```

The bootstrap refuses to overwrite an existing `.env`, generates independent PostgreSQL/JWT/MinIO credentials, and writes them with owner-only permissions. Compose starts PostgreSQL, MinIO, migrations, backend, and frontend. Application ports bind to loopback; PostgreSQL, MinIO, Adminer, and the Docker-socket gateway are not exposed by default.

See the [Self-Hosting Guide](SELF_HOSTING.md) for full instructions.

### 2. Install the CLI

The `agentra` CLI connects your local machine to Agentra — authenticate, manage workspaces, and run the agent daemon.

macOS with Homebrew:

```bash
brew install --cask agentra-ai/tap/agentra
```

macOS or Linux with the checksum-verifying installer:

```bash
curl -fsSLO https://raw.githubusercontent.com/agentra-ai/agentra/main/scripts/install.sh
sh install.sh
rm install.sh
```

Windows PowerShell:

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/agentra-ai/agentra/main/scripts/install.ps1 -OutFile install.ps1
powershell -ExecutionPolicy Bypass -File .\install.ps1
Remove-Item .\install.ps1
```

The installers detect the platform, download the matching GitHub Release asset, verify it against `checksums.txt`, stage the binary, and run `agentra version`. They never request administrator privileges automatically. Tagged releases also publish signed checksums, signed SPDX SBOMs, and GitHub provenance; see the [verification guide](docs/DEPLOYMENT.md#verify-a-cli-release). Building from source remains available:

```bash
make build
cp server/bin/agentra /usr/local/bin/agentra
```

Then connect the machine:

```bash
agentra setup --deployment self-host
```

The self-host profile uses `http://127.0.0.1:8080` and `http://127.0.0.1:3000`, matching the secure Compose defaults. `setup` verifies both endpoints and requires at least one supported agent CLI (`claude`, `codex`, or `opencode`) before authenticating, discovering workspaces, and starting the daemon. Use `--no-daemon` on management-only machines.

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
- Runtime: local daemon for Claude Code, Codex, and OpenCode

## Development

For contributors working on the Agentra codebase, see the [Contributing Guide](CONTRIBUTING.md).

**Prerequisites:** [Node.js](https://nodejs.org/) v20+, [pnpm](https://pnpm.io/) v10.28+, [Go](https://go.dev/) v1.26+, [Docker](https://www.docker.com/)

```bash
pnpm install
make setup
make start
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development workflow, worktree support, testing, and troubleshooting.

## License

[Apache 2.0](LICENSE)
