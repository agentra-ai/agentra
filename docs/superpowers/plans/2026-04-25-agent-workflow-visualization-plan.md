# Agent Workflow Visualization — Implementation Plan

## Overview

**Feature:** Real-time Agent Workflow Visualization
**Spec:** `docs/superpowers/specs/2026-04-25-agent-workflow-visualization-design.md`
**Start:** 2026-04-25

## Task Breakdown

### Phase 1: Backend — WebSocket Events

#### 1.1 Add new event types

**Files:**
- `server/internal/events/events.go` — add `AgentStageEvent`, `StreamingLogEvent` types

**Steps:**
1. Define `AgentStage` enum: `reading`, `implementing`, `testing`, `committing`, `done`
2. Define `StreamingLogEvent` struct with `taskId`, `agentId`, `lines`, `timestamp`
3. Add to existing `WSEvent` union type

**Verification:** `go build ./server/...`

---

#### 1.2 Update Task Service to emit stage events

**File:** `server/internal/service/task.go`

**Steps:**
1. Find task lifecycle transitions (enqueue → claim → start → complete/fail)
2. Insert `agent:stage` event emissions at each transition:
   - `start` → `agent:working` + `agent:stage:reading`
   - When daemon starts executing → `agent:stage:implementing`
   - When tests start → `agent:stage:testing`
   - On commit → `agent:stage:committing`
   - On complete → `agent:stage:done`

**Verification:** `go test ./internal/service/...`

---

#### 1.3 Update Hub to broadcast streaming logs

**File:** `server/internal/realtime/hub.go`

**Steps:**
1. Add per-task subscription tracking
2. Implement log buffering (batch every 100ms or 50 lines)
3. Broadcast `streaming:logs` only to subscribers of the specific task

**Verification:** `go build ./server/...`

---

### Phase 2: Frontend — Types & Hooks

#### 2.1 Add WebSocket event types

**File:** `apps/web/shared/types/ws-events.ts` (or similar)

**Steps:**
1. Add `AgentStageEvent` interface
2. Add `StreamingLogEvent` interface
3. Add to `WSEvent` union type

**Verification:** `pnpm typecheck`

---

#### 2.2 Create `useStreamingLogs` hook

**File:** `apps/web/features/issues/hooks/useStreamingLogs.ts`

**Steps:**
1. Subscribe to WebSocket `streaming:logs` events
2. Maintain `logLines: string[]` state
3. Return `{ logLines, clearLogs }`

**Verification:** `pnpm test` (if test file exists)

---

#### 2.3 Create `useAgentStage` hook

**File:** `apps/web/features/issues/hooks/useAgentStage.ts`

**Steps:**
1. Subscribe to WebSocket `agent:stage` events
2. Maintain `stage` state
3. Also infer stage from log patterns (as fallback)
4. Return `{ stage, stageLabel }`

**Verification:** `pnpm typecheck`

---

### Phase 3: Frontend — Components

#### 3.1 Create `StageIndicator` component

**File:** `apps/web/features/issues/components/StageIndicator.tsx`

**Steps:**
1. Create badge-style component
2. Map stage to color/icon:
   - `reading` → blue, book icon
   - `implementing` → orange, code icon
   - `testing` → purple, test icon
   - `committing` → green, git icon
   - `done` → gray, check icon
3. Props: `stage`, `timestamp?`

**Verification:** `pnpm typecheck`

---

#### 3.2 Create `LiveTerminal` component

**File:** `apps/web/features/issues/components/LiveTerminal.tsx`

**Steps:**
1. Create terminal-style container with dark background
2. Use `useStreamingLogs` hook
3. Auto-scroll to bottom on new logs
4. Show stage indicator at top
5. Collapsible panel

**Verification:** `pnpm typecheck`

---

#### 3.3 Integrate into Issue Detail

**File:** `apps/web/features/issues/components/IssueDetail.tsx` (or page)

**Steps:**
1. Add `StageIndicator` next to issue status
2. Add `LiveTerminal` panel below description
3. Conditionally show when agent is assigned and active

**Verification:** Manual browser test

---

### Phase 4: Integration Testing

#### 4.1 Manual E2E test

**Steps:**
1. Start backend + frontend
2. Create issue, assign to agent
3. Observe in browser:
   - Stage indicator appears within 2s
   - Logs stream in real-time
   - Stage transitions as agent works
   - Issue status updates on completion

---

## File Summary

### Backend (Go)
| File | Changes |
|------|---------|
| `server/internal/events/events.go` | New event types |
| `server/internal/service/task.go` | Emit stage events |
| `server/internal/realtime/hub.go` | Per-task subscription, log buffering |

### Frontend (TypeScript)
| File | Changes |
|------|---------|
| `shared/types/ws-events.ts` | New event interfaces |
| `features/issues/hooks/useStreamingLogs.ts` | New hook |
| `features/issues/hooks/useAgentStage.ts` | New hook |
| `features/issues/components/StageIndicator.tsx` | New component |
| `features/issues/components/LiveTerminal.tsx` | New component |
| `features/issues/components/IssueDetail.tsx` | Integrate new components |

---

## Dependencies

- Backend Phase 1 must complete before Frontend Phase 2
- Frontend Phase 2 must complete before Phase 3

---

## Estimated Tasks

1. Backend: Add event types
2. Backend: Emit stage events in task service
3. Backend: Update hub for streaming logs
4. Frontend: Add types
5. Frontend: Create useStreamingLogs hook
6. Frontend: Create useAgentStage hook
7. Frontend: Create StageIndicator component
8. Frontend: Create LiveTerminal component
9. Frontend: Integrate into Issue Detail
10. E2E: Manual verification
