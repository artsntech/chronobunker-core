# ADR 002: Postgres RLS over Application-Layer Filtering

## Status
Accepted

## Context
ChronoBunker is multi-tenant. Every table has a `tenant_id` column. The isolation must be guaranteed, not best-effort. Two options:
1. **Application-layer filtering**: every query includes `WHERE tenant_id = $1`, with the tenant ID passed from the JWT middleware.
2. **Postgres Row-Level Security**: the database engine enforces the filter automatically, based on a session variable set before the query.

The application-layer approach is what most SaaS companies start with. It is also what most multi-tenant data leaks are caused by. A single missing `WHERE` clause, a single refactor that forgets to pass the tenant ID, and the leak happens.

## Decision
We chose **Postgres RLS** with the following setup:

```sql
ALTER TABLE agents ENABLE ROW LEVEL SECURITY;
ALTER TABLE agents FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON agents
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID);
```

The Go layer opens a transaction, sets `SET LOCAL app.tenant_id = '<uuid>'`, and runs the query. The RLS policy filters the rows.

Critical detail: the runtime role (`chrono_app`) is not a superuser and does not have `BYPASSRLS`. The `main.go` verifies this on startup and refuses to run if the role can bypass RLS.

The `NULLIF(..., '')` is also critical. Without it, an unset `app.tenant_id` returns an empty string, and the cast to UUID fails with invalid input syntax for type uuid. With NULLIF, an unset variable becomes NULL, and the comparison `tenant_id = NULL` evaluates to FALSE—which means zero rows. Fail-closed.

## Consequences

**Positive:**
- **No data leaks via Go bugs:** If a developer forgets a `WHERE` clause, the database refuses to return cross-tenant rows.
- **Auditable isolation:** A CISO can read the policy in SQL and verify that it is correct.
- **Testable policy:** `TestTenantIsolation` in `internal/agent/repository_test.go` verifies that a query with the wrong tenant returns zero rows.
- **One place to change:** If the tenant model changes (e.g., sub-tenants), the policy changes in one place.

**Negative:**
- **Performance overhead:** RLS adds a filter to every query. Without an index on `tenant_id`, the Postgres planner may choose a sequential scan. The migration `007_hardening.sql` adds the missing indexes.
- **Harder debugging:** A query that returns zero rows because of RLS looks identical to a query that returns zero rows because the row does not exist.
- **Runtime role dependency:** If the DBA sets the runtime role to `postgres` (superuser) for convenience, RLS is silently disabled. The `main.go` fail-fast check prevents this.

## When to revisit
If the system needs to support cross-tenant admin queries (e.g., for billing), the admin role needs `BYPASSRLS` and a separate policy. This is a known future extension.