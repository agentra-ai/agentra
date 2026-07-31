-- name: CreateTaskMessage :execrows
INSERT INTO task_message (task_id, seq, type, tool, content, input, output)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (task_id, seq) DO NOTHING;

-- name: ListTaskMessages :many
SELECT *
FROM (
    SELECT * FROM task_message
    WHERE task_id = $1
    ORDER BY seq DESC
    LIMIT $2
) AS recent
ORDER BY seq ASC;

-- name: ListTaskMessagesSince :many
SELECT * FROM task_message
WHERE task_id = $1 AND seq > $2
ORDER BY seq ASC
LIMIT $3;

-- name: DeleteTaskMessages :exec
DELETE FROM task_message
WHERE task_id = $1;
