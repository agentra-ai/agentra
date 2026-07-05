-- Issue #24 [P3 Enterprise SSO]: workspace-level email domain claim.
-- When acme.com is claimed by workspace W, any user signing up via OTP
-- with an @acme.com email is auto-provisioned as a W member.

ALTER TABLE workspace
    ADD COLUMN claimed_domain TEXT NOT NULL DEFAULT '',
    ADD COLUMN sso_policy     TEXT NOT NULL DEFAULT 'open'
                    CHECK (sso_policy IN ('open', 'domain_claim', 'saml', 'oidc'));

CREATE INDEX idx_workspace_claimed_domain
    ON workspace (claimed_domain)
    WHERE claimed_domain <> '';
