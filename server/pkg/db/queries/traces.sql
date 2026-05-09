-- name: CreateTaskRun :one
INSERT INTO task_runs (task_id, agent_id, status, started_at)
VALUES ($1, $2, 'running', NOW())
RETURNING *;

-- name: GetTaskRun :one
SELECT * FROM task_runs WHERE id = $1;

-- name: CompleteTaskRun :one
UPDATE task_runs SET
    status = COALESCE($2, status),
    completed_at = NOW(),
    duration_ms = $3,
    exit_code = $4,
    total_steps = $5,
    total_tokens = $6,
    total_cost = $7,
    output = $8,
    error = $9
WHERE id = $1
RETURNING *;

-- name: ListTaskRuns :many
SELECT * FROM task_runs WHERE agent_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: ListTaskRunsByTask :many
SELECT * FROM task_runs WHERE task_id = $1 ORDER BY created_at DESC;

-- name: RecordTraceSteps :many
INSERT INTO trace_steps (task_run_id, step_number, timestamp, action, tool, input_text, output_text, tokens_used, duration_ms, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListTraceSteps :many
SELECT * FROM trace_steps WHERE task_run_id = $1 ORDER BY step_number;

-- name: GetTraceAnalytics :one
SELECT
    COUNT(*) as total_runs,
    AVG(duration_ms) as avg_duration,
    AVG(total_tokens) as avg_tokens,
    AVG(total_cost) as avg_cost,
    SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_count,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count
FROM task_runs
WHERE agent_id = $1 AND created_at > NOW() - $2::interval;
