-- Drop cloud_runtime_tasks table
DROP INDEX IF EXISTS idx_cloud_runtime_tasks_status;
DROP INDEX IF EXISTS idx_cloud_runtime_tasks_runtime;
DROP TABLE IF EXISTS cloud_runtime_tasks;

-- Drop cloud_runtimes table
DROP TABLE IF EXISTS cloud_runtimes;

-- Remove runtime_type and cloud_runtime_id from agent_task_queue
ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS cloud_runtime_id;
ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS runtime_type;

-- Remove preferred_runtime from agent
ALTER TABLE agent DROP COLUMN IF EXISTS preferred_runtime;
