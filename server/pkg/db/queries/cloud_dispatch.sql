-- name: ListQueuedCloudTasks :many
SELECT task.*
FROM agent_task_queue task
JOIN cloud_runtimes runtime ON runtime.id = task.cloud_runtime_id
WHERE task.status = 'queued'
  AND task.runtime_type = 'cloud'
  AND runtime.is_active = true
ORDER BY task.priority DESC, task.created_at, task.id
LIMIT 100;

-- name: ClaimCloudTaskRunByID :one
WITH candidate AS (
    SELECT task.id, task.agent_id, task.cloud_runtime_id
    FROM agent_task_queue task
    JOIN agent ON agent.id = task.agent_id
    JOIN cloud_runtimes runtime ON runtime.id = task.cloud_runtime_id
    WHERE task.id = $1
      AND task.status = 'queued'
      AND task.runtime_type = 'cloud'
      AND runtime.is_active = true
      AND NOT EXISTS (
          SELECT 1 FROM agent_task_queue active
          WHERE active.issue_id = task.issue_id
            AND active.status IN ('dispatched', 'running')
      )
      AND (
          SELECT count(*) FROM agent_task_queue active
          WHERE active.agent_id = task.agent_id
            AND active.status IN ('dispatched', 'running')
      ) < agent.max_concurrent_tasks
      AND (
          SELECT count(*) FROM agent_task_queue active
          WHERE active.cloud_runtime_id = task.cloud_runtime_id
            AND active.status IN ('dispatched', 'running')
      ) < runtime.max_concurrent_tasks
    FOR UPDATE OF task SKIP LOCKED
), claimed_run AS (
    INSERT INTO task_runs (task_id, agent_id, status, started_at)
    SELECT id, agent_id, 'dispatched', now()
    FROM candidate
    RETURNING id, task_id
), claimed_task AS (
    UPDATE agent_task_queue task
    SET status = 'dispatched', dispatched_at = now(), active_run_id = claimed_run.id
    FROM claimed_run
    WHERE task.id = claimed_run.task_id AND task.status = 'queued'
    RETURNING task.id, task.cloud_runtime_id
), delivery AS (
    INSERT INTO cloud_dispatch_delivery (run_id, work_item_id, cloud_runtime_id)
    SELECT claimed_run.id, claimed_task.id, claimed_task.cloud_runtime_id
    FROM claimed_run
    JOIN claimed_task ON claimed_task.id = claimed_run.task_id
    RETURNING work_item_id, run_id
)
SELECT work_item_id AS task_id, run_id FROM delivery;

-- name: ClaimCloudDispatchDelivery :one
WITH candidate AS (
    SELECT delivery.run_id
    FROM cloud_dispatch_delivery delivery
    JOIN agent_task_queue task ON task.id = delivery.work_item_id
    WHERE delivery.acknowledged_at IS NULL
      AND delivery.dead_lettered_at IS NULL
      AND delivery.attempts < 20
      AND delivery.available_at <= now()
      AND (delivery.locked_at IS NULL OR delivery.locked_at < now() - interval '30 seconds')
      AND task.status = 'dispatched'
      AND task.active_run_id = delivery.run_id
    ORDER BY delivery.available_at, delivery.created_at, delivery.run_id
    FOR UPDATE OF delivery SKIP LOCKED
    LIMIT 1
)
UPDATE cloud_dispatch_delivery delivery
SET locked_at = now(), lock_token = gen_random_uuid(), last_error = NULL
FROM candidate
WHERE delivery.run_id = candidate.run_id
RETURNING delivery.*;

-- name: DeferCloudDispatchDelivery :execrows
UPDATE cloud_dispatch_delivery
SET locked_at = NULL,
    lock_token = NULL,
    available_at = now() + interval '2 seconds',
    last_error = $3
WHERE run_id = $1
  AND lock_token = $2
  AND acknowledged_at IS NULL
  AND dead_lettered_at IS NULL;

-- name: MarkCloudDispatchSent :one
UPDATE cloud_dispatch_delivery
SET attempts = attempts + 1,
    locked_at = NULL,
    lock_token = NULL,
    last_sent_at = now(),
    available_at = now() + interval '10 seconds',
    last_error = NULL
WHERE run_id = $1
  AND lock_token = $2
  AND acknowledged_at IS NULL
  AND dead_lettered_at IS NULL
RETURNING *;

-- name: RecordCloudDispatchFailure :one
UPDATE cloud_dispatch_delivery
SET attempts = attempts + 1,
    locked_at = NULL,
    lock_token = NULL,
    available_at = CASE
        WHEN attempts + 1 >= 20 THEN now()
        ELSE now() + make_interval(secs => LEAST(300, (1 << LEAST(attempts + 1, 8))))
    END,
    last_error = $3
WHERE run_id = $1
  AND lock_token = $2
  AND acknowledged_at IS NULL
  AND dead_lettered_at IS NULL
RETURNING *;

-- name: GetExhaustedCloudDispatchDelivery :one
SELECT delivery.*
FROM cloud_dispatch_delivery delivery
JOIN agent_task_queue task ON task.id = delivery.work_item_id
WHERE delivery.acknowledged_at IS NULL
  AND delivery.dead_lettered_at IS NULL
  AND delivery.attempts >= 20
  AND delivery.available_at <= now()
  AND task.status = 'dispatched'
  AND task.active_run_id = delivery.run_id
ORDER BY delivery.available_at, delivery.created_at, delivery.run_id
LIMIT 1;

-- name: RetireStaleCloudDispatchDeliveries :execrows
UPDATE cloud_dispatch_delivery delivery
SET dead_lettered_at = now(),
    locked_at = NULL,
    lock_token = NULL,
    last_error = 'Work Item left dispatched state before Gateway acknowledgement'
FROM agent_task_queue task
WHERE task.id = delivery.work_item_id
  AND delivery.acknowledged_at IS NULL
  AND delivery.dead_lettered_at IS NULL
  AND (task.status <> 'dispatched' OR task.active_run_id IS DISTINCT FROM delivery.run_id);

-- name: AcknowledgeCloudDispatchDelivery :execrows
UPDATE cloud_dispatch_delivery
SET acknowledged_at = now(),
    locked_at = NULL,
    lock_token = NULL,
    last_error = NULL
WHERE work_item_id = $1
  AND run_id = $2
  AND acknowledged_at IS NULL
  AND dead_lettered_at IS NULL;

-- name: DeadLetterCloudDispatchDelivery :execrows
UPDATE cloud_dispatch_delivery
SET dead_lettered_at = now(),
    locked_at = NULL,
    lock_token = NULL,
    last_error = $3
WHERE work_item_id = $1
  AND run_id = $2
  AND acknowledged_at IS NULL
  AND dead_lettered_at IS NULL;

-- name: GetCloudDispatchDelivery :one
SELECT * FROM cloud_dispatch_delivery WHERE run_id = $1;
