-- ChronoBunker Core - Phase 3b: tool registry, idempotency ledger, audit trail.

BEGIN;

-- ---------------------------------------------------------------------------
-- Table: tools
-- Global registry of tools that agents can invoke. Each tool declares its
-- input schema (JSON Schema) and its risk level. High-risk tools require
-- human approval before execution.
-- ---------------------------------------------------------------------------
CREATE TABLE tools (
    name            TEXT PRIMARY KEY,
    description     TEXT NOT NULL,
    input_schema    JSONB NOT NULL,
    risk_level      TEXT NOT NULL DEFAULT 'low'
                    CHECK (risk_level IN ('low', 'medium', 'high')),
    requires_approval BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE tools IS 'Global tool registry with input schemas and risk levels.';

-- ---------------------------------------------------------------------------
-- Table: idempotency_keys
-- The 3-phase execution ledger. A tool invocation is reserved before the
-- external side effect, marked executed after it, and committed once the
-- result is recorded. Retries with the same key return the cached response
-- instead of re-executing the side effect.
-- ---------------------------------------------------------------------------
CREATE TYPE idempotency_status AS ENUM ('reserved', 'executed', 'committed', 'failed');

CREATE TABLE idempotency_keys (
    key           TEXT PRIMARY KEY,
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    session_id    TEXT NOT NULL,
    tool_name     TEXT NOT NULL REFERENCES tools(name),
    status        idempotency_status NOT NULL DEFAULT 'reserved',
    params        JSONB NOT NULL,
    response      JSONB,
    error         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE idempotency_keys IS 'Effectively-once execution ledger for tool invocations.';

CREATE INDEX idx_idempotency_tenant_status ON idempotency_keys (tenant_id, status);
CREATE INDEX idx_idempotency_session ON idempotency_keys (session_id);

ALTER TABLE idempotency_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE idempotency_keys FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_idempotency ON idempotency_keys
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID);

-- ---------------------------------------------------------------------------
-- Table: audit_entries
-- Append-only audit trail with a SHA-256 hash chain. Each row's row_hash is
-- computed over (prev_hash || row content), so tampering with any historical
-- row breaks the chain. Database triggers block UPDATE and DELETE.
-- ---------------------------------------------------------------------------
CREATE TABLE audit_entries (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    session_id    TEXT NOT NULL,
    tool_name     TEXT NOT NULL,
    params        JSONB NOT NULL,
    result        JSONB,
    decision      TEXT NOT NULL,
    prev_hash     TEXT NOT NULL,
    row_hash      TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_tenant_created ON audit_entries (tenant_id, created_at DESC);
CREATE INDEX idx_audit_session ON audit_entries (session_id);

ALTER TABLE audit_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_entries FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_audit ON audit_entries
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID);

-- Prevent UPDATE and DELETE on the audit trail.
CREATE OR REPLACE FUNCTION audit_immutable() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_entries is append-only; UPDATE and DELETE are forbidden';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_no_update
    BEFORE UPDATE ON audit_entries
    FOR EACH ROW EXECUTE FUNCTION audit_immutable();

CREATE TRIGGER audit_no_delete
    BEFORE DELETE ON audit_entries
    FOR EACH ROW EXECUTE FUNCTION audit_immutable();

-- ---------------------------------------------------------------------------
-- Grants
-- ---------------------------------------------------------------------------
GRANT SELECT, INSERT ON tools TO chrono_app;
GRANT SELECT, INSERT, UPDATE ON idempotency_keys TO chrono_app;
GRANT SELECT, INSERT ON audit_entries TO chrono_app;
GRANT USAGE, SELECT ON SEQUENCE audit_entries_id_seq TO chrono_app;

COMMIT;