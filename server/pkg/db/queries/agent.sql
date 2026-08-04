-- name: ListAgents :many
SELECT * FROM agent
WHERE workspace_id = $1 AND archived_at IS NULL
ORDER BY created_at ASC;

-- name: ListAllAgents :many
SELECT * FROM agent
WHERE workspace_id = $1
ORDER BY created_at ASC;

-- name: GetAgent :one
SELECT * FROM agent
WHERE id = $1;

-- name: GetAgentInWorkspace :one
SELECT * FROM agent
WHERE id = $1 AND workspace_id = $2;

-- name: CreateAgent :one
INSERT INTO agent (
    workspace_id, name, description, avatar_url, runtime_mode,
    runtime_config, runtime_id, visibility, max_concurrent_tasks, owner_id,
    tools, triggers, instructions, provider, model_override, provider_config
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING *;

-- name: UpdateAgent :one
UPDATE agent SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url),
    runtime_config = COALESCE(sqlc.narg('runtime_config'), runtime_config),
    runtime_mode = COALESCE(sqlc.narg('runtime_mode'), runtime_mode),
    runtime_id = COALESCE(sqlc.narg('runtime_id'), runtime_id),
    visibility = COALESCE(sqlc.narg('visibility'), visibility),
    status = COALESCE(sqlc.narg('status'), status),
    max_concurrent_tasks = COALESCE(sqlc.narg('max_concurrent_tasks'), max_concurrent_tasks),
    tools = COALESCE(sqlc.narg('tools'), tools),
    triggers = COALESCE(sqlc.narg('triggers'), triggers),
    instructions = COALESCE(sqlc.narg('instructions'), instructions),
    provider = COALESCE(sqlc.narg('provider'), provider),
    model_override = COALESCE(sqlc.narg('model_override'), model_override),
    provider_config = COALESCE(sqlc.narg('provider_config'), provider_config),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ArchiveAgent :one
UPDATE agent SET archived_at = now(), archived_by = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: RestoreAgent :one
UPDATE agent SET archived_at = NULL, archived_by = NULL, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListAgentTasks :many
SELECT * FROM agent_task_queue
WHERE agent_id = $1
ORDER BY created_at DESC;

-- name: CreateAgentTask :one
-- Creates an agent task. task_type defaults to 'standard' (existing behavior);
-- loop tasks pass 'loop_plan'/'loop_develop'/'loop_review'/'loop_fix' plus a loop_id.
INSERT INTO agent_task_queue (
    agent_id, runtime_id, issue_id, status, priority,
    trigger_comment_id, runtime_type, cloud_runtime_id,
    task_type, loop_id
)
VALUES (
    $1, $2, $3, 'queued', $4, $5,
    COALESCE(sqlc.narg('runtime_type'), 'local'),
    sqlc.narg('cloud_runtime_id'),
    COALESCE(sqlc.narg('task_type'), 'standard'),
    sqlc.narg('loop_id')
)
RETURNING *;

-- name: CancelAgentTasksByIssue :exec
UPDATE agent_task_queue
SET status = 'cancelled'
WHERE issue_id = $1 AND status IN ('queued', 'dispatched', 'running');

-- name: CancelAgentTasksByAgent :exec
UPDATE agent_task_queue
SET status = 'cancelled'
WHERE agent_id = $1 AND status IN ('queued', 'dispatched', 'running');

-- name: GetAgentTask :one
SELECT * FROM agent_task_queue
WHERE id = $1;

-- name: ClaimAgentTask :one
-- Claims the next queued task for an agent, enforcing per-issue serialization:
-- a task is only claimable when no other task for the same issue is already
-- dispatched or running. This guarantees serial execution within an issue
-- while allowing parallel execution across different issues.
UPDATE agent_task_queue
SET status = 'dispatched', dispatched_at = now()
WHERE id = (
    SELECT atq.id FROM agent_task_queue atq
    WHERE atq.agent_id = $1 AND atq.status = 'queued'
      AND NOT EXISTS (
          SELECT 1 FROM agent_task_queue active
          WHERE active.issue_id = atq.issue_id
            AND active.status IN ('dispatched', 'running')
      )
    ORDER BY atq.priority DESC, atq.created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: ClaimAgentTaskByID :one
-- Claims one task whose runtime policy has already been validated. The exact
-- ID prevents a concurrent queue change from dispatching a different task
-- than the service inspected.
UPDATE agent_task_queue AS target
SET status = 'dispatched', dispatched_at = now()
WHERE target.id = $1 AND target.status = 'queued'
  AND NOT EXISTS (
      SELECT 1 FROM agent_task_queue active
      WHERE active.issue_id = target.issue_id
        AND active.status IN ('dispatched', 'running')
  )
RETURNING *;

-- name: RejectQueuedAgentTask :one
-- Capability mismatches are terminal configuration errors, not retryable
-- execution failures. Reject them before dispatch so daemons never launch an
-- incompatible provider process and the queue cannot retain them forever.
UPDATE agent_task_queue
SET status = 'failed', completed_at = now(), error = $2
WHERE id = $1 AND status = 'queued'
RETURNING *;

-- name: StartAgentTaskRun :one
-- The Work Item transition and Run creation are one database statement: a
-- caller can never observe running without an attempt identity.
WITH started_task AS (
    UPDATE agent_task_queue
    SET status = 'running', started_at = now()
    WHERE agent_task_queue.id = $1 AND agent_task_queue.status = 'dispatched'
    RETURNING agent_task_queue.id, agent_task_queue.agent_id
), started_run AS (
    INSERT INTO task_runs (task_id, agent_id, status, started_at)
    SELECT started_task.id, started_task.agent_id, 'running', now()
    FROM started_task
    RETURNING task_runs.id, task_runs.task_id
)
SELECT started_run.task_id, started_run.id AS run_id
FROM started_run;

-- name: CompleteAgentTask :one
UPDATE agent_task_queue
SET status = 'completed', completed_at = now(), result = $2, session_id = $3, work_dir = $4
WHERE id = $1 AND status = 'running'
RETURNING *;

-- name: CheckpointAgentTaskSession :one
-- Persists resumable state as soon as the provider creates a session. The
-- running-state guard prevents a late daemon callback from mutating a task
-- that has already completed, failed, or been cancelled.
UPDATE agent_task_queue
SET session_id = $2, work_dir = $3
WHERE id = $1 AND status = 'running'
RETURNING *;

-- name: GetLastTaskSession :one
-- Returns the session_id and work_dir from the most recent completed task
-- for a given (agent_id, issue_id) pair, used for session resumption.
SELECT session_id, work_dir FROM agent_task_queue
WHERE agent_id = $1 AND issue_id = $2 AND status = 'completed' AND session_id IS NOT NULL
ORDER BY completed_at DESC
LIMIT 1;

-- name: FailAgentTask :one
UPDATE agent_task_queue
SET status = 'failed', completed_at = now(), error = $2
WHERE id = $1 AND status IN ('dispatched', 'running')
RETURNING *;

-- name: RetryAgentTask :one
-- Resets a failed or active task back to queued, incrementing retry_count.
-- Cloud gateways report transient failures directly from dispatched/running;
-- accepting those states avoids first emitting a terminal failure.
UPDATE agent_task_queue
SET status = 'queued',
    completed_at = NULL,
    error = NULL,
    retry_count = retry_count + 1,
    dispatched_at = NULL,
    started_at = NULL
WHERE id = $1 AND status IN ('failed', 'dispatched', 'running') AND retry_count < max_retries
RETURNING *;

-- name: FailStaleTasks :many
-- Fails tasks stuck in dispatched/running beyond the given thresholds.
-- Handles cases where the daemon is alive but the task is orphaned
-- (e.g. agent process hung, daemon failed to report completion).
UPDATE agent_task_queue
SET status = 'failed', completed_at = now(), error = 'task timed out'
WHERE (status = 'dispatched' AND dispatched_at < now() - make_interval(secs => @dispatch_timeout_secs::double precision))
   OR (status = 'running' AND started_at < now() - make_interval(secs => @running_timeout_secs::double precision))
RETURNING id, agent_id, issue_id;

-- name: CancelAgentTask :one
UPDATE agent_task_queue
SET status = 'cancelled', completed_at = now()
WHERE id = $1 AND status IN ('queued', 'dispatched', 'running')
RETURNING *;

-- name: CountRunningTasks :one
SELECT count(*) FROM agent_task_queue
WHERE agent_id = $1 AND status IN ('dispatched', 'running');

-- name: HasActiveTaskForIssue :one
-- Returns true if there is any queued, dispatched, or running task for the issue.
SELECT count(*) > 0 AS has_active FROM agent_task_queue
WHERE issue_id = $1 AND status IN ('queued', 'dispatched', 'running');

-- name: HasPendingTaskForIssue :one
-- Returns true if there is a queued or dispatched (but not yet running) task for the issue.
-- Used by the coalescing queue: allow enqueue when a task is running (so
-- the agent picks up new comments on the next cycle) but skip if a pending
-- task already exists (natural dedup).
SELECT count(*) > 0 AS has_pending FROM agent_task_queue
WHERE issue_id = $1 AND status IN ('queued', 'dispatched');

-- name: HasPendingTaskForIssueAndAgent :one
-- Returns true if a specific agent already has a queued or dispatched task
-- for the given issue. Used by @mention trigger dedup.
SELECT count(*) > 0 AS has_pending FROM agent_task_queue
WHERE issue_id = $1 AND agent_id = $2 AND status IN ('queued', 'dispatched');

-- name: ListPendingTasksByRuntime :many
SELECT * FROM agent_task_queue
WHERE runtime_id = $1 AND status IN ('queued', 'dispatched')
ORDER BY priority DESC, created_at ASC;

-- name: ListActiveTasksByIssue :many
SELECT * FROM agent_task_queue
WHERE issue_id = $1 AND status IN ('dispatched', 'running')
ORDER BY created_at DESC;

-- name: ListTasksByIssue :many
SELECT * FROM agent_task_queue
WHERE issue_id = $1
ORDER BY created_at DESC;

-- name: UpdateAgentStatus :one
UPDATE agent SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;
