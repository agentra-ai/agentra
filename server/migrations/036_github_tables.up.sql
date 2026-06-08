CREATE TABLE github_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    installation_id BIGINT NOT NULL,
    account_login TEXT NOT NULL,
    account_type TEXT NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT,
    token_expires_at TIMESTAMPTZ,
    repositories JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE github_issue_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    repository TEXT NOT NULL,
    pr_number INT,
    commit_sha TEXT,
    branch_name TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX github_installations_workspace_idx ON github_installations(workspace_id);
CREATE INDEX github_issue_links_issue_idx ON github_issue_links(issue_id);