-- Cloud runtimes table
CREATE TABLE cloud_runtimes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    gateway_url VARCHAR(500),  -- NULL for basic tier (Agentra-hosted)
    provider VARCHAR(50) NOT NULL,  -- 'anthropic' | 'openai'
    encrypted_api_key BYTEA NOT NULL,  -- AES-256-GCM encrypted
    api_key_hash VARCHAR(64) NOT NULL,  -- For key lookup/validation
    is_active BOOLEAN NOT NULL DEFAULT true,
    max_concurrent_tasks INT NOT NULL DEFAULT 3,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cloud_runtimes_workspace ON cloud_runtimes(workspace_id);

-- Cloud runtime tasks table
CREATE TABLE cloud_runtime_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cloud_runtime_id UUID NOT NULL REFERENCES cloud_runtimes(id) ON DELETE CASCADE,
    task_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE,
    container_id VARCHAR(100),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    exit_code INT,
    token_usage JSONB,
    cost_estimate DECIMAL(10, 6),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',  -- pending, running, completed, failed
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_task_id UNIQUE (task_id)
);

CREATE INDEX idx_cloud_runtime_tasks_runtime ON cloud_runtime_tasks(cloud_runtime_id);
CREATE INDEX idx_cloud_runtime_tasks_status ON cloud_runtime_tasks(status);

-- Add runtime_type to agent_task_queue table
ALTER TABLE agent_task_queue ADD COLUMN runtime_type VARCHAR(20) NOT NULL DEFAULT 'local';
ALTER TABLE agent_task_queue ADD COLUMN cloud_runtime_id UUID REFERENCES cloud_runtimes(id);

-- Add preferred_runtime to agent table
ALTER TABLE agent ADD COLUMN preferred_runtime VARCHAR(20) NOT NULL DEFAULT 'any';
