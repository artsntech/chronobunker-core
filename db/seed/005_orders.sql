BEGIN;

SET LOCAL app.tenant_id = '11111111-1111-1111-1111-111111111111';

INSERT INTO orders (tenant_id, customer_name, product, quantity, total_usd, status) VALUES
    ('11111111-1111-1111-1111-111111111111', 'Alice Silva', 'Bunker Pro Plan', 1, 299.00, 'delivered'),
    ('11111111-1111-1111-1111-111111111111', 'Alice Silva', 'Chrono Agent License', 3, 89.00, 'processing'),
    ('11111111-1111-1111-1111-111111111111', 'Bob Costa', 'Bunker Starter Plan', 1, 99.00, 'pending')
ON CONFLICT DO NOTHING;

SET LOCAL app.tenant_id = '22222222-2222-2222-2222-222222222222';

INSERT INTO orders (tenant_id, customer_name, product, quantity, total_usd, status) VALUES
    ('22222222-2222-2222-2222-222222222222', 'Carla Rodrigues', 'Bunker Enterprise Plan', 1, 1299.00, 'delivered'),
    ('22222222-2222-2222-2222-222222222222', 'Carla Rodrigues', 'Chrono Agent License', 5, 149.00, 'shipped')
ON CONFLICT DO NOTHING;

COMMIT;