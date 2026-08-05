-- Cloud dispatch is a push transport: allocating a Run and attempting one
-- WebSocket write cannot prove that the Gateway received or started it. Keep
-- one durable delivery per Run so retries retain the same idempotency key.
CREATE TABLE cloud_dispatch_delivery (
    run_id             UUID PRIMARY KEY REFERENCES task_runs(id) ON DELETE CASCADE,
    work_item_id       UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    cloud_runtime_id   UUID NOT NULL REFERENCES cloud_runtimes(id) ON DELETE CASCADE,
    attempts           INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    available_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_at          TIMESTAMPTZ,
    lock_token         UUID,
    last_sent_at       TIMESTAMPTZ,
    acknowledged_at    TIMESTAMPTZ,
    dead_lettered_at   TIMESTAMPTZ,
    last_error         TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (work_item_id, run_id)
);

CREATE INDEX idx_cloud_dispatch_delivery_pending
    ON cloud_dispatch_delivery (available_at, created_at, run_id)
    WHERE acknowledged_at IS NULL AND dead_lettered_at IS NULL;
