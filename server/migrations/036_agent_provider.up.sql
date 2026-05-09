-- 036_agent_provider.up.sql
ALTER TABLE agent ADD COLUMN provider TEXT NOT NULL DEFAULT 'claude';
ALTER TABLE agent ADD COLUMN model_override TEXT;
ALTER TABLE agent ADD COLUMN provider_config JSONB DEFAULT '{}';