# Daemon Auto-Upgrade via Heartbeat

> How the Agentra daemon self-updates without operator babysitting — and what it looks like in 90 seconds.

## TL;DR

The daemon sends a **heartbeat** every N seconds (default: 60s). The heartbeat response can include a `PendingUpdate` instruction. When one arrives, the daemon:

1. Downloads the new binary via `cli.UpdateViaDownload(targetVersion)`
2. Reports status (`running` → `completed`) back to the server
3. Calls `triggerRestart()` → resolves the new binary path → `cancelFunc()` to gracefully shutdown
4. The foreground launcher (`cmd_daemon.go` ~line 309) detects `RestartBinary() != ""` and re-exec with the new binary
5. The old process exits; the new process picks up state and continues polling

**Total downtime: typically 2–5 seconds. Tasks in progress are NOT interrupted — the new daemon rehydrates from the database.**

## Architecture

```
┌──────────────┐                    ┌──────────────┐
│   Server     │  ← heartbeat ←    │   Daemon     │
│  (Go + Chi)  │  → PendingUpdate →│   (Go)       │
└──────────────┘                    └──────┬───────┘
                                           │
                              handleUpdate()│
                                           ▼
                              cli.UpdateViaDownload(version)
                                           │
                                           ▼
                              triggerRestart()
                              └─ resolve binary path
                              └─ skip symlink eval for brew
                              └─ cancelFunc() → graceful shutdown
                                           │
                                           ▼
                              cmd_daemon foreground loop detects
                              d.RestartBinary() != ""
                                           │
                                           ▼
                              exec.Command(newBin, buildDaemonStartArgs(...))
                              └─ child.Start()
                              └─ old process exits
```

## Key Files

| File | Responsibility |
|------|---------------|
| `server/internal/daemon/daemon.go:436-467` | `heartbeatLoop` — polls all runtimes, dispatches `PendingUpdate` to `handleUpdate` |
| `server/internal/daemon/daemon.go:543-578` | `handleUpdate` — downloads new binary, reports status |
| `server/internal/daemon/daemon.go:585-606` | `triggerRestart` — resolves binary, schedules shutdown |
| `server/cmd/agentra/cmd_daemon.go:304-334` | restart launch — child process spawn with new binary |
| `server/internal/cli/update.go` | `UpdateViaDownload` — GitHub Releases fetch + binary replace |
| `server/internal/cli/update.go` | `IsBrewInstall` — homebrew symlink detection |

## Configuring the upgrade channel

Heartbeat interval (how often the daemon checks for updates):

```bash
agentra daemon start --heartbeat-interval 60s   # default
```

Server-side: the operator pushes a new GitHub Release (via `release.yml` + GoReleaser). The server marks the release as a `PendingUpdate` for each registered runtime.

## Task continuity

When the daemon restarts:

- **In-flight tasks**: the DB record stays in `started` status. The new daemon's `rehydrateTasks()` picks them up from `WHERE status = 'started'` and resumes polling their agent session.

- **In-memory state**: ring-buffer history (AgentLiveCard) is **not** persisted — it's a UI-side cache. After restart, the live timeline starts empty, but `issue_comments` and `execution_traces` are queryable from the DB.

- **Agent sessions**: each agent (Claude Code / Codex) may still be running their CLI process. The daemon uses the task's `agent_session_id` to reconnect to the live session.

### What operators should know

1. **No SSH required** — the daemon updates itself. No SSH, no Ansible, no SSH keys.
2. **Brew users get it for free** — the symlink path is preserved; `brew upgrade agentra` works without touching the daemon config.
3. **Zero-downtime for the user** — users see a ~2s gap in WebSocket reconnect; client-side `useRealtimeSync` automatically rehydrates.
4. **Rollback is manual** — to revert, run `agentra daemon stop && brew switch agentra <old-version>` (brew) or download the previous release from GitHub Releases.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Daemon stuck on old version | Heartbeat interval too long | Check `agentra daemon status --output json` for `last_heartbeat_at` |
| Task shows `started` forever after restart | Agent CLI crashed; session unreachable | `agentra daemon logs -f` for error; re-assign the task to re-execute |
| `handleUpdate` fails: "download update failed" | Network egress blocked; GitHub Releases unreachable | Outbound HTTPS to `api.github.com` and `objects.githubusercontent.com` required |
| New daemon fails to start | Binary permission denied | Ensure the binary is `+x` (GoReleaser sets 0755) |

## Security

- Downloads come only from `github.com/agentra-ai/agentra/releases` — hard-coded host.
- No signature verification yet (planned Q3-2026 — see PROTOCOL.md).
- Daemon only updates on **heartbeat response** — no server push, no websocket command. Pull-only model.

---

*This document accompanies issue [#8](https://github.com/agentra-ai/agentra/issues/8) from the observability-triangle roadmap.*
