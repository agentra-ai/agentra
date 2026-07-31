ALTER TABLE execution_traces
    DROP CONSTRAINT execution_traces_issue_id_fkey,
    ADD CONSTRAINT execution_traces_issue_id_fkey
        FOREIGN KEY (issue_id) REFERENCES issue(id),
    DROP CONSTRAINT execution_traces_agent_id_fkey,
    ADD CONSTRAINT execution_traces_agent_id_fkey
        FOREIGN KEY (agent_id) REFERENCES agent(id);

ALTER TABLE task_runs
    DROP CONSTRAINT task_runs_agent_id_fkey,
    ADD CONSTRAINT task_runs_agent_id_fkey
        FOREIGN KEY (agent_id) REFERENCES agent(id);
