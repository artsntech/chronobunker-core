-- ChronoBunker Core - Phase 2: extend agents with routing and tool configuration.
-- These columns are consumed by the Model Router and Tool Gateway in Phase 3.

BEGIN;

ALTER TABLE agents
    ADD COLUMN allowed_tools TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN target_model TEXT NOT NULL DEFAULT '',
    ADD COLUMN system_prompt TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN agents.allowed_tools IS 'Tool names this agent is permitted to invoke.';
COMMENT ON COLUMN agents.target_model IS 'Default model identifier for this agent (e.g. groq/llama-3.1-8b).';
COMMENT ON COLUMN agents.system_prompt IS 'System prompt template applied to every completion for this agent.';

COMMIT;