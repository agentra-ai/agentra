-- name: CreateLoop :one
-- Creates a new loop for an issue. status defaults to 'pending' if not specified;
-- agent_id is optional and can be assigned later.
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
-- Returns a single loop by ID, or no rows.
SELECT * FROM loops WHERE id = $1;

-- name: GetLoopForUpdate :one
SELECT * FROM loops WHERE id = $1 FOR UPDATE;

-- name: ListLoops :many
-- Lists loops for a workspace with optional status/issue filters.
-- limit is required and bounded to prevent unbounded scans.
SELECT * FROM loops
WHERE workspace_id = $1
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('issue_id')::uuid IS NULL OR issue_id = sqlc.narg('issue_id'))
ORDER BY created_at DESC
LIMIT sqlc.narg('limit')::int;

-- name: UpdateLoopStatus :one
-- Updates the status, current stage, iteration, and (optionally) failure_reason.
-- started_at is preserved across updates (only set on the first transition to 'running').
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
-- Records the PR URL/number/branch produced by a successful develop stage.
UPDATE loops
SET pr_url = $2, pr_number = $3, branch_name = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: LoadActiveLoops :many
-- Returns all running/paused loops for the coordinator to resume on startup.
-- Bounded to prevent unbounded scans on servers with many historical loops.
SELECT * FROM loops
WHERE status IN ('running', 'paused')
ORDER BY created_at
LIMIT 1000;

-- name: HasInFlightTaskForLoopStage :one
-- Returns true if there is an in-flight (queued, dispatched, or running)
-- task of the given task_type for the given loop. Used by
-- Coordinator.RestoreOnStartup to detect loops whose stage task was lost
-- during a restart and needs to be re-enqueued.
SELECT count(*) > 0 AS has_in_flight FROM agent_task_queue
WHERE loop_id = $1 AND task_type = $2
  AND status IN ('queued', 'dispatched', 'running');

-- name: GetLoopBranchAndIteration :one
-- Returns the branch_name and iteration for a loop. Used by the daemon
-- claim handler to populate the per-stage prompts (review/fix need the
-- develop stage's branch and the current iteration count). branch_name
-- may be empty (e.g. before the develop stage has pushed a branch) and
-- iteration may be 0 (e.g. the plan-stage bootstrap); both come back as
-- valid nullable fields so callers can fall back to placeholders.
SELECT branch_name, iteration FROM loops WHERE id = $1;
