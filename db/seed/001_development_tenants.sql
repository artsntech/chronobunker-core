-- ChronoBunker Core - Development seed data
-- Creates two tenants and one agent per tenant for Phase 1 integration tests.
-- Because FORCE ROW LEVEL SECURITY is active on the agents table, each INSERT
-- must run with app.tenant_id set to the target tenant.

BEGIN;

INSERT INTO tenants (id, name, slug) VALUES
    ('11111111-1111-1111-1111-111111111111', 'Acme Corporation', 'acme'),
    ('22222222-2222-2222-2222-222222222222', 'Globex Industries', 'globex')
ON CONFLICT (slug) DO NOTHING;

SET LOCAL app.tenant_id = '11111111-1111-1111-1111-111111111111';
INSERT INTO agents (tenant_id, name, department, status) VALUES
    ('11111111-1111-1111-1111-111111111111', 'Acme Triage Agent', 'support', 'production')
ON CONFLICT (tenant_id, name) DO NOTHING;

SET LOCAL app.tenant_id = '22222222-2222-2222-2222-222222222222';
INSERT INTO agents (tenant_id, name, department, status) VALUES
    ('22222222-2222-2222-2222-222222222222', 'Globex Triage Agent', 'support', 'production')
ON CONFLICT (tenant_id, name) DO NOTHING;

COMMIT;