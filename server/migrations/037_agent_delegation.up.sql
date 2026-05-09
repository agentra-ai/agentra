-- 037_agent_delegation.up.sql

CREATE TABLE agent_delegation_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    from_agent_id UUID REFERENCES agents(id),
    to_agent_type TEXT NOT NULL CHECK (to_agent_type IN ('planner', 'executor', 'synthesis')),
    max_depth INT DEFAULT 3,
    allow_parallel BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX agent_delegation_policies_workspace_idx ON agent_delegation_policies(workspace_id);
