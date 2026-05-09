-- 036_agent_provider.down.sql
ALTER TABLE agent DROP COLUMN IF EXISTS provider_config;
ALTER TABLE agent DROP COLUMN IF EXISTS model_override;
ALTER TABLE agent DROP COLUMN IF EXISTS provider;