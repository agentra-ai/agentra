# CLI and Agent Daemon Guide

The `agentra` CLI connects your local machine to Agentra. It handles authentication, workspace management, issue tracking, and runs the agent daemon that executes AI tasks locally.

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap agentra-ai/tap
brew install agentra-cli
```

### Build from Source

```bash
git clone https://github.com/agentra-ai/agentra.git
cd agentra
make build
cp server/bin/agentra /usr/local/bin/agentra
```

### Update

```bash
agentra update
```

This auto-detects your installation method (Homebrew or manual) and upgrades accordingly.

## Quick Start

```bash
# 1. Authenticate (opens browser for login)
agentra login

# 2. Start the agent daemon
agentra daemon start

# 3. Done — agents in your watched workspaces can now execute tasks on your machine
```

`agentra login` automatically discovers all workspaces you belong to and adds them to the daemon watch list.

## Authentication

### Browser Login

```bash
agentra login
```

Opens your browser for OAuth authentication, creates a 90-day personal access token, and auto-configures your workspaces.

### Token Login

```bash
agentra login --token
```

Authenticate by pasting a personal access token directly. Useful for headless environments.

### Check Status

```bash
agentra auth status
```

Shows your current server, user, and token validity.

### Logout

```bash
agentra auth logout
```

Removes the stored authentication token.

## Agent Daemon

The daemon is the local agent runtime. It detects available AI CLIs on your machine, registers them with the Agentra server, and executes tasks when agents are assigned work.

### Start

```bash
agentra daemon start
```

By default, the daemon runs in the background and logs to `~/.agentra/daemon.log`.

To run in the foreground (useful for debugging):

```bash
agentra daemon start --foreground
```

### Stop

```bash
agentra daemon stop
```

### Status

```bash
agentra daemon status
agentra daemon status --output json
```

Shows PID, uptime, detected agents, and watched workspaces.

### Logs

```bash
agentra daemon logs              # Last 50 lines
agentra daemon logs -f           # Follow (tail -f)
agentra daemon logs -n 100       # Last 100 lines
```

### Supported Agents

The daemon auto-detects these AI CLIs on your PATH:

| CLI | Command | Description |
|-----|---------|-------------|
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | `claude` | Anthropic's coding agent |
| [Codex](https://github.com/openai/codex) | `codex` | OpenAI's coding agent |

You need at least one installed. The daemon registers each detected CLI as an available runtime.

### How It Works

1. On start, the daemon detects installed agent CLIs and registers a runtime for each agent in each watched workspace
2. It polls the server at a configurable interval (default: 3s) for claimed tasks
3. When a task arrives, it creates an isolated workspace directory, spawns the agent CLI, and streams results back
4. Heartbeats are sent periodically (default: 15s) so the server knows the daemon is alive
5. On shutdown, all runtimes are deregistered

### Configuration

Daemon behavior is configured via flags or environment variables:

| Setting | Flag | Env Variable | Default |
|---------|------|--------------|---------|
| Poll interval | `--poll-interval` | `AGENTRA_DAEMON_POLL_INTERVAL` | `3s` |
| Heartbeat interval | `--heartbeat-interval` | `AGENTRA_DAEMON_HEARTBEAT_INTERVAL` | `15s` |
| Agent timeout | `--agent-timeout` | `AGENTRA_AGENT_TIMEOUT` | `2h` |
| Max concurrent tasks | `--max-concurrent-tasks` | `AGENTRA_DAEMON_MAX_CONCURRENT_TASKS` | `20` |
| Daemon ID | `--daemon-id` | `AGENTRA_DAEMON_ID` | hostname |
| Device name | `--device-name` | `AGENTRA_DAEMON_DEVICE_NAME` | hostname |
| Runtime name | `--runtime-name` | `AGENTRA_AGENT_RUNTIME_NAME` | `Local Agent` |
| Workspaces root | — | `AGENTRA_WORKSPACES_ROOT` | `~/agentra_workspaces` |

Agent-specific overrides:

| Variable | Description |
|----------|-------------|
| `AGENTRA_CLAUDE_PATH` | Custom path to the `claude` binary |
| `AGENTRA_CLAUDE_MODEL` | Override the Claude model used |
| `AGENTRA_CODEX_PATH` | Custom path to the `codex` binary |
| `AGENTRA_CODEX_MODEL` | Override the Codex model used |

### Self-Hosted Server

When connecting to a self-hosted Agentra instance, point the CLI to your server before logging in:

```bash
export AGENTRA_APP_URL=https://app.example.com
export AGENTRA_SERVER_URL=wss://api.example.com/ws

agentra login
agentra daemon start
```

Or set them persistently:

```bash
agentra config set app_url https://app.example.com
agentra config set server_url wss://api.example.com/ws
```

### Profiles

Profiles let you run multiple daemons on the same machine — for example, one for production and one for a staging server.

```bash
# Start a daemon for the staging server
agentra --profile staging login
agentra --profile staging daemon start

# Default profile runs separately
agentra daemon start
```

Each profile gets its own config directory (`~/.agentra/profiles/<name>/`), daemon state, health port, and workspace root.

## Workspaces

### List Workspaces

```bash
agentra workspace list
```

Watched workspaces are marked with `*`. The daemon only processes tasks for watched workspaces.

### Watch / Unwatch

```bash
agentra workspace watch <workspace-id>
agentra workspace unwatch <workspace-id>
```

### Get Details

```bash
agentra workspace get <workspace-id>
agentra workspace get <workspace-id> --output json
```

### List Members

```bash
agentra workspace members <workspace-id>
```

## Issues

### List Issues

```bash
agentra issue list
agentra issue list --status in_progress
agentra issue list --priority urgent --assignee "Agent Name"
agentra issue list --limit 20 --output json
```

Available filters: `--status`, `--priority`, `--assignee`, `--limit`.

### Get Issue

```bash
agentra issue get <id>
agentra issue get <id> --output json
```

### Create Issue

```bash
agentra issue create --title "Fix login bug" --description "..." --priority high --assignee "Lambda"
```

Flags: `--title` (required), `--description`, `--status`, `--priority`, `--assignee`, `--parent`, `--due-date`.

### Update Issue

```bash
agentra issue update <id> --title "New title" --priority urgent
```

### Assign Issue

```bash
agentra issue assign <id> --to "Lambda"
agentra issue assign <id> --unassign
```

### Change Status

```bash
agentra issue status <id> in_progress
```

Valid statuses: `backlog`, `todo`, `in_progress`, `in_review`, `done`, `blocked`, `cancelled`.

### Comments

```bash
# List comments
agentra issue comment list <issue-id>

# Add a comment
agentra issue comment add <issue-id> --content "Looks good, merging now"

# Reply to a specific comment
agentra issue comment add <issue-id> --parent <comment-id> --content "Thanks!"

# Delete a comment
agentra issue comment delete <comment-id>
```

### Execution History

```bash
# List all execution runs for an issue
agentra issue runs <issue-id>
agentra issue runs <issue-id> --output json

# View messages for a specific execution run
agentra issue run-messages <task-id>
agentra issue run-messages <task-id> --output json

# Incremental fetch (only messages after a given sequence number)
agentra issue run-messages <task-id> --since 42 --output json
```

The `runs` command shows all past and current executions for an issue, including running tasks. The `run-messages` command shows the detailed message log (tool calls, thinking, text, errors) for a single run. Use `--since` for efficient polling of in-progress runs.

## Configuration

### View Config

```bash
agentra config show
```

Shows config file path, server URL, app URL, and default workspace.

### Set Values

```bash
agentra config set server_url wss://api.example.com/ws
agentra config set app_url https://app.example.com
agentra config set workspace_id <workspace-id>
```

## Other Commands

```bash
agentra version              # Show CLI version and commit hash
agentra update               # Update to latest version
agentra agent list           # List agents in the current workspace
```

## Output Formats

Most commands support `--output` with two formats:

- `table` — human-readable table (default for list commands)
- `json` — structured JSON (useful for scripting and automation)

```bash
agentra issue list --output json
agentra daemon status --output json
```
