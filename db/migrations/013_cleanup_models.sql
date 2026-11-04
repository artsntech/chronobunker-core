-- ChronoBunker Core - Phase cleanup: mark non-implemented models as deprecated
--
-- We do not delete rows from the models table because cost_events has a
-- foreign key to it. Historical cost records must remain intact for audit
-- and reconciliation. Instead, we mark the models without an implemented
-- provider as 'deprecated'. The router's modelCache filters by status='ga',
-- so deprecated models are never selected for new invocations.

BEGIN;

UPDATE models
SET status = 'deprecated', updated_at = now()
WHERE id IN (
    'google/gemini-1.5-flash',
    'google/gemini-1.5-pro',
    'anthropic/claude-3.5-sonnet',
    'openai/gpt-4o-mini',
    'openai/gpt-4o',
    'xai/grok-2',
    'groq/llama-3.1-8b',
    'groq/llama-3.3-70b'
);

COMMIT;