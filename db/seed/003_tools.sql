BEGIN;

INSERT INTO tools (name, description, input_schema, risk_level, requires_approval) VALUES
    ('search_help_center',
     'Searches the help center for articles matching a query.',
     '{"type":"object","properties":{"query":{"type":"string","minLength":1,"maxLength":500}},"required":["query"],"additionalProperties":false}',
     'low', false),

    ('get_order',
     'Retrieves order details by order ID.',
     '{"type":"object","properties":{"order_id":{"type":"string","pattern":"^[A-Z0-9-]{4,32}$"}},"required":["order_id"],"additionalProperties":false}',
     'low', false),

    ('issue_refund',
     'Issues a refund for an order. Requires approval above the configured limit.',
     '{"type":"object","properties":{"order_id":{"type":"string","pattern":"^[A-Z0-9-]{4,32}$"},"amount":{"type":"number","minimum":0.01,"maximum":10000},"reason":{"type":"string","maxLength":500}},"required":["order_id","amount","reason"],"additionalProperties":false}',
     'high', true),

    ('cancel_order',
     'Cancels an order that has not yet shipped.',
     '{"type":"object","properties":{"order_id":{"type":"string","pattern":"^[A-Z0-9-]{4,32}$"},"reason":{"type":"string","maxLength":500}},"required":["order_id","reason"],"additionalProperties":false}',
     'high', true),

    ('send_email',
     'Sends a templated email to the customer.',
     '{"type":"object","properties":{"to":{"type":"string","format":"email"},"template":{"type":"string"},"variables":{"type":"object"}},"required":["to","template"],"additionalProperties":false}',
     'medium', false)
ON CONFLICT (name) DO NOTHING;

COMMIT;