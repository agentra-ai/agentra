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
INSERT INTO github_issue_links (issue_id, repository, pr_number, commit_sha, branch_name)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetIssueLinks :many
SELECT * FROM github_issue_links WHERE issue_id = $1;

-- name: UpdateIssueLink :one
UPDATE github_issue_links SET
    pr_number = COALESCE($2, pr_number),
    commit_sha = COALESCE($3, commit_sha),
    branch_name = COALESCE($4, branch_name)
WHERE id = $1
RETURNING *;

-- name: DeleteIssueLink :one
DELETE FROM github_issue_links WHERE id = $1 RETURNING *;