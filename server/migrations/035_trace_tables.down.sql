DROP INDEX IF EXISTS trace_steps_timestamp_idx;
DROP INDEX IF EXISTS trace_steps_task_run_id_idx;
DROP INDEX IF EXISTS task_runs_created_at_idx;
DROP INDEX IF EXISTS task_runs_agent_id_idx;
DROP INDEX IF EXISTS task_runs_task_id_idx;
DROP TABLE IF EXISTS trace_steps;
DROP TABLE IF EXISTS task_runs;