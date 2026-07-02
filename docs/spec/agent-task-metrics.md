# Agent Task Metrics — Telemetry Schema

> Backend-only telemetry for cross-provider success rate measurement.
> Implements issue [#11](https://github.com/agentra-ai/agentra/issues/11).
> **Torvalds guardrail**: this is a backend pipe, NOT a product UI. No frontend work in v0.

## Problem

We cannot improve what we cannot measure. Today, Agentra has no structured way to answer:

- Which provider resolves which task type most reliably?
- What's the per-task cost across providers?
- Are agents silently regressing after a model upgrade?

## Solution

A single append-only table `agent_task_metrics` that records one row per task completion. No PII, no code content — only structural signals.

## Schema

```sql
CREATE TABLE agent_task_metrics (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL REFERENCES workspaces(id),
  task_id         UUID NOT NULL REFERENCES agent_task_queue(id),
  issue_id        UUID NOT NULL REFERENCES issues(id),

  -- Provider identity
  provider        TEXT NOT NULL,                 -- 'anthropic' | 'openai' | 'ollama' | ...
  model           TEXT NOT NULL,                 -- 'claude-opus-4-7' | 'gpt-5.4' | ...
  runtime_type    TEXT NOT NULL DEFAULT 'local', -- 'local' | 'cloud'

  -- Task classification
  task_type       TEXT NOT NULL,                 -- 'feature' | 'bug' | 'refactor' | 'test' | 'docs'
  issue_priority  TEXT NOT NULL,                 -- 'urgent' | 'high' | 'medium' | 'low'

  -- Outcome
  status          TEXT NOT NULL,                 -- 'completed' | 'failed' | 'cancelled' | 'timeout'
  error_category  TEXT,                          -- 'context_filter' | 'timeout' | 'pr_failed' | 'test_failed' | NULL

  -- Performance
  duration_ms     BIGINT NOT NULL,
  token_input     INTEGER,
  token_output    INTEGER,
  cost_usd        NUMERIC(10,6),

  -- Timestamps
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Read patterns
CREATE INDEX idx_metrics_workspace_time  ON agent_task_metrics(workspace_id, created_at DESC);
CREATE INDEX idx_metrics_provider        ON agent_task_metrics(provider, model, created_at DESC);
CREATE INDEX idx_metrics_task_type       ON agent_task_metrics(task_type, status, created_at DESC);
CREATE INDEX idx_metrics_issue           ON agent_task_metrics(issue_id);
```

## Write Path

```
TaskService.CompleteTask()
  └─► metrics := buildMetricRecord(task, session)
  └─► ctx, cancel := context.WithTimeout(DetachedContext, 2s)
  └─► defer cancel()
  └─► go queries.InsertAgentTaskMetrics(ctx, metrics)  // fire-and-forget, non-blocking
```

**Critical**: metric write must NEVER block the task completion path. If the insert fails, log a warning and move on.

## Read Path (internal only)

```sql
-- Provider reliability by task type (last 30 days)
SELECT
  provider,
  task_type,
  COUNT(*)                                        AS total,
  COUNT(*) FILTER (WHERE status = 'completed')    AS successes,
  ROUND(100.0 * COUNT(*) FILTER (WHERE status = 'completed')
        / COUNT(*), 1)                            AS success_rate_pct,
  PERCENTILE_CONT(0.5) WITHIN GROUP (
    ORDER BY duration_ms)                         AS median_duration_ms,
  SUM(cost_usd)                                   AS total_cost_usd
FROM agent_task_metrics
WHERE workspace_id = $1
  AND created_at > NOW() - INTERVAL '30 days'
GROUP BY provider, task_type
ORDER BY success_rate_pct DESC;
```

## Admin API (internal, not public)

```
GET /api/admin/metrics/summary?workspace_id=<uuid>&days=30
Authorization: Bearer <PAT>  (owner/admin only)

Response:
{
  "workspace_id": "...",
  "window_days": 30,
  "providers": [
    {
      "provider": "anthropic",
      "model": "claude-opus-4-7",
      "total_tasks": 42,
      "success_rate_pct": 78.6,
      "median_duration_ms": 185000,
      "total_cost_usd": 12.45,
      "by_task_type": { "feature": 81.2, "bug": 75.0, "refactor": 77.8 }
    }
  ]
}
```

## Retention

- Raw rows: **90 days** (partitioned by `created_at` month)
- Aggregated rollups: **permanent** (daily `metrics_daily_rollup` table)
- Auto-vacuum: daily cron at 03:00 UTC

## What We Explicitly Do NOT Do (Torvalds guardrail)

- ❌ No frontend UI / dashboard / charts
- ❌ No public API endpoint
- ❌ No cross-workspace comparison
- ❌ No per-user attribution
- ❌ No real-time streaming of metrics

These are deferred to **Unresolved Question #1** from the council verdict — revisit after 30 days of data accumulation.

## Privacy

- No code content stored (no diffs, no prompts, no outputs)
- No user-identifying data beyond `workspace_id`
- `cost_usd` is estimated from token counts × provider list price; not actual billing

## Migration

```bash
cd server && go run ./cmd/migrate up
# applies: migrations/0000XX_create_agent_task_metrics.up.sql
```

---

*Companion issues: #12 (repo-DNA to enrich task_type classification), #13 (Agentra-Eval golden set to validate metric accuracy).*
