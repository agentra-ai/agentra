-- name: CreateCloudRuntime :one
INSERT INTO cloud_runtimes (workspace_id, gateway_url, provider, encrypted_api_key, api_key_hash, max_concurrent_tasks)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetCloudRuntimeByWorkspace :one
SELECT * FROM cloud_runtimes WHERE workspace_id = $1 AND is_active = true;

-- name: UpdateCloudRuntime :one
UPDATE cloud_runtimes
SET gateway_url = $2, provider = $3, encrypted_api_key = $4, api_key_hash = $5, max_concurrent_tasks = $6, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteCloudRuntime :exec
UPDATE cloud_runtimes SET is_active = false, updated_at = NOW() WHERE id = $1;

-- name: CreateCloudRuntimeTask :one
INSERT INTO cloud_runtime_tasks (cloud_runtime_id, task_id)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateCloudRuntimeTask :one
UPDATE cloud_runtime_tasks
SET container_id = $2, started_at = $3, status = $4
WHERE id = $1
RETURNING *;

-- name: CompleteCloudRuntimeTask :exec
UPDATE cloud_runtime_tasks
SET completed_at = NOW(), exit_code = $2, token_usage = $3, cost_estimate = $4, status = $5
WHERE id = $1;

-- name: GetCloudRuntimeTaskByTaskID :one
SELECT * FROM cloud_runtime_tasks WHERE task_id = $1;
