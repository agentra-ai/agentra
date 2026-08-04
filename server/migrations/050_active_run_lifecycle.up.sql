-- A Run identity is allocated when a Work Item is dispatched, before either
-- the local daemon or cloud gateway starts execution. This lets every Adapter
-- carry one stable identity through provisioning, logs, checkpoints, and the
-- terminal callback.
ALTER TABLE task_runs
    DROP CONSTRAINT task_runs_status_check,
    ADD CONSTRAINT task_runs_status_check
        CHECK (status IN ('dispatched', 'running', 'completed', 'failed', 'cancelled'));

ALTER TABLE task_runs
    ADD COLUMN session_id TEXT,
    ADD COLUMN work_dir TEXT;

ALTER TABLE agent_task_queue
    ADD COLUMN active_run_id UUID;

-- Preserve the resumable session on the most recent historical Run. The Work
-- Item columns remain as a current-session projection for compatibility.
WITH latest_runs AS (
    SELECT
        id,
        task_id,
        ROW_NUMBER() OVER (
            PARTITION BY task_id
            ORDER BY created_at DESC, id DESC
        ) AS position
    FROM task_runs
)
UPDATE task_runs tr
SET session_id = atq.session_id,
    work_dir = atq.work_dir
FROM latest_runs lr
JOIN agent_task_queue atq ON atq.id = lr.task_id
WHERE tr.id = lr.id
  AND lr.position = 1
  AND (atq.session_id IS NOT NULL OR atq.work_dir IS NOT NULL);

-- Older retry/recovery implementations could leave more than one running Run
-- or a running Run under a non-running Work Item. Repair that history before
-- enforcing the active-Run uniqueness invariant.
WITH ranked_active_runs AS (
    SELECT
        tr.id,
        tr.task_id,
        ROW_NUMBER() OVER (
            PARTITION BY tr.task_id
            ORDER BY tr.created_at DESC, tr.id DESC
        ) AS position
    FROM task_runs tr
    WHERE tr.status = 'running'
)
UPDATE task_runs tr
SET status = CASE atq.status
        WHEN 'completed' THEN 'completed'
        WHEN 'cancelled' THEN 'cancelled'
        ELSE 'failed'
    END,
    completed_at = COALESCE(tr.completed_at, NOW()),
    error = CASE
        WHEN atq.status IN ('completed', 'cancelled') THEN tr.error
        ELSE COALESCE(tr.error, 'superseded before active Run enforcement')
    END
FROM ranked_active_runs rar
JOIN agent_task_queue atq ON atq.id = rar.task_id
WHERE tr.id = rar.id
  AND (atq.status <> 'running' OR rar.position > 1);

-- A legacy running Work Item should already have a Run, but best-effort trace
-- creation allowed gaps. Repair them so the pointer can be made authoritative.
INSERT INTO task_runs (task_id, agent_id, status, started_at, session_id, work_dir)
SELECT
    atq.id,
    atq.agent_id,
    'running',
    COALESCE(atq.started_at, atq.dispatched_at, atq.created_at),
    atq.session_id,
    atq.work_dir
FROM agent_task_queue atq
WHERE atq.status = 'running'
  AND NOT EXISTS (
      SELECT 1 FROM task_runs tr
      WHERE tr.task_id = atq.id AND tr.status = 'running'
  );

-- Dispatched Work Items predate dispatch-allocated Run identity. Allocate one
-- so a daemon/gateway already holding the Work Item can still start it.
INSERT INTO task_runs (task_id, agent_id, status, started_at, session_id, work_dir)
SELECT
    atq.id,
    atq.agent_id,
    'dispatched',
    COALESCE(atq.dispatched_at, atq.created_at),
    atq.session_id,
    atq.work_dir
FROM agent_task_queue atq
WHERE atq.status = 'dispatched';

UPDATE agent_task_queue atq
SET active_run_id = (
    SELECT tr.id
    FROM task_runs tr
    WHERE tr.task_id = atq.id
      AND tr.status = atq.status
    ORDER BY tr.created_at DESC, tr.id DESC
    LIMIT 1
)
WHERE atq.status IN ('dispatched', 'running');

CREATE UNIQUE INDEX uq_task_runs_active_task
    ON task_runs(task_id)
    WHERE status IN ('dispatched', 'running');

CREATE UNIQUE INDEX uq_agent_task_queue_active_run
    ON agent_task_queue(active_run_id)
    WHERE active_run_id IS NOT NULL;

ALTER TABLE agent_task_queue
    ADD CONSTRAINT agent_task_queue_active_run_id_fkey
        FOREIGN KEY (active_run_id) REFERENCES task_runs(id) ON DELETE SET NULL;
