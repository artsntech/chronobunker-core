BEGIN;

SET LOCAL app.tenant_id = '11111111-1111-1111-1111-111111111111';

INSERT INTO agents (tenant_id, name, department, status, allowed_tools, target_model)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    'Chaos Probe Agent',
    'chaos',
    'production',
    ARRAY['search_help_center'],
    'groq/invalid-model-does-not-exist'
)
ON CONFLICT (tenant_id, name) DO UPDATE
    SET department = EXCLUDED.department,
        target_model = EXCLUDED.target_model;

COMMIT;