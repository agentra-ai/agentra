-- Issue #11 [P1] Multi-provider telemetry backend pipe.
-- Append-only; never updated. Long rows (tokenUsage) are rare, so no
-- HOT-update concerns.

CREATE TABLE agent_task_metrics (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    task_id         UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    issue_id        UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,

    provider        TEXT NOT NULL,
    model           TEXT NOT NULL,
    runtime_mode    TEXT NOT NULL DEFAULT 'local' CHECK (runtime_mode IN ('local', 'cloud')),

    task_type       TEXT NOT NULL DEFAULT 'feature'
                    CHECK (task_type IN ('feature', 'bug', 'refactor', 'test', 'docs', 'other')),
    issue_priority  TEXT NOT NULL DEFAULT 'medium',

    status          TEXT NOT NULL
                    CHECK (status IN ('completed', 'failed', 'cancelled', 'timeout')),
    error_category  TEXT,

    duration_ms     BIGINT NOT NULL,
    token_input     INTEGER NOT NULL DEFAULT 0,
    token_output    INTEGER NOT NULL DEFAULT 0,
    cost_usd        NUMERIC(10,6),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Primary read path: workspace-scoped time-window aggregates.
CREATE INDEX idx_metrics_workspace_time
    ON agent_task_metrics (workspace_id, created_at DESC);

-- Per-provider drilldown: which provider wins on which task type.
CREATE INDEX idx_metrics_provider
    ON agent_task_metrics (provider, model, created_at DESC);

-- Per-task-type drilldown.
CREATE INDEX idx_metrics_task_type
    ON agent_task_metrics (task_type, status, created_at DESC);

-- Reverse lookup for a single issue’s runs.
CREATE INDEX idx_metrics_issue
    ON agent_task_metrics (issue_id);
