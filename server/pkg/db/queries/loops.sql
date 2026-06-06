-- name: CreateLoop :one
INSERT INTO loops (
    issue_id, workspace_id, status, current_stage,
    iteration, max_iterations, agent_id, config
) VALUES (
    $1, $2, COALESCE(sqlc.narg('status'), 'pending'),
    sqlc.narg('current_stage'),
    COALESCE(sqlc.narg('iteration'), 0),
    COALESCE(sqlc.narg('max_iterations'), 5),
    sqlc.narg('agent_id'),
    COALESCE(sqlc.narg('config'), '{}'::jsonb)
)
RETURNING *;

-- name: GetLoop :one
SELECT * FROM loops WHERE id = $1;

-- name: ListLoops :many
SELECT * FROM loops
WHERE workspace_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('issue_id')::uuid IS NULL OR issue_id = sqlc.narg('issue_id'))
ORDER BY created_at DESC
LIMIT sqlc.narg('limit')::int;

-- name: UpdateLoopStatus :one
UPDATE loops
SET status = $2,
    current_stage = $3,
    iteration = $4,
    failure_reason = $5,
    started_at = COALESCE(started_at, $6),
    completed_at = $7,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: SetLoopPR :one
UPDATE loops
SET pr_url = $2, pr_number = $3, branch_name = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: LoadActiveLoops :many
SELECT * FROM loops
WHERE status IN ('running', 'paused')
ORDER BY created_at;
