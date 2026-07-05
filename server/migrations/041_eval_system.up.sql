-- Issue #13 [P2] Agentra-Eval v0: benchmark harness.

CREATE TABLE eval_golden_issues (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug          TEXT NOT NULL UNIQUE,                  -- e.g. "bug-001-login-redirect"
    category      TEXT NOT NULL CHECK (category IN ('feature', 'bug', 'refactor', 'test', 'docs')),
    workspace_id  UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    issue_id      UUID REFERENCES issue(id) ON DELETE SET NULL,
    title         TEXT NOT NULL,
    description   TEXT NOT NULL,
    expected_test TEXT,                                 -- regex the agent output should match
    max_duration_ms BIGINT DEFAULT 120000,              -- 2 minutes timeout
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE eval_runs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at   TIMESTAMPTZ,
    status        TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'running', 'completed', 'failed', 'timeout')),
    Total_cases   INTEGER NOT NULL DEFAULT 0,
    passed        INTEGER NOT NULL DEFAULT 0,
    failed        INTEGER NOT NULL DEFAULT 0,
    score         NUMERIC(5,2),                         -- 0.00 - 100.00
    prev_score    NUMERIC(5,2),                         -- used for regression detection
    regression    BOOLEAN NOT NULL DEFAULT FALSE,
    summary       JSONB,                                -- per-case results
    CONSTRAINT eval_runs_regression_check CHECK (score >= 0 AND score <= 100)
);

CREATE INDEX idx_eval_runs_workspace_time ON eval_runs(workspace_id, started_at DESC);
