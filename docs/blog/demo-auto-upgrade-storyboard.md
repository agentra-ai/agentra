# Demo Storyboard — Daemon Auto-Upgrade in 90 Seconds

> Issue [#8](https://github.com/agentra-ai/agentra/issues/8). Shot list for a 90-second demo video.

## Pre-conditions

- Agentra server running at `http://localhost:8080` (or staging), **v0.3.0** deployed
- One daemon running on local machine at a **previous release** (must be an embedded-version binary from GoReleaser, not `go run` which reports `dev`)
- Agent assigned to a real task visible in the web app (so task-continuity via AgentLiveCard is demonstrable)
- `agentra` CLI installed locally for text narration
- Github Release **v0.3.0** published with `agentra_darwin_arm64.tar.gz` + other assets (release.yml builds these)
- **Trigger mechanism**: Frontend Settings → Runtimes → [runtime] → "Trigger update" button (member role+). This calls `PUT /api/runtimes/{runtimeId}/update` with `{"target_version": "0.3.0"}` and writes into the server's in-memory UpdateStore.

## Shot List

| Time | Visual | Narration (EN) | Narration (ZH) |
|------|--------|----------------|----------------|
| 0:00–0:08 | Terminal: `agentra daemon status --output json` → shows previous version | "This daemon is running an older release. I'll push a new release from the server — no SSH, no manual download, no touching the machine." | "Daemon 运行旧版本。我从服务器远程发一个 release —— 无 SSH、无手动下载、不碰机器。" |
| 0:08–0:15 | Web app: Settings → Runtimes → click "Trigger update" → modal "Target: v0.3.0" → confirm | "I click 'Trigger update' in the web UI. The server stages a pending update." | "我在 Web 端点击 Trigger update。服务端标记 PendingUpdate。" |
| 0:15–0:25 | Terminal: daemon logs (`agentra daemon logs -f`) show heartbeat received `pending_update`, `UpdateViaDownload` kicking in | "On the next heartbeat, the daemon sees the update request and downloads the new binary from GitHub Releases." | "下一次心跳，Daemon 看到更新请求，从 GitHub Releases 下载新二进制。" |
| 0:25–0:35 | Terminal: process switches — old PID exits (graceful `cancelFunc()`), new PID replaces it via `triggerRestart()` | "Old process exits gracefully. New one picks up — ~3 seconds of downtime." | "旧进程优雅退出，新进程接管 —— 共计约 3 秒中断。" |
| 0:35–0:50 | Web app: AgentLiveCard continues streaming tool_use events — **no task loss** (daemon rehydrates from DB `WHERE status='started'`) | "Notice the task I assigned earlier? Still streaming. The new daemon picked up where the old one left off." | "注意我之前分配的任务？仍在流式传输。新 daemon 从旧 daemon 的地方接手。" |
| 0:50–1:10 | Terminal: `agentra daemon status --output json` now shows v0.3.0, uptime accounting continuous, active tasks count unchanged | "Status confirms v0.3.0, continuous uptime, all tasks still active." | "状态确认 v0.3.0，运行时间连续累计，所有任务仍在活跃。" |
| 1:10–1:20 | Speed-montage: brew variant (optional) | "`brew upgrade agentra` works the same way — symlink preserved." | "`brew upgrade agentra` 同样流程 —— 符号链接被保留。" |
| 1:20–1:30 | Fade to GitHub Releases v0.3.0 asset list + README embed | "The whole loop: release on GitHub, daemon does the rest. This is zero-ops agent runtime management." | "完整闭环：GitHub 发布，Daemon 完成剩余工作。这就是零运维 agent 运行时管理。" |

## Technical Notes

- **Camera**: OBS Studio, 1920×1080, 30fps, lossless capture of terminal + browser side-by-side
- **Terminal theme**: Dark theme for contrast, 14pt font, iTerm2 default
- **Browser**: Arc / Chrome with devtools hidden, Agentra web app logged in as demo user
- **Audio**: lapel mic, room tone minimal, under-bed music track
- **Duration cap**: 90 seconds hard limit; additional "deep dive" content can follow as bonus

## Publishing Checklist

- [ ] Record at 1080p, export as H.264 MP4
- [ ] Upload to GitHub Releases v0.2.2 asset (so the video ships with the release)
- [ ] Embed link into README.md "Quick Start" section
- [ ] Cross-post to X / Hacker News / Reddit with caption "this is what zero-ops agent management looks like"

---

*Companion doc: [auto-upgrade.md](../self-hosting/auto-upgrade.md) for the raw architecture.*
