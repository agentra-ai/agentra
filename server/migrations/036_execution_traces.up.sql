CREATE TABLE execution_traces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agent(id),
    issue_id UUID REFERENCES issue(id),
    provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    steps JSONB DEFAULT '[]',
    tools JSONB DEFAULT '[]',
    tokens JSONB DEFAULT '{}',
    cost NUMERIC(10,6) DEFAULT 0,
    start_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_time TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'completed', 'failed', 'aborted')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX execution_traces_task_id_idx ON execution_traces(task_id);
CREATE INDEX execution_traces_agent_id_idx ON execution_traces(agent_id);
CREATE INDEX execution_traces_issue_id_idx ON execution_traces(issue_id);
CREATE INDEX execution_traces_status_idx ON execution_traces(status);
CREATE INDEX execution_traces_start_time_idx ON execution_traces(start_time);
