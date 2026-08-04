-- name: CreateTaskMessage :one
-- Lock the Work Item before checking its active Run. Terminal Lifecycle
-- transitions take the same lock first, so a message can never commit after
-- its Run is completed, failed, retried, or cancelled.
WITH active AS MATERIALIZED (
    SELECT atq.id
    FROM agent_task_queue atq
    WHERE atq.id = sqlc.arg('task_id')
      AND atq.active_run_id = sqlc.arg('run_id')
      AND atq.status = 'running'
      AND EXISTS (
          SELECT 1 FROM task_runs tr
          WHERE tr.id = sqlc.arg('run_id')
            AND tr.task_id = sqlc.arg('task_id')
            AND tr.status = 'running'
      )
    FOR UPDATE OF atq
), inserted AS (
    INSERT INTO task_message (task_id, run_id, seq, type, tool, content, input, output)
    SELECT
        sqlc.arg('task_id'),
        sqlc.arg('run_id'),
        sqlc.arg('seq'),
        sqlc.arg('type'),
        sqlc.arg('tool'),
        sqlc.arg('content'),
        sqlc.arg('input'),
        sqlc.arg('output')
    FROM active
    ON CONFLICT (run_id, seq) DO NOTHING
    RETURNING 1
)
SELECT
    EXISTS (SELECT 1 FROM active) AS active,
    EXISTS (SELECT 1 FROM inserted) AS inserted;

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
SELECT tr.* FROM task_runs tr
WHERE tr.task_id = $1
  AND tr.id = $2
  AND tr.status = 'running'
  AND tr.id = (SELECT atq.active_run_id FROM agent_task_queue atq WHERE atq.id = $1);

-- name: DeleteTaskMessages :exec
DELETE FROM task_message
WHERE task_id = $1;
