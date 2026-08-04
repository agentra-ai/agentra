DROP INDEX IF EXISTS uq_execution_traces_run_id;
ALTER TABLE execution_traces
    DROP CONSTRAINT IF EXISTS execution_traces_run_id_fkey,
    DROP COLUMN IF EXISTS run_id;

DROP INDEX IF EXISTS idx_task_message_run_id_seq;
DROP INDEX IF EXISTS uq_task_message_run_id_seq;

-- The old schema cannot represent two Runs that both start at seq=1. Keep the
-- newest row for each legacy (task_id, seq) cursor during downgrade.
DELETE FROM task_message newer
USING task_message older
WHERE newer.task_id = older.task_id
  AND newer.seq = older.seq
  AND (newer.created_at, newer.id) < (older.created_at, older.id);

ALTER TABLE task_message
    DROP CONSTRAINT IF EXISTS task_message_run_id_fkey,
    DROP COLUMN IF EXISTS run_id;

CREATE UNIQUE INDEX uq_task_message_task_id_seq
    ON task_message(task_id, seq);
