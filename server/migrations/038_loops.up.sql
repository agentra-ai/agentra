-- Agentic Engineering Loop: parent record for a Plan→Develop→Review→Fix cycle.

CREATE TABLE loops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,

    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'paused', 'done', 'failed', 'cancelled')),
    current_stage TEXT
        CHECK (current_stage IS NULL OR current_stage IN ('plan', 'develop', 'review', 'fix')),
    iteration INT NOT NULL DEFAULT 0,
    max_iterations INT NOT NULL DEFAULT 5,

    -- Outputs
    pr_url TEXT,
    pr_number INT,
    branch_name TEXT,

    -- Config
    agent_id UUID REFERENCES agent(id),
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    failure_reason TEXT,

    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_loops_issue_id ON loops(issue_id);
CREATE INDEX idx_loops_status ON loops(status)
    WHERE status IN ('pending', 'running', 'paused');

-- Discriminator on agent_task_queue: standard task vs one of 4 loop stages.
-- Replaces the spec's "task_type" assumption; previously absent.
ALTER TABLE agent_task_queue
    ADD COLUMN task_type VARCHAR(50) NOT NULL DEFAULT 'standard',
    ADD COLUMN loop_id UUID REFERENCES loops(id) ON DELETE SET NULL;

ALTER TABLE agent_task_queue
    ADD CONSTRAINT agent_task_queue_task_type_check
    CHECK (task_type IN ('standard', 'loop_plan', 'loop_develop', 'loop_review', 'loop_fix'));

CREATE INDEX idx_agent_task_queue_loop_id ON agent_task_queue(loop_id)
    WHERE loop_id IS NOT NULL;
CREATE INDEX idx_agent_task_queue_task_type ON agent_task_queue(task_type)
    WHERE task_type <> 'standard';
