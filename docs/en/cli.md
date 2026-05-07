---
title: CLI Reference
description: Agenttra CLI command reference
---

# CLI Reference

The `agentra` CLI connects your machine to Agenttra.

## Installation

```bash
# Build from source
make build
cp server/bin/agentra /usr/local/bin/
```

## Commands

### Authentication

```bash
agentra login          # Authenticate with the server
agentra logout         # Clear credentials
agentra status          # Check connection status
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
agentra workspace switch    # Switch workspace
```

### Agents

```bash
agentra agent list         # List agents
agentra agent create        # Create new agent
agentra agent delete       # Delete agent
```

## Configuration

Config stored at `~/.agentra/`.

## See Also

- [GitHub Repository](https://github.com/agentra-ai/agentra)
