-- name: ListComments :many
SELECT * FROM comment
WHERE issue_id = $1 AND workspace_id = $2
ORDER BY created_at ASC;

-- name: GetComment :one
SELECT * FROM comment
WHERE id = $1;

-- name: GetCommentInWorkspace :one
SELECT * FROM comment
WHERE id = $1 AND workspace_id = $2;

-- name: CreateComment :one
INSERT INTO comment (issue_id, workspace_id, author_type, author_id, content, type, parent_id)
VALUES ($1, $2, $3, $4, $5, $6, sqlc.narg(parent_id))
RETURNING *;

-- name: CreateCommentForLifecycleEvent :one
INSERT INTO comment (
    issue_id, workspace_id, author_type, author_id, content, type, parent_id,
    lifecycle_event_id
)
VALUES ($1, $2, $3, $4, $5, $6, sqlc.narg(parent_id), $7)
ON CONFLICT (lifecycle_event_id) WHERE lifecycle_event_id IS NOT NULL
DO UPDATE SET lifecycle_event_id = EXCLUDED.lifecycle_event_id
RETURNING *;

-- name: UpdateComment :one
UPDATE comment SET
    content = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteComment :exec
DELETE FROM comment WHERE id = $1;
