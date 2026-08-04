-- A Work Item can be retried, so task_id is not an execution-attempt
-- identity. Backfill one Run for historical message/trace rows that predate
-- explicit run identity, then bind both ledgers to run_id.
INSERT INTO task_runs (task_id, agent_id, status, started_at, completed_at, output, error)
SELECT DISTINCT ON (atq.id)
    atq.id,
    atq.agent_id,
    CASE
        WHEN atq.status IN ('completed', 'failed', 'cancelled') THEN atq.status
        ELSE 'running'
    END,
    COALESCE(atq.started_at, atq.dispatched_at, atq.created_at),
    CASE
        WHEN atq.status IN ('completed', 'failed', 'cancelled')
            THEN COALESCE(atq.completed_at, NOW())
        ELSE NULL
    END,
    CASE WHEN atq.result IS NOT NULL THEN atq.result::text ELSE NULL END,
    atq.error
FROM agent_task_queue atq
WHERE (
        EXISTS (SELECT 1 FROM task_message tm WHERE tm.task_id = atq.id)
        OR EXISTS (SELECT 1 FROM execution_traces et WHERE et.task_id = atq.id)
    )
  AND NOT EXISTS (SELECT 1 FROM task_runs tr WHERE tr.task_id = atq.id)
ORDER BY atq.id, atq.created_at DESC;

-- Legacy StartTask created task_runs and execution_traces in separate
-- best-effort writes. A task can therefore have more traces than runs. Add one
-- Run for every unmatched trace before pairing the two histories; otherwise a
-- many-to-one backfill would fail the run_id uniqueness invariant below.
WITH ranked_traces AS (
    SELECT
        et.*,
        ROW_NUMBER() OVER (
            PARTITION BY et.task_id
            ORDER BY et.start_time, et.created_at, et.id
        ) AS attempt_number
    FROM execution_traces et
), run_counts AS (
    SELECT task_id, COUNT(*) AS run_count
    FROM task_runs
    GROUP BY task_id
)
INSERT INTO task_runs (task_id, agent_id, status, started_at, completed_at)
SELECT
    rt.task_id,
    COALESCE(rt.agent_id, atq.agent_id),
    CASE rt.status
        WHEN 'completed' THEN 'completed'
        WHEN 'failed' THEN 'failed'
        WHEN 'aborted' THEN 'cancelled'
        ELSE 'running'
    END,
    rt.start_time,
    CASE WHEN rt.status = 'running' THEN NULL ELSE rt.end_time END
FROM ranked_traces rt
JOIN agent_task_queue atq ON atq.id = rt.task_id
LEFT JOIN run_counts rc ON rc.task_id = rt.task_id
WHERE rt.attempt_number > COALESCE(rc.run_count, 0);

ALTER TABLE task_message ADD COLUMN run_id UUID;

UPDATE task_message tm
SET run_id = (
    SELECT tr.id
    FROM task_runs tr
    WHERE tr.task_id = tm.task_id
    ORDER BY tr.created_at DESC
    LIMIT 1
);

ALTER TABLE task_message
    ALTER COLUMN run_id SET NOT NULL,
    ADD CONSTRAINT task_message_run_id_fkey
        FOREIGN KEY (run_id) REFERENCES task_runs(id) ON DELETE CASCADE;

DROP INDEX uq_task_message_task_id_seq;
CREATE UNIQUE INDEX uq_task_message_run_id_seq
    ON task_message(run_id, seq);
CREATE INDEX idx_task_message_run_id_seq
    ON task_message(run_id, seq);

ALTER TABLE execution_traces ADD COLUMN run_id UUID;

-- Before run_id existed, chronological position was the only durable
-- relationship between the two ledgers. Pair them by attempt order. The
-- missing-run repair above guarantees every Trace receives a distinct Run.
WITH ranked_traces AS (
    SELECT
        id,
        task_id,
        ROW_NUMBER() OVER (
            PARTITION BY task_id
            ORDER BY start_time, created_at, id
        ) AS attempt_number
    FROM execution_traces
), ranked_runs AS (
    SELECT
        id,
        task_id,
        ROW_NUMBER() OVER (
            PARTITION BY task_id
            ORDER BY started_at, created_at, id
        ) AS attempt_number
    FROM task_runs
)
UPDATE execution_traces et
SET run_id = rr.id
FROM ranked_traces rt
JOIN ranked_runs rr
  ON rr.task_id = rt.task_id
 AND rr.attempt_number = rt.attempt_number
WHERE et.id = rt.id;

ALTER TABLE execution_traces
    ALTER COLUMN run_id SET NOT NULL,
    ADD CONSTRAINT execution_traces_run_id_fkey
        FOREIGN KEY (run_id) REFERENCES task_runs(id) ON DELETE CASCADE;

CREATE UNIQUE INDEX uq_execution_traces_run_id
    ON execution_traces(run_id);
