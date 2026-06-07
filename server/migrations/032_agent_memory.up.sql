CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE agent_memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    memory_type TEXT NOT NULL CHECK (memory_type IN ('learning', 'task_result', 'context', 'pattern')),
    content TEXT NOT NULL,
    embedding vector(1536),
    metadata JSONB DEFAULT '{}',
    is_private BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX agent_memories_agent_id_idx ON agent_memories(agent_id);
CREATE INDEX agent_memories_workspace_id_idx ON agent_memories(workspace_id);
CREATE INDEX agent_memories_embedding_idx ON agent_memories USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);