DROP INDEX IF EXISTS idx_agent_task_queue_task_type;
DROP INDEX IF EXISTS idx_agent_task_queue_loop_id;

ALTER TABLE agent_task_queue DROP CONSTRAINT IF EXISTS agent_task_queue_task_type_check;
ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS loop_id;
ALTER TABLE agent_task_queue DROP COLUMN IF EXISTS task_type;

DROP INDEX IF EXISTS idx_loops_status;
DROP INDEX IF EXISTS idx_loops_issue_id;
DROP TABLE IF EXISTS loops;
