# Architecture

## Topology

```text
┌─────────────────────────────────────────────────────────────┐
│ CHRONOBUNKER CORE                                           │
│                                                             │
│ ┌──────────────────────┐     ┌──────────────────────────┐   │
│ │   AGENTOPS ENGINE    │     │  PROOF SURFACE (React)   │   │
│ │        (Go)          │     │                          │   │
│ │ - Model Router       │◄────┤ - Agent Monitor          │   │
│ │ - Circuit Breaker    │     │ - Security Intercept     │   │
│ │ - Tool Gateway       │     │ - Breaker Lab            │   │
│ │ - Idempotency        │     │ - Tenant Isolation       │   │
│ │ - Audit Trail        │     │ - Audit Trail            │   │
│ │ - Red Team Harness   │     │ - Red Team Harness       │   │
│ └──────────┬───────────┘     └──────────────────────────┘   │
│            │                                                │
│            ▼                                                │
│ ┌──────────────────────────────────────────────────────┐    │
│ │ PostgreSQL (RLS) + Redis (Breaker) + Groq (LLM)      │    │
│ └──────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## Data flow

1. **Browser** sends a GraphQL request with `Authorization: Bearer <JWT RS256>`.
2. **CORS middleware** validates the origin (`localhost:5173`).
3. **Auth middleware** validates the JWT (algorithm pinned to RS256, audience `chronobunker-api`, expiration required), injects `tenant_id` into the context.
4. **Rate limiter** enforces 100 req/s per tenant.
5. **GraphQL handler** enforces depth limit (10), complexity limit (200), batching prevention.
6. **Resolver** extracts `tenant_id` from context, opens a transaction, sets `SET LOCAL app.tenant_id`, queries Postgres.
7. **Postgres RLS** filters rows by `tenant_id`. A bug in Go cannot leak data.
8. **Intent classifier** (for `invokeAgent`) validates that the prompt is within the agent's domain.
9. **Groq provider** calls the LLM with the sanitized prompt.
10. **Output sanitizer** strips HTML from the LLM response.
11. **Cost ledger** records the invocation in `cost_events`.
12. **Audit trail** records the action with a SHA-256 hash chain.
13. **Postgres NOTIFY** triggers, the `audit.Hub` receives, and the WebSocket pushes to the browser.

## Components

| Package | Responsibility | Key file |
|---|---|---|
| `internal/agent` | Agent repository (RLS-enforced) | `repository.go` |
| `internal/audit` | Postgres LISTEN/NOTIFY hub | `hub.go` |
| `internal/breaker` | Circuit breaker FSM | `breaker.go` |
| `internal/db` | Tenant context, statement timeout | `tenant_conn.go` |
| `internal/redteam` | OWASP scenario harness | `harness.go` |
| `internal/router` | Model router + Groq + intent classifier | `router.go`, `classifier.go` |
| `internal/security` | Ed25519, sanitizer, gateway, JWT | `capability.go`, `gateway.go`, `jwt_issuer.go` |
| `internal/telemetry` | Prometheus metrics | `metrics.go` |
| `internal/transport` | GraphQL + middleware | `graph/`, `middleware/` |

## State stores

| Store | What it holds | Why |
|---|---|---|
| **PostgreSQL** | Tenants, agents, cost events, audit entries, idempotency keys | Source of truth. RLS enforces isolation. Hash chain prevents tampering. |
| **Redis** | Circuit breaker state, rate limit counters | Fast shared state across replicas. FSM survives restarts. |
| **Browser localStorage** | JWT, tenant ID, device ID | Session persistence. Cleared on logout. |

## Deployment

Local development: `./just dev-api` (Go binary) + `npm run dev` (Vite). Both run on the host. PostgreSQL and Redis are portable binaries.

Production (roadmap): Docker Compose with the Go binary, Postgres, Redis, Prometheus, and Grafana. Kubernetes manifests for the API and the workers.