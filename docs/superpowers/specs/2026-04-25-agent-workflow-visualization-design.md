# Agent Workflow Visualization — Design Spec

## Overview

**Project:** Agenttra
**Feature:** Real-time Agent Workflow Visualization
**Date:** 2026-04-25
**Status:** Approved for implementation

## Problem

Agent 执行时 board 一片黑箱，只有完成那一刻状态才突变。用户无法看到 agent 在做什么、进展如何、是否正常执行。

## Goals

1. **实时日志流** — 用户能像看 CI pipeline 一样，实时看到 agent 的每一步操作
2. **阶段状态展示** — agent 工作时显示当前阶段（Reading / Implementing / Testing / Committing）
3. **进度感知** — 用户知道 agent 在工作而不是卡住

---

## Architecture

```
Agent Daemon                  Backend                     Frontend
    │                           │                            │
    │─ task:start ────────────► │                            │
    │─ agent:working ─────────► │ ── WS broadcast ─────────► │  显示 "Agent is working..."
    │─ log:line ──────────────► │ ── WS broadcast ─────────► │  Live terminal 滚动日志
    │─ agent:stage ───────────► │                            │
    │─ log:line ──────────────► │                            │
    │─ task:done ────────────► │ ── WS broadcast ─────────► │  Issue 状态更新
```

---

## WebSocket Events

### New Event Types

```typescript
// shared/types/ws-events.ts

type WSEventType =
  | 'task:started'      // agent 开始处理
  | 'agent:working'     // agent 正在工作（泛化状态）
  | 'agent:stage'       // agent 阶段变化
  | 'streaming:logs'    // 实时日志行
  | 'task:completed';   // 任务完成

interface StreamingLogEvent {
  type: 'streaming:logs';
  taskId: string;
  agentId: string;
  lines: string[];      // 日志行数组
  timestamp: string;
}

interface AgentStageEvent {
  type: 'agent:stage';
  taskId: string;
  agentId: string;
  stage: 'reading' | 'implementing' | 'testing' | 'committing' | 'done';
  timestamp: string;
}
```

### Existing Events (to be reused)

- `task:started` — already exists
- `task:completed` — already exists (maps to done)

---

## Frontend Components

### LiveTerminal

Location: `apps/web/features/issues/components/LiveTerminal.tsx`

**Props:**
```typescript
interface LiveTerminalProps {
  taskId: string;
  agentId: string;
}
```

**Behavior:**
- Subscribes to `streaming:logs` events for this task
- Displays log lines in real-time, auto-scrolls to bottom
- Shows stage indicator badge at top
- Similar to CI pipeline log view

### StageIndicator

Location: `apps/web/features/issues/components/StageIndicator.tsx`

**Props:**
```typescript
interface StageIndicatorProps {
  stage: 'reading' | 'implementing' | 'testing' | 'committing' | 'done' | 'idle';
  timestamp?: string;
}
```

**Visual:**
- Badge style with icon
- Color coded by stage

### Issue Detail Page Integration

File: `apps/web/features/issues/components/IssueDetail.tsx` (or split into sub-components)

Layout:
```
Issue Detail
├── Header
│   ├── IssueStatusBadge (updated to show agent stage)
│   └── StageIndicator
├── Description
├── LiveTerminal (new, collapsible panel)
└── Comments
```

---

## Stage Inference

Frontend infers stage from log content patterns:

| Pattern Keywords | Stage |
|-----------------|-------|
| "Reading" / "Loading spec" / "Fetching" | `reading` |
| "Implementing" / "Writing" / "Creating" / "Modifying" | `implementing` |
| "Running" / "Testing" / "Test" | `testing` |
| "Committing" / "git commit" / "Pushing" | `committing` |
| "Complete" / "Done" / "Success" | `done` |

Daemon also sends explicit `agent:stage` events for reliable state changes.

---

## Backend Changes

### 1. Task Service (Go)

File: `server/internal/service/task.go`

- Emit `agent:stage` events at meaningful transitions
- Stream log output via existing event bus

### 2. WebSocket Hub (Go)

File: `server/internal/realtime/hub.go`

- Ensure `streaming:logs` events are broadcast to all subscribers for a workspace
- Rate limit log events to prevent flooding (batch every 100ms)
- Only broadcast to clients subscribed to the specific task (per-task subscription)

### 3. Log Buffering

Daemon buffers logs and flushes every 100ms or when buffer reaches 50 lines, whichever comes first.

---

## Files to Create/Modify

### Frontend (TypeScript)

| File | Action | Description |
|------|--------|-------------|
| `features/issues/components/LiveTerminal.tsx` | Create | Real-time log viewer component |
| `features/issues/components/StageIndicator.tsx` | Create | Stage badge component |
| `features/issues/hooks/useAgentStage.ts` | Create | Hook to track agent stage for a task |
| `features/issues/hooks/useStreamingLogs.ts` | Create | Hook to subscribe to log stream |
| `features/issues/components/IssueDetail.tsx` | Modify | Add LiveTerminal and StageIndicator |
| `shared/types/ws-events.ts` | Modify | Add new event types |

### Backend (Go)

| File | Action | Description |
|------|--------|-------------|
| `internal/service/task.go` | Modify | Emit stage events |
| `internal/realtime/hub.go` | Modify | Handle streaming:logs broadcast |
| `pkg/events/events.go` | Modify | Add new event types |

---

## Testing

1. **Unit tests** for stage inference logic
2. **Integration tests** for WS event flow
3. **Manual verification** — run agent on a task, observe live terminal

---

## Out of Scope (for MVP)

- Log persistence (stored in DB for later review)
- Log search/filter
- Agent pause/resume
- Multiple agents on same task
- Backward replay of logs

---

## Success Criteria

1. When agent is assigned a task, user sees "Agent is working..." within 2 seconds
2. Log lines appear in LiveTerminal within 500ms of being generated
3. Stage transitions are visible and accurate
4. When agent completes, issue status updates automatically
