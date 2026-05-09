CREATE TABLE team_memory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    memory_type TEXT NOT NULL CHECK (memory_type IN ('learning', 'task_result', 'context', 'pattern')),
    content TEXT NOT NULL,
    embedding vector(1536),
    metadata JSONB DEFAULT '{}',
    created_by UUID REFERENCES agents(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX team_memory_workspace_id_idx ON team_memory(workspace_id);
CREATE INDEX team_memory_embedding_idx ON team_memory USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);