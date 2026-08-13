---
title: CLI Reference
description: Agentra CLI command reference
---

# CLI Reference

The `agentra` CLI connects your machine to Agentra.

## Installation

```bash
# macOS
brew install --cask agentra-ai/tap/agentra

# macOS or Linux (download first so you can review it)
curl -fsSLO https://raw.githubusercontent.com/agentra-ai/agentra/main/scripts/install.sh
sh install.sh
rm install.sh
```

```powershell
# Windows PowerShell
Invoke-WebRequest https://raw.githubusercontent.com/agentra-ai/agentra/main/scripts/install.ps1 -OutFile install.ps1
powershell -ExecutionPolicy Bypass -File .\install.ps1
Remove-Item .\install.ps1
```

The scripts verify release archives against `checksums.txt`, install without automatically elevating privileges, and run `agentra version`. Tagged releases also provide a Cosign keyless bundle for the checksum file, signed SPDX SBOMs, and GitHub provenance; follow the [release verification guide](../DEPLOYMENT.md#verify-a-cli-release) to verify the expected workflow identity.

## Commands

### Guided setup

```bash
agentra setup
agentra setup --server-url https://api.example.com --app-url https://app.example.com
```

`setup` checks API readiness and the Web app, verifies that `claude`, `codex`, or `opencode` is available for a local runtime, authenticates, discovers workspaces, and starts the daemon. It defaults to the loopback ports published by Compose. Use `--token` for PAT login, `--no-daemon` for management-only machines, and `--profile` to isolate deployments.

### Authentication

```bash
agentra login          # Authenticate with the server
agentra auth logout    # Clear credentials
agentra auth status    # Check authentication status
```

### Daemon

```bash
agentra daemon start   # Start the agent daemon
agentra daemon stop    # Stop the daemon
agentra daemon status  # Check daemon status
```

### Workspaces

```bash
agentra workspace list      # List workspaces
agentra workspace watch     # Add a workspace to the local daemon
agentra workspace unwatch   # Remove a workspace from the local daemon
```

### Agents

```bash
agentra agent list         # List agents
agentra agent create       # Create an agent
agentra agent archive      # Archive an agent
```

### Installation diagnostics

Run this before starting the daemon or when an installation stops working:

```bash
agentra doctor
agentra doctor --repo /path/to/repository
agentra doctor --output json
agentra doctor --skip-repo-remote   # Offline/local-only repository check
```

`doctor` checks the active profile's config permissions, Web and API reachability, server readiness, object storage, authentication, workspace access, supported runtime CLIs, daemon workspace permissions, Git origin access, local daemon, and authenticated WebSocket ping/pong. Checks run concurrently and each external operation defaults to a five-second timeout; override it with `--timeout`.

The command exits non-zero when a required check fails. Warnings such as an intentionally disabled object store, missing local daemon, or a directory without a Git worktree do not cause a failing exit. JSON output uses the versioned `schema_version: 1` contract and never includes the access token. The WebSocket probe sends PATs in the `Authorization` header rather than the URL.

## Configuration

The default profile is stored at `~/.agentra/config.json`; named profiles use `~/.agentra/profiles/<name>/config.json`. Config files containing access tokens should be owner-only (`0600` on macOS/Linux).

## See Also

- [GitHub Repository](https://github.com/agentra-ai/agentra)
