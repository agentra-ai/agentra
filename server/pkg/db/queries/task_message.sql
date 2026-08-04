-- name: CreateTaskMessage :execrows
INSERT INTO task_message (task_id, run_id, seq, type, tool, content, input, output)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (run_id, seq) DO NOTHING;

-- name: ListTaskMessages :many
SELECT *
FROM (
    SELECT * FROM task_message
    WHERE run_id = (
        SELECT tr.id FROM task_runs tr
        WHERE tr.task_id = $1
        ORDER BY tr.created_at DESC
        LIMIT 1
    )
    ORDER BY seq DESC
    LIMIT $2
) AS recent
ORDER BY seq ASC;

-- name: ListTaskMessagesSince :many
SELECT * FROM task_message
WHERE run_id = (
    SELECT tr.id FROM task_runs tr
    WHERE tr.task_id = $1
    ORDER BY tr.created_at DESC
    LIMIT 1
) AND seq > $2
ORDER BY seq ASC
LIMIT $3;

-- name: GetActiveTaskRun :one
SELECT * FROM task_runs
WHERE task_id = $1 AND id = $2 AND status = 'running';

-- name: GetLatestRunningTaskRun :one
SELECT * FROM task_runs
WHERE task_id = $1 AND status = 'running'
ORDER BY created_at DESC
LIMIT 1;

-- name: DeleteTaskMessages :exec
DELETE FROM task_message
WHERE task_id = $1;
