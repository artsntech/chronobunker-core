# Testing

## Running the tests

```bash
cd apps/agentops-backend
CHRONO_TEST_DB_URL="postgres://chrono_app:password@localhost:5432/chronobunker?sslmode=disable" \
CHRONO_REDIS_ADDR=localhost:6379 \
go test ./... -count=1
```

## Coverage by layer

| Package | Test | What it proves |
|---|---|---|
| `internal/agent` | `TestTenantIsolation` | RLS filters cross-tenant rows |
| `internal/security` | `TestGatewayRejectsCrossTenantResourceBinding` | Ed25519 resource binding |
| `internal/security` | `TestGatewayIsIdempotentOnRetry` | Exactly-once semantics |
| `internal/security` | `TestGatewayRejectsDeviceMismatch` | Device binding |
| `internal/security` | `TestSanitizerRemovesScriptTags` | OWASP LLM05 |
| `internal/security` | `TestSanitizerDetectsCanaryLeak` | OWASP LLM07 |
| `internal/security` | `TestSanitizerBlocksJavascriptURLs` | XSS mitigation |
| `internal/breaker` | `TestBreakerTripsAfterConsecutiveFailures` | FSM transitions |
| `internal/breaker` | `TestBreakerTransitionsToHalfOpenAfterTimeout` | Recovery |
| `internal/breaker` | `TestBreakerClosesAfterSuccessThreshold` | Full FSM cycle |
| `internal/router` | `TestRouterFallsBackOnProviderFailure` | Fallback chain |
| `internal/router` | `TestRouterRecordsCostEvent` | Cost ledger |
| `internal/db` | `TestStatementTimeoutAbortsSlowQuery` | 10s timeout |
| `internal/transport/graph` | `TestGraphQLTenantIsolationEndToEnd` | Full HTTP → JWT → RLS → response |
| `internal/transport/graph` | `TestInvokeToolWithCapabilityToken` | Ed25519 end-to-end |
| `internal/transport/graph` | `TestInvokeToolRejectsUnauthorizedTool` | Tool scoping |

## Red team harness

The red team harness (`internal/redteam/harness.go`) runs against the live backend via HTTP. It is not a Go test—it is a runtime verification. Run it from the cockpit or from CI (.github/workflows/redteam.yml).

## What is not tested

- **Chaos tests:** No `chaos_test.go` with Kafka/Redis failures. A future addition.
- **Frontend unit tests:** No Vitest setup. The frontend is tested via the red team harness (E2E).
- **Multi-region failover:** No leader election tests. The system is single-node.

**Coverage note:** the total statement coverage is 16.5%, but this number is misleading. The security-critical packages (`internal/security` 68%, `internal/breaker` 78%, `internal/db` 68%) are well covered. The lower total is driven by the integration test packages (`internal/transport/graph`, `internal/order`) and the modules that are tested via the red team harness in CI rather than `go test`. The red team harness covers the end-to-end security properties (RLS, Ed25519, idempotency, circuit breaker) with 5 deterministic scenarios.