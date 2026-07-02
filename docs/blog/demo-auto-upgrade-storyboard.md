# Demo Storyboard — Daemon Auto-Upgrade in 90 Seconds

> Issue [#8](https://github.com/agentra-ai/agentra/issues/8). Shot list for a 90-second demo video.

## Pre-conditions

- Agentra server running at `http://localhost:8080` (or staging)
- One daemon running on local machine (slightly outdated version, e.g. v0.2.1)
- Agent assigned to a real task visible in the web app (so task-continuity is demonstrable)
- `agentra` CLI installed locally for text narration

## Shot List

| Time | Visual | Narration (EN) | Narration (ZH) |
|------|--------|----------------|----------------|
| 0:00–0:08 | Terminal: `agentra daemon status` showing v0.2.1 | "This daemon is running version 0.2.1. Let's watch it update itself — no SSH, no manual download." | "Daemon 正运行 v0.2.1。接下来看它自动升级——无需 SSH、无需手动下载。" |
| 0:08–0:15 | Server-side mark new release v0.2.2 as PendingUpdate (admin API or GitHub Release publish) | "I'll publish v0.2.2 as a pending update from the server." | "服务端推送 v0.2.2 为 PendingUpdate。" |
| 0:15–0:25 | Terminal: daemon logs show heartbeat received update request, downloading new binary | "On the next heartbeat, the daemon sees the instruction and pulls the new binary." | "下一次心跳，Daemon 感知到并开始下载新二进制。" |
| 0:25–0:35 | Terminal: status flips to v0.2.2, old process exits, new process starts | "Old process exits. New one picks up — 3 seconds of downtime." | "旧进程退出，新进程接管——总计 3 秒中断。" |
| 0:35–0:50 | Web app: AgentLiveCard still streaming for the in-flight task | "The task I assigned earlier? Still running. No interruption." | "之前分配的任务？仍在运行，未被中断。" |
| 0:50–1:10 | Terminal: `agentra daemon status --output json` showing new version, uptime, active tasks | "Status confirms the new version, continuous uptime accounting." | "状态确认新版本，运行时间智能累计。" |
| 1:10–1:25 | Speed-montage: brew install path variant (optional second take) | "Brew users follow the same flow — symlink preserved." | "Brew 用户流程一致——符号链接被保留。" |
| 1:25–1:30 | Fade to GitHub Releases page showing v0.2.2 asset list | "That's the whole loop. Release on GitHub; the daemon does the rest." | "完整闭环：GitHub 发布，Daemon 自行完成剩余工作。" |

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
