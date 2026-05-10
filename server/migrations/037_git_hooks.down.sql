DROP INDEX IF EXISTS idx_issue_git_links_sha;
DROP INDEX IF EXISTS idx_issue_git_links_issue_id;

ALTER TABLE issue_git_links DROP COLUMN IF EXISTS branch;
ALTER TABLE issue_git_links DROP COLUMN IF EXISTS pr_title;
ALTER TABLE issue_git_links DROP COLUMN IF EXISTS merged_at;
ALTER TABLE issue_git_links DROP COLUMN IF EXISTS pr_state;
ALTER TABLE issue_git_links DROP COLUMN IF EXISTS authored_at;
ALTER TABLE issue_git_links DROP COLUMN IF EXISTS message;
ALTER TABLE issue_git_links DROP COLUMN IF EXISTS sha;
ALTER TABLE issue_git_links DROP COLUMN IF EXISTS link_type;

ALTER TABLE issue_git_links RENAME TO github_issue_links;