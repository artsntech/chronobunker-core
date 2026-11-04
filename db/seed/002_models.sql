BEGIN;

INSERT INTO models (id, provider, input_cost_per_1m, output_cost_per_1m, context_window, capabilities, status) VALUES
    ('groq/openai/gpt-oss-120b', 'groq',   0.1500, 0.6000, 131072,  '{"vision": false, "tools": true, "json": true}', 'ga'),
    ('groq/openai/gpt-oss-20b',  'groq',   0.0750, 0.3000, 131072,  '{"vision": false, "tools": true, "json": true}', 'ga'),
    ('gemini/gemini-3.5-flash-lite', 'gemini', 0.3000, 2.5000, 1048576, '{"vision": true, "tools": true, "json": true}', 'ga'),
    ('gemini/gemini-2.5-flash', 'gemini', 0.0750, 0.3000, 1000000, '{"vision": true, "tools": true, "json": true}', 'ga')
ON CONFLICT (id) DO NOTHING;

COMMIT;