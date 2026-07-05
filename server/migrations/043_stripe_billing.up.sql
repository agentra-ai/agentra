CREATE TABLE subscriptions (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id            UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    stripe_subscription_id  TEXT,
    stripe_customer_id      TEXT,
    plan                    TEXT NOT NULL DEFAULT 'free' CHECK (plan IN ('free', 'pro')),
    status                  TEXT NOT NULL DEFAULT 'incomplete'
                            CHECK (status IN ('incomplete', 'active', 'past_due', 'canceled', 'unpaid')),
    seats                   INTEGER NOT NULL DEFAULT 0,
    current_period_start    TIMESTAMPTZ,
    current_period_end      TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE invoices (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    stripe_invoice_id   TEXT,
    amount_cents        INTEGER NOT NULL DEFAULT 0,
    currency            TEXT NOT NULL DEFAULT 'usd',
    status              TEXT NOT NULL DEFAULT 'draft'
                        CHECK (status IN ('draft', 'open', 'paid', 'void', 'uncollectible')),
    period_start        TIMESTAMPTZ,
    period_end          TIMESTAMPTZ,
    hosted_invoice_url  TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE usage_records (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    metric          TEXT NOT NULL,            -- 'seat', 'agent_run_minutes'
    quantity        NUMERIC NOT NULL DEFAULT 0,
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_subscriptions_workspace_status
    ON subscriptions(workspace_id)
    WHERE status = 'active';

CREATE INDEX idx_invoices_workspace_time
    ON invoices(workspace_id, created_at DESC);

CREATE INDEX idx_usage_records_workspace_period
    ON usage_records(workspace_id, recorded_at DESC);
