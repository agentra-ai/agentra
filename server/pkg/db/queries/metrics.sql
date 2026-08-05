-- name: InsertAgentTaskMetric :one
INSERT INTO agent_task_metrics (
    workspace_id, task_id, issue_id, run_id,
    provider, model, runtime_mode,
    task_type, issue_priority,
    status, error_category,
    duration_ms, token_input, token_output, cost_usd
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7,
    $8, $9,
    $10, $11,
    $12, $13, $14, $15
)
ON CONFLICT (run_id) WHERE run_id IS NOT NULL
DO UPDATE SET run_id = EXCLUDED.run_id
RETURNING *;

-- name: GetMetricsSummary :many
-- Returns per-provider aggregates for a workspace in a time window.
SELECT
    provider,
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE status = 'completed') AS successes,
    ROUND(100.0 * COUNT(*) FILTER (WHERE status = 'completed')
        / NULLIF(COUNT(*), 0), 1) AS success_rate_pct,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY duration_ms)::BIGINT AS median_duration_ms,
    SUM(cost_usd) AS total_cost_usd
FROM agent_task_metrics
WHERE workspace_id = $1
  AND created_at > now() - ($2::int * interval '1 day')
GROUP BY provider
ORDER BY success_rate_pct DESC;

-- name: GetMetricsByTaskType :many
-- Per-task-type breakdown within the same window.
SELECT
    task_type,
    provider,
    COUNT(*) AS total,
    COUNT(*) FILTER (WHERE status = 'completed') AS successes,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY duration_ms)::BIGINT AS median_duration_ms
FROM agent_task_metrics
WHERE workspace_id = $1
  AND created_at > now() - ($2::int * interval '1 day')
GROUP BY task_type, provider
ORDER BY task_type, successes DESC;

-- name: GetMetricsByIssue :many
-- Recent runs for a single issue.
SELECT * FROM agent_task_metrics
WHERE issue_id = $1
ORDER BY created_at DESC
LIMIT 100;
