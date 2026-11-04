# ChronoBunker

**Open-source AgentOps platform with bank-grade security. Self-hosted. Zero-trust. Exactly-once.**

ChronoBunker is a production-grade orchestrator for AI agents that execute real-world actions—refunds, emails, database mutations—under strict corporate constraints. Built in Go, React, and PostgreSQL, it combines row-level security, Ed25519 capability tokens, and exactly-once idempotency to make autonomous agents safe to run in regulated environments.

The system has **two frontends**: a **Control Plane** (the operations cockpit for admins) and a **Data Plane** (the Bunker Store for customers). Both use the same JWT, the same RLS, the same Groq provider, and the same audit trail.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16+-336791?logo=postgresql)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/License-AGPLv3-blue.svg)](LICENSE)

---

## 🚀 Quickstart

**Prerequisites:** Go 1.22+, PostgreSQL 16+, Redis 7+, Node.js 20+.

```bash
git clone https://github.com/artsntech/chronobunker-core.git
cd chronobunker-core
cp .env.example .env          # DB credentials, Groq API key, JWT secrets
./just db-migrate && ./just db-seed
bash scripts/generate_keys.sh
./just dev-api                # :8080 API, :9090 metrics
```

```bash
# Cockpit (Control Plane)
cd apps/reply-web && npm install && npm run dev   # :5173

# Bunker Store (Data Plane)
cd apps/bunker-store && npm install && npm run dev  # :5174
```

Open http://localhost:5173 (cockpit) and http://localhost:5174 (store). Log in as Acme Corp (tenant 11111111-...) on both.

### 🎯 What the frontends prove

Six screens, each corresponding to a real backend guarantee. Every event is real—no simulation.

#### Cockpit (Control Plane, :5173)

| Screen | What it proves |
| :--- | :--- |
| **Agent Monitor** | Runtime telemetry (goroutines, heap, GC), latency chart, cost ledger, breaker state, kill switch, live audit stream |
| **Security Intercept** | Ed25519 blocks unauthorized tools (issue_refund outside allowedTools), audit trail records the block |
| **Breaker Lab** | Circuit breaker opens after 5 real failures, 6th request short-circuited in ~11ms |
| **Tenant Isolation** | Postgres RLS blocks cross-tenant access at the engine level, returns null |
| **Audit Trail** | SHA-256 hash chain verified in a Web Worker, tampering detection |
| **Red Team Harness** | 5 OWASP-mapped scenarios run deterministically: 5/5 PASS |

#### Bunker Store (Data Plane, :5174)

| Screen | What it proves |
| :--- | :--- |
| **Store** | Customer login (JWT RS256), product catalog, cart, checkout (createOrder), order history (RLS-isolated), support chat (intent-gated) |
| **Mission Lab** | Customer defines an autonomous mission (objective, budget, allowed tools). The agent executes. The server enforces MaxAmount and allowedTools. Blocked attempts are visible in real time. |

The two frontends share the same backend. A `createOrder` in the store appears in the cockpit's Live Audit Stream in <1s, via Postgres NOTIFY → WebSocket. A Mission Lab `issue_refund` that exceeds the budget is blocked by the gateway and recorded in the audit trail.

---

## 🏗️ Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                    CHRONOBUNKER CORE                        │
│                                                             │
│  ┌──────────────────────┐    ┌──────────────────────────┐  │
│  │  AGENTOPS ENGINE     │    │  CONTROL PLANE (React)   │  │
│  │  (Go)                │    │  :5173 — Cockpit         │  │
│  │                      │    │                          │  │
│  │  - Model Router      │◄───┤  - Agent Monitor         │  │
│  │  - Circuit Breaker   │    │  - Security Intercept    │  │
│  │  - Tool Gateway      │    │  - Breaker Lab           │  │
│  │  - Idempotency       │    │  - Tenant Isolation      │  │
│  │  - Audit Trail       │    │  - Audit Trail           │  │
│  │  - Intent Classifier │    │  - Red Team Harness      │  │
│  │  - Mission Runner    │    │                          │  │
│  └──────────┬───────────┘    └──────────────────────────┘  │
│             │                                               │
│             │                 ┌──────────────────────────┐  │
│             └────────────────►│  DATA PLANE (React)      │  │
│                               │  :5174 — Bunker Store    │  │
│                               │                          │  │
│                               │  - Store + Cart          │  │
│                               │  - Order History         │  │
│                               │  - Support Chat          │  │
│                               │  - Mission Lab           │  │
│                               └──────────────────────────┘  │
│             │                                               │
│             ▼                                               │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  PostgreSQL (RLS) + Redis (Breaker) + Groq (LLM)    │  │
│  └──────────────────────────────────────────────────────┐  │
└─────────────────────────────────────────────────────────────┘
```

---

Data flow: Browser → JWT RS256 → GraphQL → Resolver → RLS-enforced query → Groq → Output sanitizer → Cost ledger → Response. Every mutation is audited with a SHA-256 hash chain.

---

## 🛡️ Security Model (17 layers)

ChronoBunker assumes the LLM is hostile, the frontend is compromised, and the network is untrusted. Every layer fails safe.

| Layer | Threat mitigated | Implementation |
| :--- | :--- | :--- |
| **Postgres RLS** | Cross-tenant data leaks | FORCE ROW LEVEL SECURITY on every tenant-scoped table |
| **JWT RS256** | Token forgery | Algorithm pinning, audience validation, expiration required |
| **Ed25519 Capability Tokens** | Unauthorized tool execution | 5-min TTL, scoped to tools and resource IDs, device-bound |
| **3-Phase Idempotency** | Duplicate side effects | RESERVE → EXECUTE → COMMIT with ON CONFLICT DO NOTHING |
| **Circuit Breaker FSM** | Cascading LLM outages | CLOSED → OPEN → HALF_OPEN with exponential backoff |
| **Output Sanitization** | XSS / indirect prompt injection | bluemonday allowlist + DOMPurify. OWASP LLM05 |
| **Canary Tokens** | System prompt leakage | Secret token in prompt; leak aborts response. OWASP LLM07 |
| **Semantic Intent Gating** | Out-of-scope prompts | Groq gpt-oss-20b JSON-mode classifier. OWASP LLM01 |
| **Mission Budget Enforcement** | Agent overspend | MaxAmount on the capability token |
| **Depth + Complexity Limits** | GraphQL DoS | Max depth 10, max complexity 200, batching prevented |
| **Body Limit** | Payload flooding | 1 MiB via http.MaxBytesReader |
| **Rate Limiting** | Abuse / brute force | 100 req/s per tenant, 20 req/10s per IP |
| **HTTP Hardening** | Slowloris | ReadHeaderTimeout: 5s, graceful shutdown |
| **Audit Trail Hash Chain** | Log tampering | SHA-256 chain + Postgres triggers blocking UPDATE/DELETE |
| **Real-time Audit** | Delayed incident detection | Postgres NOTIFY → WebSocket in <1s |
| **DB Role Verification** | RLS bypass by misconfiguration | Fail-fast on BYPASSRLS role |
| **Statement / Lock Timeout** | Query starvation | SET LOCAL statement_timeout = '10s' |

---

## 🧪 Testing

```bash
cd apps/agentops-backend
CHRONO_TEST_DB_URL="postgres://chrono_app:password@localhost:5432/chronobunker?sslmode=disable" \
CHRONO_REDIS_ADDR=localhost:6379 \
go test ./... -count=1
```

### Coverage by layer

| Package | Tests | What it proves |
| :--- | :--- | :--- |
| `internal/agent` | TestTenantIsolation | RLS filters cross-tenant rows |
| `internal/security` | TestGatewayRejectsCrossTenantResourceBinding, TestGatewayIsIdempotentOnRetry, TestSanitizer* | Ed25519 scoping, exactly-once, OWASP LLM05/LLM07 |
| `internal/breaker` | TestBreakerTripsAfterConsecutiveFailures, TestBreakerTransitionsToHalfOpenAfterTimeout | FSM transitions |
| `internal/router` | TestRouterFallsBackOnProviderFailure, TestRouterRecordsCostEvent | Fallback chain + cost ledger |
| `internal/db` | TestStatementTimeoutAbortsSlowQuery | 10s query timeout |
| `internal/transport/graph` | TestGraphQLTenantIsolationEndToEnd, TestInvokeToolWithCapabilityToken | Full HTTP → JWT → RLS → response |

Red team harness: 5 OWASP-mapped scenarios run from the cockpit (RED TEAM → Run Full Suite). 5/5 PASS.

---

## 📊 Performance

Load test with k6 against the live backend (`createOrder` mutation, 50 req/s for 30s, 1500 requests):

| Metric | Result |
| :--- | :--- |
| **p95 latency** | 7.99ms |
| **p99 latency** | ~85ms |
| **Error rate** | 0.00% |
| **Throughput** | 49.85 req/s |

The `createOrder` mutation exercises RLS (`SET LOCAL app.tenant_id`), the audit trail (hash chain `INSERT`), and the tenant rate limiter (100 req/s). At 50 req/s sustained, none of these layers degrade.

The `invokeAgent` mutation is limited by Groq's free tier (~30 req/min), not by ChronoBunker.

---

## 💰 Enterprise Edition (Planned)

The core is free and self-hosted. The enterprise tier adds what large organizations actually buy:

| Feature | Community (AGPLv3) | Enterprise (Commercial) |
| :--- | :---: | :---: |
| Core AgentOps (RLS, Ed25519, idempotency, breaker) | ✅ | ✅ |
| Cockpit (Control Plane) | ✅ | ✅ |
| Bunker Store (Data Plane) | ✅ | ✅ |
| Red Team Harness | ✅ | ✅ |
| Mission Lab | ✅ | ✅ |
| SSO / SAML | ❌ | ✅ |
| Audit export to SIEM (Splunk, Datadog) | ❌ | ✅ |
| Multi-region failover | ❌ | ✅ |
| SLA-backed support | ❌ | ✅ |
| EU AI Act compliance reporting | ❌ | ✅ |

---

Why open core? The free tier builds adoption; the enterprise tier pays the bills.

---

## 📁 Project Structure

```text
chronobunker-core/
├── apps/
│   ├── agentops-backend/     # Go: router, gateway, breaker, audit, redteam
│   ├── reply-web/            # React: cockpit (Control Plane)
│   ├── bunker-store/         # React: store + Mission Lab (Data Plane)
│   └── evals-engine/         # Python: LLM-as-judge (roadmap)
├── db/
│   ├── migrations/           # SQL: RLS, triggers, idempotency, orders, store_actions
│   └── seed/                 # Tenants, models, tools, chaos agent, orders
├── policies/routing.yaml     # Model routing rules
├── deploy/
│   ├── local/                # Docker Compose (monitoring)
│   └── k8s/                  # Kubernetes manifests (roadmap)
└── docs/
    ├── adr/                  # 6 Architecture Decision Records
    ├── architecture.md
    ├── security-model.md
    ├── threat-model.md
    ├── red-team.md
    ├── operations.md
    ├── testing.md
    ├── limitations.md
    └── roadmap.md
```

---

## 📚 Documentation

* **Architecture** — topology, data flow, components
* **Security model** — 17 layers, threat by threat
* **Threat model (STRIDE)** — formal analysis
* **Red team harness** — 5 scenarios, OWASP mapping
* **Operations runbooks** — breaker, classifier, rate limit, audit, RLS
* **Testing** — test coverage by layer
* **Limitations** — what is not done (honest)
* **Roadmap** — what comes next, ordered by value
* **ADRs** — 6 architecture decisions

---

## 🗺️ Roadmap

* [x] Multi-tenant RLS with Postgres engine enforcement
* [x] JWT RS256 with algorithm pinning and audience
* [x] Ed25519 capability tokens with device binding
* [x] 3-phase idempotency (exactly-once side effects)
* [x] Circuit breaker FSM with Redis state
* [x] Groq LLM integration (openai/gpt-oss-120b)
* [x] Semantic intent gating (gpt-oss-20b)
* [x] Two frontends: cockpit (Control Plane) + Bunker Store (Data Plane)
* [x] Mission Lab with budget enforcement
* [x] Red team harness (5 OWASP scenarios)
* [x] k6 load tests (createOrder, 50 req/s, 0% errors)
* [ ] Prometheus + Grafana dashboards
* [ ] Kubernetes manifests with liveness/readiness/startup probes
* [ ] OpenTelemetry exporter (Jaeger/Tempo)
* [ ] Enterprise SSO/SAML and SIEM export

---

## 📄 License

* **Core:** GNU Affero General Public License v3.0 (AGPLv3). See LICENSE.
* **Enterprise:** Commercial license available. Contact for pricing.

---

## 🙏 Acknowledgements

Built with `gqlgen`, `go-redis`, `bluemonday`, `ECharts`, and `Groq`.