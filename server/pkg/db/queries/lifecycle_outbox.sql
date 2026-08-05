-- name: AppendLifecycleOutboxEvent :one
INSERT INTO lifecycle_outbox (
    work_item_id, run_id, event_type, event_version, payload
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ClaimLifecycleOutboxEvent :one
WITH candidate AS (
    SELECT id
    FROM lifecycle_outbox
    WHERE processed_at IS NULL
      AND dead_lettered_at IS NULL
      AND available_at <= now()
      AND (locked_at IS NULL OR locked_at < now() - interval '30 seconds')
      AND NOT EXISTS (
          SELECT 1
          FROM lifecycle_outbox earlier
          WHERE earlier.work_item_id = lifecycle_outbox.work_item_id
            AND earlier.processed_at IS NULL
            AND earlier.dead_lettered_at IS NULL
            AND (earlier.created_at, earlier.id) < (lifecycle_outbox.created_at, lifecycle_outbox.id)
      )
    ORDER BY created_at, id
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
UPDATE lifecycle_outbox event
SET locked_at = now(),
    lock_token = gen_random_uuid(),
    attempts = event.attempts + 1,
    last_error = NULL
FROM candidate
WHERE event.id = candidate.id
RETURNING event.*;

-- name: MarkLifecycleOutboxEventProcessed :execrows
UPDATE lifecycle_outbox
SET processed_at = now(), locked_at = NULL, lock_token = NULL, last_error = NULL
WHERE id = $1 AND lock_token = $2 AND processed_at IS NULL;

-- name: ReleaseLifecycleOutboxEvent :execrows
UPDATE lifecycle_outbox
SET locked_at = NULL,
    lock_token = NULL,
    available_at = now() + make_interval(secs => LEAST(300, (1 << LEAST(attempts, 8)))),
    dead_lettered_at = CASE WHEN attempts >= 20 THEN now() ELSE NULL END,
    last_error = $2
WHERE id = $1 AND lock_token = $3 AND processed_at IS NULL;

-- name: GetLifecycleOutboxEvent :one
SELECT * FROM lifecycle_outbox WHERE id = $1;

-- name: CountPendingLifecycleOutboxEvents :one
SELECT count(*) FROM lifecycle_outbox
WHERE processed_at IS NULL AND dead_lettered_at IS NULL;

-- name: ClaimEngineeringLoopLifecycleEvent :one
SELECT event.*
FROM lifecycle_outbox event
JOIN agent_task_queue task ON task.id = event.work_item_id
LEFT JOIN lifecycle_event_delivery delivery
  ON delivery.event_id = event.id AND delivery.consumer = 'engineering-loop'
WHERE event.event_type IN ('run.completed', 'run.failed')
  AND task.loop_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM lifecycle_event_receipt receipt
      WHERE receipt.event_id = event.id
        AND receipt.consumer = 'engineering-loop'
  )
  AND (delivery.event_id IS NULL OR (
      delivery.available_at <= now() AND delivery.dead_lettered_at IS NULL
  ))
ORDER BY event.created_at, event.id
FOR UPDATE OF event SKIP LOCKED
LIMIT 1;

-- name: RecordEngineeringLoopLifecycleReceipt :exec
INSERT INTO lifecycle_event_receipt (event_id, consumer)
VALUES ($1, 'engineering-loop')
ON CONFLICT DO NOTHING;

-- name: RecordEngineeringLoopLifecycleFailure :exec
INSERT INTO lifecycle_event_delivery (event_id, consumer, attempts, available_at, last_error)
VALUES ($1, 'engineering-loop', 1, now() + interval '2 seconds', $2)
ON CONFLICT (event_id, consumer) DO UPDATE
SET attempts = lifecycle_event_delivery.attempts + 1,
    available_at = now() + make_interval(
        secs => LEAST(300, (1 << LEAST(lifecycle_event_delivery.attempts + 1, 8)))
    ),
    last_error = EXCLUDED.last_error,
    dead_lettered_at = CASE
        WHEN lifecycle_event_delivery.attempts + 1 >= 20 THEN now()
        ELSE NULL
    END;

-- name: HasPendingEngineeringLoopLifecycleEvent :one
SELECT count(*) > 0
FROM lifecycle_outbox event
JOIN agent_task_queue task ON task.id = event.work_item_id
WHERE task.loop_id = $1
  AND task.task_type = $2
  AND event.event_type IN ('run.completed', 'run.failed')
  AND NOT EXISTS (
      SELECT 1 FROM lifecycle_event_receipt receipt
      WHERE receipt.event_id = event.id
        AND receipt.consumer = 'engineering-loop'
  );
