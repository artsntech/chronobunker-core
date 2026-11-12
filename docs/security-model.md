# Security Model

ChronoBunker assumes the LLM is hostile, the frontend is compromised, and the network is untrusted. Every layer fails safe.

## Layer 1: Postgres Row-Level Security

The isolation is enforced by the database engine, not by Go code.

```sql
ALTER TABLE agents ENABLE ROW LEVEL SECURITY;
ALTER TABLE agents FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON agents
    USING (tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::UUID);
```

- **Threat:** cross-tenant data leaks.
- **Mitigation:** even if the Go code has a missing `WHERE tenant_id = ...`, the database refuses to return rows from another tenant.
- **Test:** `TestTenantIsolation` in `internal/agent/repository_test.go`.
- **ADR:** 002.

## Layer 2: JWT RS256 with algorithm pinning

```go
jwt.WithValidMethods([]string{"RS256"}),
jwt.WithIssuer("chronobunker"),
jwt.WithAudience("chronobunker-api"),
jwt.WithExpirationRequired(),
```

- **Threat:** token forgery, algorithm confusion (alg: none, alg: HS256).
- **Mitigation:** the parser rejects any token not signed with RS256 by the correct issuer, for the correct audience, and not expired.
- **Test:** `TestGraphQLTenantIsolationEndToEnd` in `internal/transport/graph/e2e_test.go`.

## Layer 3: Ed25519 capability tokens with cryptographic device binding

Ephemeral tokens with 5-minute TTL, scoped to specific tools, resources, and device. The device binding is cryptographic: the browser generates a non-extractable Ed25519 key pair on first login, registers the public key with the server, and signs every `invokeTool` request. The server verifies the signature against the registered public key before executing any tool.

- **Threat:** unauthorized tool execution, prompt-injected tool abuse, stolen token replay from another device.
- **Mitigation:** the Tool Gateway validates signature, expiration, tenant match, tool scope, resource binding, amount limit, and device signature. A stolen token cannot be used from a different device because the attacker does not have the private key.
- **Canonicalization:** the signed payload uses alphabetically-sorted JSON keys on both sides (browser `canonicalJSON` and Go `json.Marshal` of `map[string]any`). Without this, the order of `params` keys would differ between the two runtimes and the signature would fail.
- **Test:** `TestGatewayRejectsCrossTenantResourceBinding`, `TestGatewayIsIdempotentOnRetry`, `TestGatewayRejectsDeviceMismatch`.
- **ADR:** 003.

## Layer 4: Three-phase idempotency

RESERVE → EXECUTE → COMMIT with ON CONFLICT DO NOTHING.

- **Threat:** duplicate side effects (double refund, double email).
- **Mitigation:** the second attempt with the same deterministic key returns the cached response.
- **Test:** `TestGatewayIsIdempotentOnRetry`.
- **ADR:** 004.

## Layer 5: Circuit breaker FSM

CLOSED → OPEN → HALF_OPEN with exponential backoff.

- **Threat:** cascading LLM outages, wasted timeout cost.
- **Mitigation:** the breaker short-circuits requests in ~1ms when the provider is failing.
- **Test:** `TestBreakerTripsAfterConsecutiveFailures`, `TestBreakerTransitionsToHalfOpenAfterTimeout`.
- **ADR:** 006.

## Layer 6: Output sanitization

bluemonday allowlist (backend) + DOMPurify (frontend).

- **Threat:** XSS, indirect prompt injection in LLM output (OWASP LLM05).
- **Mitigation:** the backend strips HTML tags; the frontend sanitizes again before rendering.
- **Test:** `TestSanitizerRemovesScriptTags`, `TestSanitizerBlocksJavascriptURLs`.

## Layer 7: Canary tokens

A secret token in the system prompt. If it appears in the output, the LLM leaked its instructions.

- **Threat:** system prompt leakage (OWASP LLM07).
- **Mitigation:** any response containing the canary is aborted.
- **Test:** `TestSanitizerDetectsCanaryLeak`.

## Layer 8: Semantic intent gating

A fast LLM classifier rejects out-of-scope prompts before the main LLM is called. Primary: Groq. Fallback: Gemini. If both fail, the request is rejected (fail-closed).

- **Threat:** prompt injection (OWASP LLM01), out-of-domain queries.
- **Mitigation:** the classifier is domain-aware and fails closed.
- **Test:** RT-001 in `internal/redteam/harness.go` and `scripts/redteam_ci.go`.
- **ADR:** 005.

## Layer 9: GraphQL depth and complexity limits

Max depth 10, max complexity 200, batching prevented.

- **Threat:** GraphQL DoS via recursive queries.
- **Mitigation:** the handler rejects queries exceeding the limits before execution.

## Layer 10: Body limit

1 MiB via `http.MaxBytesReader`.

- **Threat:** payload flooding.
- **Mitigation:** the HTTP server closes the connection if the body exceeds the limit.

## Layer 11: Rate limiting

100 req/s per tenant, 20 req/10s per IP.

- **Threat:** abuse, brute force.
- **Mitigation:** the Redis-backed limiter rejects excess requests with 429 Too Many Requests and Retry-After.

## Layer 12: HTTP hardening

ReadHeaderTimeout: 5s, WriteTimeout: 15s, IdleTimeout: 60s, MaxHeaderBytes: 1 MiB, graceful shutdown.

- **Threat:** Slowloris, header flooding.
- **Mitigation:** the timeouts close slow connections; the graceful shutdown drains in-flight requests.

## Layer 13: Audit trail hash chain

SHA-256 chain with Postgres triggers blocking UPDATE/DELETE.

- **Threat:** log tampering by a privileged user.
- **Mitigation:** the trigger raises an exception; the hash chain breaks if any historical row is altered.
- **Test:** the Postgres trigger rejects `UPDATE audit_entries SET row_hash = 'tampered'`.

## Layer 14: Real-time audit

Postgres NOTIFY → WebSocket → browser in <1s.

- **Threat:** delayed incident detection.
- **Mitigation:** the browser receives new audit entries immediately, without polling.

## Layer 15: DB role verification

`main.go` queries `current_user` and `rolbypassrls` on startup.

- **Threat:** RLS bypass by misconfiguration (runtime role is superuser).
- **Mitigation:** the server refuses to start if the role can bypass RLS.

## Layer 16: Statement and lock timeouts

SET LOCAL statement_timeout = '10s', SET LOCAL lock_timeout = '5s'.

- **Threat:** query starvation (a pathological query holds a connection).
- **Mitigation:** the transaction is aborted after 10 seconds.
- **Test:** `TestStatementTimeoutAbortsSlowQuery`.

## Layer 17: CORS and security headers

Strict origin allowlist (`localhost:5173` and `localhost:5174`), CSP default-src 'none', X-Frame-Options: DENY, X-Content-Type-Options: nosniff.

- **Threat:** browser-based attacks, clickjacking, MIME sniffing.
- **Mitigation:** the middleware sets the headers on every response.

## Layer 18: OTLP tracing

`otelgqlgen` creates spans for every GraphQL operation; the OTLP exporter sends them to Jaeger.

- **Threat:** undiagnosable latency and errors.
- **Mitigation:** each request has a trace ID; a slow resolver or provider call is visible in Jaeger.
- **Fail-open:** if Jaeger is unreachable, the API continues without tracing.