BEGIN;

CREATE TABLE store_actions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    mission_id  TEXT NOT NULL,
    action      TEXT NOT NULL,
    params      JSONB NOT NULL,
    result      JSONB,
    status      TEXT NOT NULL DEFAULT 'executed'
                CHECK (status IN ('executed', 'blocked', 'failed')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_store_actions_tenant_mission ON store_actions (tenant_id, mission_id, created_at DESC);

ALTER TABLE store_actions ENABLE ROW LEVEL SECURITY;
ALTER TABLE store_actions FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON store_actions
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID);

GRANT SELECT, INSERT ON store_actions TO chrono_app;

COMMIT;