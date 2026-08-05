-- name: ListActivities :many
SELECT * FROM activity_log
WHERE issue_id = $1
ORDER BY created_at ASC
LIMIT $2 OFFSET $3;

-- name: CreateActivity :one
INSERT INTO activity_log (
    workspace_id, issue_id, actor_type, actor_id, action, details
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: CreateActivityForLifecycleEvent :one
INSERT INTO activity_log (
    workspace_id, issue_id, actor_type, actor_id, action, details,
    lifecycle_event_id
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (lifecycle_event_id) WHERE lifecycle_event_id IS NOT NULL
DO UPDATE SET lifecycle_event_id = EXCLUDED.lifecycle_event_id
RETURNING *;
