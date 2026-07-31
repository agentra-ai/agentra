-- Trace records must follow the lifecycle of their owning task and issue.
-- Agent references are optional historical metadata and should not prevent an
-- agent or workspace from being removed.
ALTER TABLE task_runs
    DROP CONSTRAINT task_runs_agent_id_fkey,
    ADD CONSTRAINT task_runs_agent_id_fkey
        FOREIGN KEY (agent_id) REFERENCES agent(id) ON DELETE SET NULL;

ALTER TABLE execution_traces
    DROP CONSTRAINT execution_traces_agent_id_fkey,
    ADD CONSTRAINT execution_traces_agent_id_fkey
        FOREIGN KEY (agent_id) REFERENCES agent(id) ON DELETE SET NULL,
    DROP CONSTRAINT execution_traces_issue_id_fkey,
    ADD CONSTRAINT execution_traces_issue_id_fkey
        FOREIGN KEY (issue_id) REFERENCES issue(id) ON DELETE CASCADE;
