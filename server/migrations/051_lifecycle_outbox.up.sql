-- Durable lifecycle facts are committed in the same transaction as the
-- authoritative Work Item + Run state. Projectors may deliver them more than
-- once, so every durable database projection carries an idempotency key.
CREATE TABLE lifecycle_outbox (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    work_item_id    UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    run_id          UUID REFERENCES task_runs(id) ON DELETE CASCADE,
    event_type      TEXT NOT NULL,
    event_version   INTEGER NOT NULL DEFAULT 1 CHECK (event_version > 0),
    payload         JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    available_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at       TIMESTAMPTZ,
    lock_token      UUID,
    processed_at    TIMESTAMPTZ,
    dead_lettered_at TIMESTAMPTZ,
    attempts        INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error      TEXT
);

CREATE INDEX idx_lifecycle_outbox_pending
    ON lifecycle_outbox (available_at, created_at, id)
    WHERE processed_at IS NULL AND dead_lettered_at IS NULL;

-- Each independent consumer owns its receipt. core-projections uses the
-- outbox row's processed_at cursor; correctness-critical consumers such as
-- Engineering Loop use this ledger so one consumer cannot acknowledge work
-- on behalf of another.
CREATE TABLE lifecycle_event_receipt (
    event_id       UUID NOT NULL REFERENCES lifecycle_outbox(id) ON DELETE CASCADE,
    consumer       TEXT NOT NULL,
    processed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, consumer)
);

CREATE TABLE lifecycle_event_delivery (
    event_id          UUID NOT NULL REFERENCES lifecycle_outbox(id) ON DELETE CASCADE,
    consumer          TEXT NOT NULL,
    attempts          INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error        TEXT,
    dead_lettered_at  TIMESTAMPTZ,
    PRIMARY KEY (event_id, consumer)
);

ALTER TABLE agent_task_metrics
    ADD COLUMN run_id UUID REFERENCES task_runs(id) ON DELETE CASCADE;
CREATE UNIQUE INDEX uq_agent_task_metrics_run
    ON agent_task_metrics(run_id)
    WHERE run_id IS NOT NULL;

ALTER TABLE comment
    ADD COLUMN lifecycle_event_id UUID REFERENCES lifecycle_outbox(id) ON DELETE SET NULL;
CREATE UNIQUE INDEX uq_comment_lifecycle_event
    ON comment(lifecycle_event_id)
    WHERE lifecycle_event_id IS NOT NULL;

ALTER TABLE activity_log
    ADD COLUMN lifecycle_event_id UUID REFERENCES lifecycle_outbox(id) ON DELETE SET NULL;
CREATE UNIQUE INDEX uq_activity_lifecycle_event
    ON activity_log(lifecycle_event_id)
    WHERE lifecycle_event_id IS NOT NULL;

ALTER TABLE inbox_item
    ADD COLUMN lifecycle_event_id UUID REFERENCES lifecycle_outbox(id) ON DELETE SET NULL;
CREATE UNIQUE INDEX uq_inbox_lifecycle_event_recipient
    ON inbox_item(lifecycle_event_id, recipient_type, recipient_id)
    WHERE lifecycle_event_id IS NOT NULL;
