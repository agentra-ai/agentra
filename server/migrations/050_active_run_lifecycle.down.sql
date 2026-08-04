ALTER TABLE agent_task_queue
    DROP CONSTRAINT IF EXISTS agent_task_queue_active_run_id_fkey;

DROP INDEX IF EXISTS uq_agent_task_queue_active_run;
DROP INDEX IF EXISTS uq_task_runs_active_task;

ALTER TABLE agent_task_queue
    DROP COLUMN IF EXISTS active_run_id;

UPDATE task_runs
SET status = 'failed',
    completed_at = COALESCE(completed_at, NOW()),
    error = COALESCE(error, 'dispatch interrupted by migration rollback')
WHERE status = 'dispatched';

ALTER TABLE task_runs
    DROP COLUMN IF EXISTS session_id,
    DROP COLUMN IF EXISTS work_dir,
    DROP CONSTRAINT task_runs_status_check,
    ADD CONSTRAINT task_runs_status_check
        CHECK (status IN ('running', 'completed', 'failed', 'cancelled'));
