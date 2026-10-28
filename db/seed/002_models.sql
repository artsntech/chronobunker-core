BEGIN;

INSERT INTO models (id, provider, input_cost_per_1m, output_cost_per_1m, context_window, capabilities, status) VALUES
    ('groq/llama-3.1-8b',      'groq',      0.0500,  0.0800, 128000, '{"vision": false, "tools": true, "json": true}',  'ga'),
    ('groq/llama-3.3-70b',     'groq',      0.5900,  0.7900, 128000, '{"vision": false, "tools": true, "json": true}',  'ga'),
    ('google/gemini-1.5-flash','google',    0.0750,  0.3000, 1000000,'{"vision": true,  "tools": true, "json": true}',  'ga'),
    ('google/gemini-1.5-pro',  'google',    1.2500,  5.0000, 2000000,'{"vision": true,  "tools": true, "json": true}',  'ga'),
    ('anthropic/claude-3.5-sonnet','anthropic',3.0000,15.0000,200000,'{"vision": true,  "tools": true, "json": true}',  'ga'),
    ('openai/gpt-4o-mini',     'openai',    0.1500,  0.6000, 128000, '{"vision": true,  "tools": true, "json": true}',  'ga'),
    ('openai/gpt-4o',          'openai',    2.5000, 10.0000, 128000, '{"vision": true,  "tools": true, "json": true}',  'ga'),
    ('xai/grok-2',             'xai',       2.0000, 10.0000, 128000, '{"vision": true,  "tools": true, "json": true}',  'ga')
ON CONFLICT (id) DO NOTHING;

COMMIT;