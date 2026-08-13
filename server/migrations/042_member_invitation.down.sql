ALTER TABLE member
    DROP COLUMN IF EXISTS declined_at,
    DROP COLUMN IF EXISTS accepted_at,
    DROP COLUMN IF EXISTS invited_at,
    DROP COLUMN IF EXISTS invited_by,
    DROP COLUMN IF EXISTS invitation_status;
