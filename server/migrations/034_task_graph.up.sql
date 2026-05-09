-- 034_task_graph.up.sql
CREATE TABLE task_graph_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id),
    node_type TEXT NOT NULL CHECK (node_type IN ('root','planner','executor','synthesis')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','running','completed','failed','blocked')),
    context JSONB DEFAULT '{}',
    result JSONB,
    position_x FLOAT,
    position_y FLOAT,
    depth INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX task_graph_nodes_issue_id_idx ON task_graph_nodes(issue_id);
CREATE INDEX task_graph_nodes_workspace_id_idx ON task_graph_nodes(workspace_id);

CREATE TABLE task_graph_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_node_id UUID NOT NULL REFERENCES task_graph_nodes(id) ON DELETE CASCADE,
    to_node_id UUID NOT NULL REFERENCES task_graph_nodes(id) ON DELETE CASCADE,
    edge_type TEXT NOT NULL DEFAULT 'depends_on'
        CHECK (edge_type IN ('depends_on','handoff','triggers')),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX task_graph_edges_from_id_idx ON task_graph_edges(from_node_id);
CREATE INDEX task_graph_edges_to_id_idx ON task_graph_edges(to_node_id);
