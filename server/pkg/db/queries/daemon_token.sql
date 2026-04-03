-- name: CreateDaemonToken :one
INSERT INTO daemon_token (token_hash, workspace_id, daemon_id, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetDaemonTokenByHash :one
SELECT * FROM daemon_token
WHERE token_hash = $1 AND expires_at > now();

-- name: DeleteDaemonTokensByWorkspaceAndDaemon :exec
DELETE FROM daemon_token
WHERE workspace_id = $1 AND daemon_id = $2;

-- name: DeleteExpiredDaemonTokens :exec
DELETE FROM daemon_token
WHERE expires_at <= now();
