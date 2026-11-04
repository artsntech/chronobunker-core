BEGIN;

-- The routing.yaml only references models that have been tested:
--   - groq/openai/gpt-oss-120b
--   - groq/openai/gpt-oss-20b
--   - gemini/gemini-3.5-flash-lite
-- Mark the rest as deprecated to keep the registry honest.
UPDATE models
SET status = 'deprecated', updated_at = now()
WHERE id IN ('gemini/gemini-2.5-pro', 'gemini/gemini-3.8-flash');

COMMIT;