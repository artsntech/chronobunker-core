# ChronoBunker

**Open-source AgentOps platform with bank-grade security. Self-hosted. Zero-trust. Exactly-once.**

ChronoBunker is a production-grade orchestrator for AI agents that execute real-world actions—refunds, emails, database mutations—under strict corporate constraints. Built in Go, React, and PostgreSQL, it combines row-level security, Ed25519 capability tokens, and exactly-once idempotency to make autonomous agents safe to run in regulated environments.

The frontend is not a dashboard. It is a **proof surface**: four interactive screens that demonstrate, with real backend events, that the security and resilience guarantees hold.

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react)](https://react.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16+-336791?logo=postgresql)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/License-AGPLv3-blue.svg)](LICENSE)

---

## Why this exists

Most "AI agent" projects are wrappers around an LLM API with a chat interface. They work in demos and break in production for three reasons:

1. **No permission model:** The agent can call any tool with any parameters. A prompt injection turns it into a remote code execution vector.
2. **No idempotency:** A network timeout causes the agent to retry. The customer gets two refunds, or two emails, or two shipments.
3. **No tenant isolation:** A single missing `WHERE tenant_id = ...` leaks data across customers.

ChronoBunker solves these at the infrastructure layer, not the application layer. Row-Level Security is enforced by PostgreSQL, not by Go code. Capability tokens are signed with Ed25519, not stored in a session. Idempotency is a 3-phase transaction in the database, not an in-memory map.

---

## Quickstart

**Prerequisites:** Go 1.22+, PostgreSQL 16+, Redis 7+, Node.js 20+.

```bash
# 1. Clone
git clone https://github.com/artsntech/chronobunker-core.git
cd chronobunker-core

# 2. Configure (DB credentials, Groq API key, JWT secrets)
cp .env.example .env

# 3. Initialize database and seed data
./just db-migrate
./just db-seed

# 4. Generate JWT RS256 keys
bash scripts/generate_keys.sh

# 5. Start the backend (:8080 API, :9090 metrics)
./just dev-api
```

In another terminal, start the frontend:

```bash
# 6. Start the frontend
cd apps/reply-web
npm install
npm run dev
```

Open `http://localhost:5173`. Log in as Acme Corp (tenant `11111111-...`). The cockpit appears.

---

## What the frontend proves

Four screens, each corresponding to a real backend guarantee. Every event is real—there is no simulation in production paths.

### 1. Agent Monitor (Operations Cockpit)
* **Shows:** Agent fleet with `breakerState` per agent (from Redis live), Go runtime metrics (goroutines, heap, GC pause via `/metrics`), Latency chart (last 20 invocations via ECharts), Cost ledger (24h via `cost_events` table), Real-time audit stream (GraphQL subscription over WebSocket), and a Kill switch (`updateAgentStatus` mutation).
* **Proves:** The system is observable. A CTO sees the Go process breathing.

### 2. Security Intercept
* **Shows:** Agent's `allowedTools` (`agents` table), "Attempt Unauthorized Tool" button (calls `invokeTool` with `issue_refund`), Backend rejection (`gateway: tool "issue_refund" not allowed in this session`), and Blocked event in audit trail (`audit_entries` via subscription).
* **Proves:** The Ed25519 capability token is scoped to the agent's tools. The LLM cannot execute what it was not granted. Every blocked attempt is recorded immutably.

### 3. Breaker Lab
* **Shows:** Chaos agent with invalid `targetModel` (`agents` table seeded), "Fire Request" button (calls `invokeAgent` with the chaos agent), Failure counter (frontend accumulator), FSM state transitions `CLOSED → OPEN` (`Agent.breakerState` in Redis), and Circuit-broken responses (`model "..." circuit-broken`).
* **Proves:** The Circuit Breaker opens after 5 real failures. The 6th request is short-circuited in ~11ms instead of waiting for a timeout.

### 4. Tenant Isolation
* **Shows:** Two tenants authenticated in parallel (Two JWT-scoped Apollo clients), Acme sees only Acme agents (RLS filters `SELECT * FROM agents`), "Try accessing [other tenant]'s agent" button (calls `agent(id)` with the other tenant's ID), and `0 rows returned` (RLS policy rejects the row).
* **Proves:** Row-Level Security is enforced by PostgreSQL, not by Go. Even if the application code had a bug, the database refuses to return cross-tenant rows.

---

## Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                    CHRONOBUNKER CORE                        │
│                                                             │
│  ┌──────────────────────┐    ┌──────────────────────────┐  │
│  │  AGENTOPS ENGINE     │    │  PROOF SURFACE (React)   │  │
│  │  (Go)                │    │                          │  │
│  │                      │    │  - Agent Monitor         │  │
│  │  - Model Router      │◄───┤  - Security Intercept    │  │
│  │  - Circuit Breaker   │    │  - Breaker Lab           │  │
│  │  - Tool Gateway      │    │  - Tenant Isolation      │  │
│  │  - Idempotency       │    │                          │  │
│  │  - Audit Trail       │    │                          │  │
│  └──────────┬───────────┘    └──────────────────────────┘  │
│             │                                               │
│             ▼                                               │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  PostgreSQL (RLS) + Redis (Breaker) + Groq (LLM)    │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

**Data flow:** Browser → JWT RS256 → GraphQL → Resolver → RLS-enforced query → Groq → Output sanitizer → Cost ledger → Response. Every mutation is audited with a SHA-256 hash chain.

---

## Security model (17 layers)

| Layer | Threat mitigated | Implementation |
| :--- | :--- | :--- |
| **Postgres RLS** | Cross-tenant data leaks | `FORCE ROW LEVEL SECURITY` on every tenant-scoped table |
| **JWT RS256** | Token forgery | Algorithm pinning, audience validation, expiration required |
| **Ed25519 Capability Tokens** | Unauthorized tool execution | 5-min TTL, scoped to tools and resource IDs, device-bound |
| **3-Phase Idempotency** | Duplicate side effects | `RESERVE → EXECUTE → COMMIT` with `ON CONFLICT DO NOTHING` |
| **Circuit Breaker FSM** | Cascading LLM outages | `CLOSED → OPEN → HALF_OPEN` with exponential backoff |
| **Output Sanitization** | XSS / indirect prompt injection | `bluemonday` allowlist (backend) + `DOMPurify` (frontend) |
| **Canary Tokens** | System prompt leakage | Secret token in prompt; leak aborts response |
| **Depth + Complexity Limits** | GraphQL DoS | Max depth 10, max complexity 200, batching prevented |
| **Body Limit** | Payload flooding | 1 MiB via `http.MaxBytesReader` |
| **Rate Limiting** | Abuse / brute force | 100 req/s per tenant, 20 req/10s per IP |
| **HTTP Hardening** | Slowloris | `ReadHeaderTimeout: 5s`, graceful shutdown |
| **Audit Trail Hash Chain** | Log tampering | SHA-256 chain + Postgres triggers blocking `UPDATE`/`DELETE` |
| **Real-time Audit** | Delayed incident detection | Postgres `NOTIFY` → WebSocket → browser in <1s |
| **DB Role Verification** | RLS bypass by misconfiguration | Fail-fast if the runtime role is superuser or has `BYPASSRLS` |
| **Statement / Lock Timeout** | Query starvation | `SET LOCAL statement_timeout = '10s'` per transaction |
| **CORS + Security Headers** | Browser-based attacks | Strict allowlist, CSP, frame-ancestors |
| **Graceful Shutdown** | Data corruption on deploy | `signal.NotifyContext` + drain window |

---

## Honest limitations

ChronoBunker is a working system, not a finished product. These are the gaps:

### Infrastructure
* **Single-node:** No horizontal scaling, no multi-region, no leader election across replicas. The Hub and Breaker assume one process. A production deployment needs Redis Cluster or Postgres advisory locks for coordination.
* **No TLS termination in the binary:** The Go server speaks HTTP. TLS must be terminated upstream (Nginx, Cloud Run, Fly.io). This is standard but means the binary alone is not production-secure.
* **No OpenTelemetry exporter:** `otelgqlgen` creates spans but they are discarded. For production observability, add an OTLP exporter to Jaeger or Tempo.
* **No Kubernetes manifests yet:** Docker Compose works locally. K8s is on the roadmap.

### AI / Agent layer
* **Groq is the only implemented provider:** The `routing.yaml` references OpenAI, Anthropic, and Gemini, but only the Groq adapter exists. A fallback to OpenAI would fail with provider not implemented.
* **No semantic caching:** The `pgvector` extension is not enabled. Every request hits the LLM. A production deployment should cache frequent prompts.
* **No prompt registry or A/B testing:** Prompts are hardcoded in the agent's `system_prompt` column. There is no versioning, no rollback, no experiment framework.
* **Single-agent execution:** The system invokes one agent per request. There is no multi-agent orchestration, no planner, no conflict resolution between agents.

### Security
* **No SSO/SAML:** Authentication is JWT-only. Enterprises require SSO. This is the first thing a CISO will ask for.
* **No SIEM export:** Audit events live in Postgres. A production deployment must export them to Splunk, Datadog, or Elastic.
* **No red-team CI:** The `bluemonday` and `DOMPurify` layers are tested in unit tests, but there is no automated adversarial suite running on every commit.
* **Device binding is stubbed:** The `DeviceID` claim is validated, but the client does not generate an ephemeral key pair. A real implementation would sign each request with a device-held private key.

### Frontend
* **No role-based UI:** All authenticated users see all screens. A production deployment needs RBAC (viewer, operator, admin) enforced by the backend.
* **No replay or time machine:** The audit trail shows events, but there is no causal reconstruction. Correlating events requires a `trace_id` column, which does not exist.
* **Hardcoded references:** The `KNOWN_AGENT_IDS` constant in Tenant Isolation is hardcoded. It should be loaded from a discovery query.

---

## What I would build next (in order)

1. **OpenAI + Anthropic + Gemini providers:** Each is ~2 hours of work. The `Provider` interface is already in place. This unlocks the fallback chain that `routing.yaml` promises but cannot deliver today.
2. **OpenTelemetry exporter:** Send spans to Jaeger. This is what makes the system debuggable in production. ~4 hours.
3. **Kubernetes manifests with liveness/readiness/startup probes:** Docker Compose works locally; K8s is what enterprises run. ~6 hours.
4. **SSO/SAML:** The first enterprise blocker. Requires an identity provider integration. ~2 days.
5. **Prompt registry with versioning and A/B testing:** Prompts are the new code. Treating them as config is a liability. ~1 week.
6. **Semantic caching with pgvector:** A 60-90% cost reduction for repetitive workloads. The `pgvector` extension and the `ai_semantic_cache` table schema already exist in the migrations. Only the caching layer is missing. ~3 days.
7. **Multi-agent orchestration:** A planner agent that decomposes a task, workers that execute, and a conflict resolver that applies policy. This is where ChronoBunker stops being an AgentOps platform and becomes an autonomous operations platform. ~2-3 weeks.

---

## Testing

```bash
# Run all tests (requires Postgres and Redis running)
cd apps/agentops-backend
CHRONO_TEST_DB_URL="postgres://chrono_app:password@localhost:5432/chronobunker?sslmode=disable" \
CHRONO_REDIS_ADDR=localhost:6379 \
go test ./... -count=1
```

### Test coverage by layer:

| Package | Tests | What it proves |
| :--- | :--- | :--- |
| `internal/agent` | `TestTenantIsolation` | RLS filters cross-tenant rows |
| `internal/security` | `TestGatewayRejectsCrossTenantResourceBinding`, `TestGatewayIsIdempotentOnRetry`, `TestSanitizer*` | Ed25519 scoping, exactly-once, OWASP LLM05/LLM07 |
| `internal/breaker` | `TestBreakerTripsAfterConsecutiveFailures`, `TestBreakerTransitionsToHalfOpenAfterTimeout` | FSM transitions |
| `internal/router` | `TestRouterFallsBackOnProviderFailure`, `TestRouterRecordsCostEvent` | Fallback chain + cost ledger |
| `internal/db` | `TestStatementTimeoutAbortsSlowQuery` | 10s query timeout |
| `internal/transport/graph` | `TestGraphQLTenantIsolationEndToEnd, TestInvokeToolWithCapabilityToken` | Full HTTP → JWT → RLS → response |

---

## Project structure

```text
chronobunker-core/
├── apps/
│   ├── agentops-backend/     # Go: router, gateway, breaker, audit
│   │   ├── cmd/api/          # HTTP server entrypoint
│   │   ├── cmd/worker/       # Background worker (roadmap)
│   │   ├── cmd/cli/          # Admin CLI (roadmap)
│   │   └── internal/
│   │       ├── agent/        # Agent repository (RLS-enforced)
│   │       ├── audit/        # Postgres LISTEN/NOTIFY hub
│   │       ├── breaker/      # FSM with Redis state
│   │       ├── db/           # Tenant context + statement timeout
│   │       ├── router/       # Model router + Groq provider
│   │       ├── security/     # Ed25519, sanitizer, gateway, JWT issuer
│   │       ├── telemetry/    # Prometheus metrics
│   │       └── transport/    # GraphQL + middleware
│   ├── reply-web/            # React: cockpit + proof surface
│   └── evals-engine/         # Python: LLM-as-judge (roadmap)
├── db/
│   ├── migrations/           # SQL: RLS, triggers, idempotency
│   └── seed/                 # Tenants, models, tools, chaos agent
├── policies/
│   └── routing.yaml          # Model routing rules
├── deploy/
│   ├── local/                # Docker Compose (roadmap)
│   └── k8s/                  # Kubernetes manifests (roadmap)
└── docs/
    └── adr/                  # Architecture Decision Records (roadmap)
```

---

## License

**Core:** GNU Affero General Public License v3.0 (AGPLv3). See [`LICENSE`](LICENSE).

The AGPLv3 ensures that if you run ChronoBunker as a network service, any modifications you make must also be open-sourced. This is the same license used by Grafana and Mattermost.

**Commercial license:** available for organizations that cannot accept AGPLv3 terms. Contact for pricing.

---

## Contributing & Acknowledgements

* **Contributing:** Contributions are welcome. Before submitting a PR, run `./just test` and `./just lint`. Open an issue first for major changes and document trade-offs in the description.
* **Acknowledgements:** Built with `gqlgen`, `go-redis`, `bluemonday`, `ECharts`, `Groq`, and PostgreSQL Row-Level Security.