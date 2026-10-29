BEGIN;

-- Each of these tables has RLS enabled on tenant_id. Without an index on
-- tenant_id, every query that touches the table performs a sequential scan
-- to evaluate the RLS policy. The indexes below make the policy evaluation
-- an index lookup.
CREATE INDEX IF NOT EXISTS idx_agents_tenant ON agents (tenant_id);
CREATE INDEX IF NOT EXISTS idx_idempotency_tenant ON idempotency_keys (tenant_id);
CREATE INDEX IF NOT EXISTS idx_audit_tenant ON audit_entries (tenant_id);
CREATE INDEX IF NOT EXISTS idx_cost_events_tenant ON cost_events (tenant_id);

-- Statement timeout for the application role. Any query that runs longer
-- than 10 seconds is aborted. This prevents a pathological query (for
-- example, a GraphQL query that triggers a full table scan) from holding
-- a connection indefinitely and starving the pool.
ALTER ROLE chrono_app SET statement_timeout = '10s';

-- Lock timeout: a query that waits more than 5 seconds for a lock is
-- aborted. Without this, a long-running transaction on one tenant can
-- block writes for other tenants.
ALTER ROLE chrono_app SET lock_timeout = '5s';

-- Idle in transaction timeout: a connection that stays idle inside a
-- transaction for more than 30 seconds is terminated. This guards against
-- application bugs that forget to commit or rollback.
ALTER ROLE chrono_app SET idle_in_transaction_session_timeout = '30s';

COMMIT;