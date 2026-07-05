-- Issue #21 Seat Management.
-- Workspace seat quota + member invitation lifecycle.

ALTER TABLE workspace
    ADD COLUMN IF NOT EXISTS plan TEXT NOT NULL DEFAULT 'free'
        CHECK (plan IN ('free', 'pro', 'enterprise')),
    ADD COLUMN IF NOT EXISTS max_seats INTEGER NOT NULL DEFAULT 5;

ALTER TABLE member
    ADD COLUMN IF NOT EXISTS invitation_status TEXT NOT NULL DEFAULT 'active'
        CHECK (invitation_status IN ('invited', 'active', 'declined')),
    ADD COLUMN IF NOT EXISTS invited_by UUID REFERENCES "user"(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS invited_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS accepted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS declined_at TIMESTAMPTZ;
