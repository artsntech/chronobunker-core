-- ChronoBunker Core - Phase 1: Multi-tenant core schema
-- This migration establishes the tenants table and the agents table,
-- with Row-Level Security enforcing strict tenant isolation at the
-- database engine level.

BEGIN;

-- Enable UUID generation. gen_random_uuid() is built into Postgres 13+
-- and does not require the pgcrypto extension.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ---------------------------------------------------------------------------
-- Table: tenants
-- Each tenant represents an isolated customer organization.
-- ---------------------------------------------------------------------------
CREATE TABLE tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE tenants IS 'Isolated customer organizations. Each row is a tenant boundary.';

-- ---------------------------------------------------------------------------
-- Table: agents
-- Each agent belongs to exactly one tenant and represents a configured
-- AI agent instance (triage, reasoning, vision, etc.).
-- ---------------------------------------------------------------------------
CREATE TABLE agents (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    department   TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'draft'
                 CHECK (status IN ('draft', 'canary', 'production', 'killed')),
    health_score NUMERIC(3,2) NOT NULL DEFAULT 1.00
                 CHECK (health_score >= 0.00 AND health_score <= 1.00),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE agents IS 'AI agent instances. Isolated per tenant via RLS.';

CREATE INDEX idx_agents_tenant_status ON agents (tenant_id, status);

-- ---------------------------------------------------------------------------
-- Row-Level Security
-- The application sets `app.tenant_id` on each connection before any query.
-- Without FORCE, the table owner bypasses RLS, which defeats the purpose.
-- ---------------------------------------------------------------------------
ALTER TABLE agents ENABLE ROW LEVEL SECURITY;
ALTER TABLE agents FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON agents
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID);

COMMENT ON POLICY tenant_isolation ON agents IS
    'Restricts all SELECT/INSERT/UPDATE/DELETE to the current tenant.';

-- ---------------------------------------------------------------------------
-- Application role
-- The runtime role must NOT be a superuser and must NOT have BYPASSRLS,
-- otherwise RLS is silently ignored. The role is created here so that
-- migrations and application startup can agree on the same identity.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'chrono_app') THEN
        CREATE ROLE chrono_app LOGIN PASSWORD 'change_me_in_production';
    END IF;
END
$$;

GRANT USAGE ON SCHEMA public TO chrono_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON tenants TO chrono_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON agents TO chrono_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO chrono_app;

COMMIT;