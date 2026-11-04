# Operations Runbooks

## Runbook: Circuit breaker is OPEN

**Symptom:** `chronobunker_breaker_state{target_id="..."} = 2` in Prometheus. The cockpit shows `THREAT: CRITICAL`.

**Diagnosis:**
1. Check the provider status page (Groq status, Gemini status).
2. Check the audit log for the last 10 minutes: `SELECT * FROM breaker_transitions ORDER BY created_at DESC LIMIT 20;`
3. Check the model registry: `SELECT id, status FROM models WHERE id = '<target_id>';`

**Action:**
- If the provider is down, wait for the cooldown (5 min). The breaker will transition to HALF_OPEN and test recovery.
- If the model is deprecated (e.g., `gemini-2.5-flash`), update the `routing.yaml` and restart the backend.
- If the model ID is invalid, fix the agent's `target_model` in the database.

**Prevention:** the `Breaker Lab` in the cockpit forces failures to test the FSM. Run it before deploying a new model.

## Runbook: Intent classifier is down

**Symptom:** `invokeAgent` returns `intent classification unavailable`.

**Diagnosis:**
1. Check that `GROQ_API_KEY` is set in the environment.
2. Check the Groq status page.
3. Check the classifier model ID: `openai/gpt-oss-20b`. Primary: `openai/gpt-oss-20b` (Groq). Fallback: `gemini-3.5-flash-lite` (Gemini).

**Action:**
- If Groq is down, the classifier falls back to Gemini automatically. If both are down, `invokeAgent` fails closed.
- If the API key is invalid, rotate it.
- If the model ID changed, update `internal/router/classifier.go`.

**Prevention:** the classifier has a health check in the red team harness (RT-001). Run it after deploying.

## Runbook: Rate limit is being hit

**Symptom:** `429 Too Many Requests` with `Retry-After` header.

**Diagnosis:**
1. Check the tenant: `GET /metrics | grep ratelimit`.
2. Check the IP: the `IPRateLimit` counters in Redis.

**Action:**
- If a tenant is abusing the API, raise the limit for that tenant or contact the customer.
- If an IP is flooding, the `IPRateLimit` already caps it at 20 req/10s. No action needed.

**Prevention:** the per-tenant limit (100 req/s) is configurable in `main.go`.

## Runbook: Audit trail is not updating

**Symptom:** the Audit Trail screen is static; new tool invocations do not appear.

**Diagnosis:**
1. Check the Postgres trigger: `SELECT * FROM pg_trigger WHERE tgname = 'audit_entry_notify';`
2. Check the `audit.Hub` logs: `audit hub: new subscriber for tenant "..."`.
3. Check the WebSocket connection in the browser DevTools (Network tab, WS filter).

**Action:**
- If the trigger is missing, re-run `db/migrations/009_audit_notify.sql`.
- If the hub is not running, restart the backend.
- If the WebSocket is closed, check `OriginPatterns` in the `transport.Websocket` config.

**Prevention:** the red team harness does not test the audit trail (it is a read-only feature). A future scenario (RT-006) would.

## Runbook: RLS is not filtering

**Symptom:** a tenant sees another tenant's agents.

**Diagnosis:**
1. Check the runtime role: `SELECT current_user, rolbypassrls FROM pg_roles WHERE rolname = current_user;`
2. Check the RLS policy: `SELECT * FROM pg_policies WHERE tablename = 'agents';`
3. Check the transaction: `SHOW app.tenant_id;` inside the transaction.

**Action:**
- If the role is `postgres` or has `BYPASSRLS`, the `main.go` should have refused to start. Check the logs.
- If the policy is missing, re-run `db/migrations/001_core.sql`.
- If `app.tenant_id` is not set, check `db.WithTenantTx`.

**Prevention:** `TestTenantIsolation` runs on every commit. RT-003 in the red team harness verifies it live.