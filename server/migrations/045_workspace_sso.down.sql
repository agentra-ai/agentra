ALTER TABLE workspace
    DROP COLUMN IF EXISTS claimed_domain,
    DROP COLUMN IF EXISTS sso_policy;
