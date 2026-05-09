-- Enable pgvector for potential vector storage
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE task_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agent(id),
    status TEXT NOT NULL DEFAULT 'running'
        CHECK (status IN ('running', 'completed', 'failed', 'cancelled')),
    started_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    duration_ms INT,
    exit_code INT,
    total_steps INT DEFAULT 0,
    total_tokens INT DEFAULT 0,
    total_cost NUMERIC(10,6) DEFAULT 0,
    output TEXT,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE trace_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_run_id UUID NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
    step_number INT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    action TEXT NOT NULL CHECK (action IN ('tool_call', 'output', 'error', 'thinking', 'status')),
    tool TEXT,
    input_text TEXT,
    output_text TEXT,
    tokens_used INT DEFAULT 0,
    duration_ms INT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX task_runs_task_id_idx ON task_runs(task_id);
CREATE INDEX task_runs_agent_id_idx ON task_runs(agent_id);
CREATE INDEX task_runs_created_at_idx ON task_runs(created_at);
CREATE INDEX trace_steps_task_run_id_idx ON trace_steps(task_run_id);
CREATE INDEX trace_steps_timestamp_idx ON trace_steps(timestamp);