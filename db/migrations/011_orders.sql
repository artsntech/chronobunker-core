BEGIN;

-- Orders table for the Bunker Store. Each order belongs to a customer
-- (identified by tenant_id) and is protected by RLS.
CREATE TABLE orders (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    customer_name TEXT NOT NULL,
    product     TEXT NOT NULL,
    quantity    INT NOT NULL CHECK (quantity > 0),
    total_usd   NUMERIC(10,2) NOT NULL CHECK (total_usd >= 0),
    status      TEXT NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'processing', 'shipped', 'delivered', 'cancelled')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_tenant_status ON orders (tenant_id, status);

ALTER TABLE orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON orders
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID);

GRANT SELECT, INSERT, UPDATE ON orders TO chrono_app;

COMMIT;