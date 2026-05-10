-- name: CreateInstallation :one
INSERT INTO github_installations (workspace_id, installation_id, account_login, account_type, access_token, refresh_token, token_expires_at, repositories)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetInstallation :one
SELECT * FROM github_installations WHERE workspace_id = $1;

-- name: UpdateInstallationToken :one
UPDATE github_installations SET
    access_token = $2,
    refresh_token = COALESCE($3, refresh_token),
    token_expires_at = $4,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteInstallation :exec
DELETE FROM github_installations WHERE id = $1;

-- name: CreateIssueLink :one
INSERT INTO issue_git_links (issue_id, repository, pr_number, commit_sha, branch_name, link_type)
VALUES ($1, $2, $3, $4, $5, 'pr')
RETURNING *;

-- name: GetIssueLinks :many
SELECT * FROM issue_git_links WHERE issue_id = $1;

-- name: UpdateIssueLink :one
UPDATE issue_git_links SET
    pr_number = COALESCE($2, pr_number),
    commit_sha = COALESCE($3, commit_sha),
    branch_name = COALESCE($4, branch_name)
WHERE id = $1
RETURNING *;

-- name: DeleteIssueLink :one
DELETE FROM issue_git_links WHERE id = $1 RETURNING *;

-- name: LinkCommit :one
INSERT INTO issue_git_links (issue_id, sha, message, branch, link_type)
VALUES ($1, $2, $3, $4, 'commit')
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetCommitsByIssue :many
SELECT * FROM issue_git_links WHERE issue_id = $1 AND link_type = 'commit';

-- name: LinkPR :one
INSERT INTO issue_git_links (issue_id, repository, pr_number, pr_state, pr_title, merged_at, link_type)
VALUES ($1, $2, $3, $4, $5, $6, 'pr')
ON CONFLICT DO NOTHING
RETURNING *;

-- name: UpdatePRState :exec
UPDATE issue_git_links SET pr_state = $2, merged_at = $3
WHERE issue_id = $1 AND pr_number = $4 AND link_type = 'pr';

-- name: LinkBranch :one
INSERT INTO issue_git_links (issue_id, branch, link_type)
VALUES ($1, $2, 'branch')
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetGitLinksByIssue :many
SELECT * FROM issue_git_links WHERE issue_id = $1;