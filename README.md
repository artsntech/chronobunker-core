# ChronoBunker

**Open-source AgentOps platform with bank-grade security. Self-hosted. Zero-trust. Exactly-once.**

ChronoBunker is a production-grade orchestrator for AI agents that execute real-world actions—refunds, emails, database mutations—under strict corporate constraints. Built in Go, React, and PostgreSQL, it combines row-level security, Ed25519 capability tokens, and exactly-once idempotency to make autonomous agents safe to run in regulated environments.

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-AGPLv3-blue.svg)](LICENSE)
[![Postgres](https://img.shields.io/badge/PostgreSQL-16+-336791?logo=postgresql)](https://www.postgresql.org/)

---

## 🚀 Quickstart

**Prerequisites:** Go 1.22+, PostgreSQL 16+, Redis 7+, Node.js 20+.

```bash
# 1. Clone
git clone https://github.com/artsntech/chronobunker-core.git
cd chronobunker-core

# 2. Configure (copy and edit with your DB credentials and Groq API key)
cp .env.example .env

# 3. Initialize database and seed data
./just db-migrate
./just db-seed

# 4. Generate JWT keys
bash scripts/generate_keys.sh

# 5. Start the backend (exposes :8080 API, :9090 metrics)
./just dev-api

# 6. In another terminal, start the frontend
cd apps/reply-web
npm install
npm run dev

Open http://localhost:5173. Log in as Acme Corp (tenant 11111111-...), select the agent, and invoke it. You'll see the Groq LLM respond in real-time, the cost ledger update, and the breaker state in the cockpit.

Need Docker? See deploy/local/ for the portable compose stack (Postgres, Redis, Prometheus, Grafana).
🏗️ Architecture

ChronoBunker is a modular monolith with two halves that feed each other:
text

┌─────────────────────────────────────────────────────────────┐
│                    CHRONOBUNKER CORE                        │
│                                                             │
│  ┌──────────────────────┐    ┌──────────────────────────┐  │
│  │  AGENTOPS ENGINE     │    │  OPERATIONS COCKPIT      │  │
│  │  (Go)                │    │  (React + ECharts)       │  │
│  │                      │    │                          │  │
│  │  - Model Router      │◄───┤  - Live audit stream     │  │
│  │  - Circuit Breaker   │    │  - Go runtime telemetry  │  │
│  │  - Tool Gateway      │    │  - Latency chart         │  │
│  │  - Idempotency       │    │  - Kill switch           │  │
│  │  - Audit Trail       │    │  - Cost ledger           │  │
│  └──────────┬───────────┘    └──────────────────────────┘  │
│             │                                               │
│             ▼                                               │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  PostgreSQL (RLS) + Redis (Breaker) + Groq (LLM)    │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘

Data flow: Browser → JWT RS256 → GraphQL → Resolver → RLS-enforced query → Groq → Output sanitizer → Cost ledger → Response. Every mutation is audited with a SHA-256 hash chain.
🛡️ Security Model (17 layers)

ChronoBunker assumes the LLM is hostile, the frontend is compromised, and the network is untrusted. Every layer fails safe.
Layer	Threat mitigated	Implementation
Postgres RLS	Cross-tenant data leaks	FORCE ROW LEVEL SECURITY on every tenant-scoped table. A bug in Go cannot leak data.
JWT RS256	Token forgery	Algorithm pinning (WithValidMethods), audience validation, expiration required.
Ed25519 Capability Tokens	Unauthorized tool execution	5-minute TTL, scoped to specific tools and resource IDs, device-bound.
3-Phase Idempotency	Duplicate side effects (refunds, emails)	RESERVE → EXECUTE → COMMIT with ON CONFLICT DO NOTHING. Proven by TestGatewayIsIdempotentOnRetry.
Circuit Breaker FSM	Cascading LLM outages	CLOSED → OPEN → HALF_OPEN with exponential backoff. Trips after 5 consecutive failures.
Output Sanitization	XSS / indirect prompt injection	bluemonday allowlist on the backend, DOMPurify on the frontend. OWASP LLM05.
Canary Tokens	System prompt leakage	Secret token in system prompt; any leak aborts the response. OWASP LLM07.
Depth + Complexity Limits	GraphQL DoS	Max depth 10, max complexity 200, batching prevented.
Body Limit	Payload flooding	1 MiB max via http.MaxBytesReader.
Rate Limiting	Abuse / brute force	100 req/s per tenant, 20 req/10s per IP.
HTTP Hardening	Slowloris	ReadHeaderTimeout: 5s, WriteTimeout: 15s, graceful shutdown.
Audit Trail Hash Chain	Log tampering	SHA-256 chain with Postgres triggers blocking UPDATE/DELETE.
Real-time Audit	Delayed incident detection	Postgres NOTIFY → WebSocket subscription → browser in <1s.

What the tests prove: TestTenantIsolation (RLS), TestGatewayRejectsCrossTenantResourceBinding (Ed25519), TestGatewayIsIdempotentOnRetry (exactly-once), TestBreakerTripsAfterConsecutiveFailures (FSM), TestSanitizerRemovesScriptTags (OWASP LLM05).
📊 Operations Cockpit

The React frontend is not a demo—it's a working operations console:

    Go Runtime Telemetry: Goroutines, heap allocation, GC pause, heap objects (polled from /metrics every 3s).

    Latency Chart: ECharts line chart of the last 20 LLM invocations.

    Cost Ledger: Real-time USD spend per tenant.

    Breaker State: Live CLOSED/OPEN/HALF_OPEN from Redis.

    Kill Switch: Atomic updateAgentStatus with RLS-enforced isolation.

    Live Audit Stream: Color-coded security events (responses, tool executions, errors).

Screenshot: (add the cockpit screenshot here)
💰 Enterprise Edition (Planned)

The core is free and self-hosted. The enterprise tier adds what large organizations actually buy:
Feature	Community (AGPLv3)	Enterprise (Commercial)
Core AgentOps (RLS, Ed25519, idempotency, breaker)	✅	✅
Operations Cockpit (React)	✅	✅
Real-time audit subscriptions	✅	✅
SSO / SAML	❌	✅
Audit export to SIEM (Splunk, Datadog)	❌	✅
Multi-region failover	❌	✅
SLA-backed support	❌	✅
EU AI Act compliance reporting	❌	✅

Why open core? 96% of organizations maintain or increase open-source usage, but procurement teams still need something to buy . The free tier builds adoption; the enterprise tier pays the bills.
🧪 Testing
bash

# Run all tests (requires Postgres and Redis running)
cd apps/agentops-backend
CHRONO_TEST_DB_URL="postgres://chrono_app:password@localhost:5432/chronobunker?sslmode=disable" \
CHRONO_REDIS_ADDR=localhost:6379 \
go test ./... -count=1

Test coverage:

    internal/agent — RLS tenant isolation

    internal/security — Ed25519 tokens, idempotency, output sanitization, device binding

    internal/breaker — FSM transitions

    internal/router — fallback chain, cost ledger

    internal/transport/graph — GraphQL end-to-end with JWT

📁 Project Structure
text

chronobunker-core/
├── apps/
│   ├── agentops-backend/     # Go: router, gateway, breaker, audit
│   ├── reply-web/            # React: cockpit, login, audit trail
│   └── evals-engine/         # Python: LLM-as-judge (roadmap)
├── db/
│   ├── migrations/           # SQL: RLS, triggers, idempotency
│   └── seed/                 # Tenants, models, tools
├── policies/                 # YAML routing rules
├── deploy/
│   ├── local/                # Docker Compose (monitoring)
│   └── k8s/                  # Kubernetes manifests (roadmap)
└── docs/
    ├── adr/                  # Architecture Decision Records
    └── threat-model.md       # STRIDE analysis

🗺️ Roadmap

    ☑

    Multi-tenant RLS with Postgres engine enforcement
    ☑

    JWT RS256 with algorithm pinning and audience
    ☑

    Ed25519 capability tokens with device binding
    ☑

    3-phase idempotency (exactly-once side effects)
    ☑

    Circuit Breaker FSM with Redis state
    ☑

    Groq LLM integration (openai/gpt-oss-120b)
    ☑

    Operations cockpit with runtime telemetry
    ☑

    Real-time audit via Postgres LISTEN/NOTIFY + WebSocket
    □

    Prometheus + Grafana dashboards (Docker Compose)
    □

    k6 load tests + Chaos Engineering (breaker demonstration)
    □

    Kubernetes manifests with liveness/readiness/startup probes
    □

    OpenTelemetry exporter (Jaeger/Tempo)
    □

    Enterprise SSO/SAML and SIEM export

🤝 Contributing

Contributions are welcome. The project uses AGPLv3 for the core. Before submitting a PR, run ./just test and ./just lint.

For major changes, open an issue first to discuss the approach.
📄 License

Core: GNU Affero General Public License v3.0 (AGPLv3). See LICENSE.

Enterprise: Commercial license available. Contact for pricing.
🙏 Acknowledgements

Built with gqlgen, go-redis, bluemonday, ECharts, and Groq.