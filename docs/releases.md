# Release History

## v0.4.1
_2026-07-04_

- `TaskService.CompleteTask` now calls `recordTaskMetric` — every task completion
  appends one row to `agent_task_metrics` with fire-and-forget 2s detached context.
- `TestRecordTaskMetric_FireAndForget` verifies the hook end-to-end.
- Migration `039_agent_task_metrics` FK fix: `issue` (singular) not `issues`.

## v0.4.0
_2026-07-04_

- **CLI:** new `agentra conventions` subcommand:
    - `init-agent-conventions` — scan repo signals, write `.agentra/AGENT.md`.
    - `validate-agent-conventions` — validate against spec.
- **Spec:** `docs/spec/agent-conventions.md` (8 fields, dual-language).

## v0.3.4
_2026-07-03_

- **Migration 039:** `agent_task_metrics` table + 4 indexes
  (workspace, provider, task_type, issue).
- **sqlc:** generated `InsertAgentTaskMetric`, `GetMetricsSummary`,
  `GetMetricsByTaskType`, `GetMetricsByIssue`.
- **cost tracking:** `NUMERIC(10,6)` column for per-task Spend.

## v0.3.3
_2026-07-03_

- Initial test release. Multi-platform CLI binaries via GoReleaser.

## v0.3.2
_Skipped_

## v0.3.1
_2026-07-03_

- Tag-only.

## v0.3.0
_2026-07-02_

- Tag-only. Triggered failed Release. First stable release is v0.3.3.

## v0.0.3 and earlier
_2026-03 → 2026-04_

- Original Agentra rebrand from internal tools.
- JWT auth, JWT_SECRET startup enforcement.
- Issue + comment + realtime WebSocket.
- Specialist agent templates (frontend/backend/test/security/devops/tech writer).
