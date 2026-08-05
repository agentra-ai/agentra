DROP INDEX IF EXISTS uq_inbox_lifecycle_event_recipient;
ALTER TABLE inbox_item DROP COLUMN IF EXISTS lifecycle_event_id;

DROP INDEX IF EXISTS uq_activity_lifecycle_event;
ALTER TABLE activity_log DROP COLUMN IF EXISTS lifecycle_event_id;

DROP INDEX IF EXISTS uq_comment_lifecycle_event;
ALTER TABLE comment DROP COLUMN IF EXISTS lifecycle_event_id;

DROP INDEX IF EXISTS uq_agent_task_metrics_run;
ALTER TABLE agent_task_metrics DROP COLUMN IF EXISTS run_id;

DROP TABLE IF EXISTS lifecycle_event_delivery;
DROP TABLE IF EXISTS lifecycle_event_receipt;
DROP TABLE IF EXISTS lifecycle_outbox;
