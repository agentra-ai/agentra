-- Add retry tracking columns to agent_task_queue
ALTER TABLE agent_task_queue ADD COLUMN retry_count INT NOT NULL DEFAULT 0;
ALTER TABLE agent_task_queue ADD COLUMN max_retries INT NOT NULL DEFAULT 3;
