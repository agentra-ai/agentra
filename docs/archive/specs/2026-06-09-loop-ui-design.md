# Loop UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add web UI to start/manage Agentic Engineering Loops from the issue detail page + a dedicated loops list page.

**Architecture:** New `features/loops/` module (zustand store + API client + components) following the existing feature-based pattern. Two new routes (`/loops`, `/loops/[id]`). A "Run Loop" button on the issue detail page opens a dialog that POSTs to `/api/loops`. WebSocket events keep both views in sync.

**Tech Stack:** Next.js 16 (App Router), React, zustand, shadcn/ui, existing `WSClient` + `useWSEvent`.

---

## API Contract (already implemented on server)

| Method | Path | Body | Returns |
|---|---|---|---|
| `POST` | `/api/loops` | `{ issue_id, agent_id, max_iterations? }` | `Loop` |
| `GET` | `/api/loops` | — | `{ loops: Loop[] }` |
| `GET` | `/api/loops/:id` | — | `Loop` |
| `POST` | `/api/loops/:id/pause` | — | `Loop` |
| `POST` | `/api/loops/:id/resume` | — | `Loop` |
| `POST` | `/api/loops/:id/cancel` | — | `Loop` |

`Loop` shape (from `server/internal/loop/loop.go`):
```ts
type LoopStatus = "pending" | "running" | "paused" | "done" | "failed" | "cancelled";
type LoopStage = "plan" | "develop" | "review" | "fix";

interface Loop {
  id: string;
  issue_id: string;
  workspace_id: string;
  status: LoopStatus;
  current_stage?: LoopStage;
  iteration: number;
  max_iterations: number;
  pr_url?: string;
  pr_number?: number;
  branch_name?: string;
  agent_id?: string;
  failure_reason?: string;
  started_at?: string;  // ISO
  completed_at?: string; // ISO
  created_at: string;   // ISO
  updated_at: string;   // ISO
}
```

WebSocket events (already broadcast by the loop coordinator — see `server/internal/realtime/hub.go`): `loop:created`, `loop:updated`, `loop:stage_changed`, `loop:completed`, `loop:failed`. Payload contains the full `Loop` object.

---

## File Structure

```
apps/web/
├── app/(dashboard)/
│   └── loops/
│       ├── page.tsx                    # NEW — list page (thin shell)
│       └── [id]/
│           └── page.tsx                # NEW — detail page (thin shell)
├── features/loops/                     # NEW MODULE
│   ├── index.ts                        # public exports
│   ├── store.ts                        # zustand store
│   ├── api.ts                          # REST client methods (or extend shared/api)
│   ├── components/
│   │   ├── index.ts
│   │   ├── loops-page.tsx              # list view
│   │   ├── loop-detail-page.tsx        # detail view
│   │   ├── loop-list-row.tsx           # one row in list
│   │   ├── start-loop-dialog.tsx       # start dialog with agent picker
│   │   ├── loop-status-badge.tsx       # status pill
│   │   └── loop-stage-indicator.tsx    # plan → develop → review → fix
│   └── hooks.ts                        # useLoops, useLoop, useStartLoop, useLoopTransition
├── shared/types/
│   └── loop.ts                         # NEW — Loop, LoopStatus, LoopStage types
├── shared/api/
│   ├── client.ts                       # MODIFY — add loop methods (or use a new file)
│   └── loops.ts                        # NEW — loop-specific REST methods
├── features/issues/components/
│   └── issue-detail.tsx                # MODIFY — add "Run Loop" button
└── messages/
    ├── en.json                         # MODIFY — add `loops` namespace
    └── zh-CN.json                      # MODIFY — add `loops` namespace
```

---

## Tasks

### Task 1: Shared types

**Files:** Create `apps/web/shared/types/loop.ts`

Export the `Loop`, `LoopStatus`, `LoopStage` types matching the server shape (see API Contract above). Single source of truth — both `features/loops/` and `shared/api/loops.ts` import from here.

### Task 2: API client methods

**Files:** Create `apps/web/shared/api/loops.ts`

Pure functions wrapping `api.post()` / `api.get()`. Six methods, one per endpoint. Each returns a `Promise<Loop>` (or `Promise<{ loops: Loop[] }>` for list). Use the existing `ApiClient` from `@/shared/api` — do not introduce a new HTTP layer.

- [ ] `listLoops(): Promise<Loop[]>`
- [ ] `getLoop(id: string): Promise<Loop>`
- [ ] `startLoop(input: { issue_id: string; agent_id: string; max_iterations?: number }): Promise<Loop>`
- [ ] `pauseLoop(id: string): Promise<Loop>`
- [ ] `resumeLoop(id: string): Promise<Loop>`
- [ ] `cancelLoop(id: string): Promise<Loop>`

### Task 3: Zustand store

**Files:** Create `apps/web/features/loops/store.ts`

One store with:
- `loops: Record<string, Loop>` (id → loop)
- `loopIds: string[]` (insertion order)
- `loading: boolean`, `error: string | null`
- Actions: `setLoops`, `upsertLoop`, `removeLoop`, `setLoading`, `setError`

Follow the existing pattern in `features/issues/store.ts`. Do not call `useRouter` or hooks inside the store.

### Task 4: Hooks

**Files:** Create `apps/web/features/loops/hooks.ts`

- `useLoops()` — returns sorted loop list, fetches on mount if empty, subscribes to `loop:created` / `loop:updated` to keep store fresh
- `useLoop(id)` — returns single loop, fetches on mount, subscribes to events for that id
- `useStartLoop()` — returns `({ issue_id, agent_id, max_iterations }) => Promise<Loop>`, invalidates list on success
- `useLoopTransition()` — returns `{ pause, resume, cancel }` bound to current loop id, invalidates on success

Use `useWSEvent` from `@/features/realtime` for subscriptions. Follow `useWSEvent` usage in `features/issues/components/agent-live-card.tsx` as a reference.

### Task 5: Components — badges and indicators

**Files:**
- `features/loops/components/loop-status-badge.tsx` — colored pill for status (shadcn `Badge`)
- `features/loops/components/loop-stage-indicator.tsx` — horizontal stepper: plan → develop → review → fix. Highlight current stage. Reuse the visual language of `features/issues/components/stage-indicator.tsx` but for the loop stages.

Use shadcn tokens (`bg-primary`, `text-muted-foreground`, etc.) — no hardcoded colors.

### Task 6: Component — start dialog

**Files:** `features/loops/components/start-loop-dialog.tsx`

Props: `{ open, onOpenChange, issueId, onSuccess?: (loop: Loop) => void }`.

Form fields:
- Agent (required) — `Select` populated from `useWorkspaceStore` agent list (find the existing accessor in `features/workspace/`)
- Max iterations (optional, default 5) — number input, 1-10
- Submit button: "Start Loop" — disabled while submitting
- On success: close dialog, call `onSuccess(loop)`

Use shadcn `Dialog`, `Select`, `Input`, `Button`. Validate: agent must be selected; show inline error if not.

### Task 7: Component — list page

**Files:** `features/loops/components/loops-page.tsx`

Render `useLoops()` as a table:
- Columns: ID (truncated), Issue ID (truncated, link to issue page), Status badge, Stage indicator, Iteration (e.g. `2/5`), PR (link if `pr_url`), Created
- Empty state: "No loops yet" with hint to start one from an issue
- Header: "Loops" title + refresh button (re-fetches via store action)
- Subscribe to `loop:created` / `loop:updated` events so list updates in real time

### Task 8: Component — detail page

**Files:** `features/loops/components/loop-detail-page.tsx`

Renders a single loop:
- Header: status badge + stage indicator + iteration counter
- Info grid: issue link, branch name, PR link, agent, started/completed timestamps
- Action buttons (gated by status):
  - `running` → Pause, Cancel
  - `paused` → Resume, Cancel
  - `pending` → Cancel
  - `done` / `failed` / `cancelled` → none (read-only)
  - Failure reason shown as a destructive callout if `failure_reason` present
- Subscribes to `loop:updated` for live refresh

### Task 9: Routes

**Files:**
- Create `app/(dashboard)/loops/page.tsx` — thin shell: `export default function Page() { return <LoopsPage />; }`
- Create `app/(dashboard)/loops/[id]/page.tsx` — thin shell: `export default function Page({ params }: { params: { id: string } }) { return <LoopDetailPage id={params.id} />; }`

### Task 10: Public exports

**Files:** Create `features/loops/index.ts` exporting the public surface:
- Components: `LoopsPage`, `LoopDetailPage`, `StartLoopDialog`, `LoopStatusBadge`, `LoopStageIndicator`
- Hooks: `useLoops`, `useLoop`, `useStartLoop`, `useLoopTransition`
- Types re-exported from `@/shared/types/loop`

### Task 11: Wire into issue detail page

**Files:** Modify `apps/web/features/issues/components/issue-detail.tsx`

Add a "Run Loop" button in the action area (next to status/assignee pickers). On click, open `StartLoopDialog` with `issueId={issue.id}`. On success, navigate to `/loops/:id` (or show a toast with a link — pick the cleaner option).

Gate the button: only show for issues in `todo` or `in_progress` status (no point looping a `done` issue).

### Task 12: i18n

**Files:** Modify `apps/web/messages/en.json` and `apps/web/messages/zh-CN.json`

Add a new top-level `loops` namespace. Minimum keys:
- `title`: "Loops" / "循环"
- `startLoop`: "Run Loop" / "运行循环"
- `status.*` for each of the 6 statuses
- `stage.*` for each of the 4 stages
- `actions.pause/resume/cancel`
- `empty`: "No loops yet" / "暂无循环"
- `dialog.title`: "Start Agentic Loop" / "启动工程循环"
- `dialog.agent`: "Agent" / "代理"
- `dialog.maxIterations`: "Max iterations" / "最大迭代次数"
- `dialog.submit`: "Start Loop" / "启动循环"

Reuse `common.cancel` / `common.close` where possible.

### Task 13: Tests

Add Vitest unit tests for the pure logic — no React component tests required for v1:
- `features/loops/store.test.ts` — `upsertLoop` adds new and updates existing; `setLoops` resets state
- `shared/api/loops.test.ts` — verify the six methods call the right paths with the right bodies (mock `api.get` / `api.post`)

Mock `api` via the same pattern used in existing tests. Run `pnpm --filter @agentra/web exec vitest run apps/web/features/loops apps/web/shared/api/loops.test.ts`.

### Task 14: Typecheck + lint

Run `pnpm typecheck` and `pnpm lint` from the project root. Fix all errors before considering the task done. Do not silence TypeScript with `any` — if a type is unclear, ask.

---

## Conventions (from CLAUDE.md — follow strictly)

- **No `useState` for shared state.** Loops state goes in the zustand store.
- **One store per feature.** This is a new feature — `features/loops/store.ts` is the only store.
- **No `useRouter` in stores.** Navigation happens in components.
- **Use shadcn components.** Install missing ones with `npx shadcn add` (likely candidates: `dialog`, `select` — check if already present).
- **No hardcoded colors.** `bg-primary`, `text-muted-foreground`, `text-destructive` only.
- **i18n is bilingual.** Every user-facing string lives in both `en.json` and `zh-CN.json`.
- **No backwards-compat shims.** This is greenfield UI; no legacy code to preserve.
- **WebSocket subscriptions belong in hooks/components**, not stores. Stores receive upserts from event handlers; the subscription itself is `useWSEvent` in `useLoops` / `useLoop`.

---

## Out of scope (do NOT build)

- Loop creation from a non-issue context (e.g. "new loop" global page) — v1 only triggers from issue detail
- Live log streaming / terminal view of in-progress loop — `LiveTerminal` already exists for the per-task terminal; the loop is a higher-level concept
- Loop template / config JSON editor — POST body only has the 3 documented fields
- Cross-workspace loop view — workspace is implicit
- Pause/resume confirmation modals — buttons fire immediately, with a destructive variant for cancel
