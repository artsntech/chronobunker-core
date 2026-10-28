-- ChronoBunker Core - Phase 3a: model registry and cost ledger.
-- The models table is global (shared across tenants).
-- The cost_events table is tenant-scoped and protected by RLS.

BEGIN;

CREATE TABLE models (
    id                  TEXT PRIMARY KEY,
    provider            TEXT NOT NULL,
    input_cost_per_1m   NUMERIC(10,4) NOT NULL,
    output_cost_per_1m  NUMERIC(10,4) NOT NULL,
    context_window      INT NOT NULL,
    capabilities        JSONB NOT NULL DEFAULT '{}',
    status              TEXT NOT NULL DEFAULT 'ga'
                        CHECK (status IN ('ga', 'preview', 'deprecated')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE models IS 'Global model registry with cost and capability metadata.';

CREATE TABLE cost_events (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    session_id      TEXT NOT NULL,
    model_id        TEXT NOT NULL REFERENCES models(id),
    input_tokens    INT NOT NULL,
    output_tokens   INT NOT NULL,
    cost_usd        NUMERIC(12,8) NOT NULL,
    latency_ms      INT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE cost_events IS 'Per-call cost ledger. Tenant-scoped via RLS.';

CREATE INDEX idx_cost_events_tenant_created ON cost_events (tenant_id, created_at DESC);
CREATE INDEX idx_cost_events_session ON cost_events (session_id);

ALTER TABLE cost_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE cost_events FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_cost_events ON cost_events
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID);

GRANT SELECT, INSERT ON models TO chrono_app;
GRANT SELECT, INSERT ON cost_events TO chrono_app;
GRANT USAGE, SELECT ON SEQUENCE cost_events_id_seq TO chrono_app;

COMMIT;