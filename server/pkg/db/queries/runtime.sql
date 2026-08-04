-- name: ListAgentRuntimes :many
SELECT * FROM agent_runtime
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: GetAgentRuntime :one
SELECT * FROM agent_runtime
WHERE id = $1;

-- name: GetAgentRuntimeForWorkspace :one
SELECT * FROM agent_runtime
WHERE id = $1 AND workspace_id = $2;

-- name: GetAgentRuntimeByIdentity :one
SELECT * FROM agent_runtime
WHERE workspace_id = $1 AND daemon_id = $2 AND provider = $3;

-- name: UpsertAgentRuntime :one
INSERT INTO agent_runtime (
    workspace_id,
    daemon_id,
    name,
    runtime_mode,
    provider,
    status,
    device_info,
    metadata,
    last_seen_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (workspace_id, daemon_id, provider)
DO UPDATE SET
    name = EXCLUDED.name,
    runtime_mode = EXCLUDED.runtime_mode,
    status = EXCLUDED.status,
    device_info = EXCLUDED.device_info,
    metadata = EXCLUDED.metadata,
    last_seen_at = now(),
    updated_at = now()
RETURNING *;

-- name: UpdateAgentRuntimeHeartbeat :one
UPDATE agent_runtime
SET status = 'online', last_seen_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetAgentRuntimeOffline :exec
UPDATE agent_runtime
SET status = 'offline', updated_at = now()
WHERE id = $1;

-- name: MarkStaleRuntimesOffline :many
UPDATE agent_runtime
SET status = 'offline', updated_at = now()
WHERE status = 'online'
  AND last_seen_at < now() - make_interval(secs => @stale_seconds::double precision)
RETURNING id, workspace_id;

-- name: FailTasksForOfflineRuntimes :many
-- Marks dispatched/running tasks as failed when their runtime is offline.
-- This cleans up orphaned tasks after a daemon crash or network partition.
WITH targets AS (
    SELECT atq.id, atq.active_run_id
    FROM agent_task_queue atq
    WHERE atq.status IN ('dispatched', 'running')
      AND atq.runtime_id IN (
        SELECT id FROM agent_runtime WHERE status = 'offline'
      )
    FOR UPDATE
), closed_runs AS (
    UPDATE task_runs tr
    SET status = 'failed', completed_at = NOW(), error = 'runtime went offline'
    FROM targets
    WHERE tr.id = targets.active_run_id
      AND tr.status IN ('dispatched', 'running')
    RETURNING tr.id
)
UPDATE agent_task_queue atq
SET status = 'failed', completed_at = now(), error = 'runtime went offline', active_run_id = NULL
FROM targets
WHERE atq.id = targets.id
RETURNING atq.id, atq.agent_id, atq.issue_id;

-- name: RecoverTasksForRuntime :many
-- A daemon registration represents a fresh process for this stable runtime
-- identity. Requeue orphaned work within its retry budget so the new process
-- can resume from the in-flight session checkpoint; fail closed once the
-- budget is exhausted.
WITH targets AS (
    SELECT id, active_run_id
    FROM agent_task_queue atq
    WHERE atq.runtime_id = $1
      AND atq.runtime_type = 'local'
      AND (
          atq.status IN ('dispatched', 'running')
          OR (atq.status = 'failed' AND atq.error = 'runtime went offline')
      )
    FOR UPDATE
), closed_runs AS (
    UPDATE task_runs tr
    SET status = 'failed',
        completed_at = NOW(),
        error = 'runtime restarted; task queued for resume'
    FROM targets
    WHERE tr.id = targets.active_run_id
      AND tr.status IN ('dispatched', 'running')
    RETURNING tr.id
)
UPDATE agent_task_queue atq
SET status = CASE
        WHEN retry_count < max_retries THEN 'queued'
        ELSE 'failed'
    END,
    completed_at = CASE
        WHEN retry_count < max_retries THEN NULL
        ELSE now()
    END,
    error = CASE
        WHEN retry_count < max_retries THEN NULL
        ELSE 'runtime restarted and retry budget was exhausted'
    END,
    retry_count = CASE
        WHEN retry_count < max_retries THEN retry_count + 1
        ELSE retry_count
    END,
    dispatched_at = NULL,
    started_at = NULL,
    active_run_id = NULL
FROM targets
WHERE atq.id = targets.id
RETURNING atq.id AS task_id, targets.active_run_id AS run_id;
