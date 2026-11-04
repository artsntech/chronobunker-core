BEGIN;

-- gemini-2.5-flash is not referenced by routing.yaml or the provider default
-- (which now points to gemini-3.5-flash-lite). It was the default before the
-- Gemini 3.5 upgrade. Mark it deprecated to keep the registry aligned with
-- the code that actually uses models.
UPDATE models
SET status = 'deprecated', updated_at = now()
WHERE id = 'gemini/gemini-2.5-flash';

COMMIT;