-- Rename github_issue_links to issue_git_links for broader VCS support
ALTER TABLE github_issue_links RENAME TO issue_git_links;

-- Add link_type to distinguish commit vs PR vs branch links
ALTER TABLE issue_git_links ADD COLUMN link_type TEXT NOT NULL DEFAULT 'pr';

-- Add commit-specific fields
ALTER TABLE issue_git_links ADD COLUMN sha TEXT;
ALTER TABLE issue_git_links ADD COLUMN message TEXT;
ALTER TABLE issue_git_links ADD COLUMN authored_at TIMESTAMPTZ;

-- Add PR-specific fields
ALTER TABLE issue_git_links ADD COLUMN pr_state TEXT;
ALTER TABLE issue_git_links ADD COLUMN merged_at TIMESTAMPTZ;
ALTER TABLE issue_git_links ADD COLUMN pr_title TEXT;

-- Branch name used to link the branch to the issue
ALTER TABLE issue_git_links ADD COLUMN branch TEXT;

-- Create index for fast lookups by issue_id + link_type
CREATE INDEX idx_issue_git_links_issue_id ON issue_git_links(issue_id);
CREATE INDEX idx_issue_git_links_sha ON issue_git_links(sha) WHERE sha IS NOT NULL;