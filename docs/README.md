# ChronoBunker

**Open-source AgentOps platform with bank-grade security. Self-hosted. Zero-trust. Exactly-once.**

ChronoBunker is a production-grade orchestrator for AI agents that execute real-world actions—refunds, emails, database mutations—under strict corporate constraints. Built in Go, React, and PostgreSQL, it combines row-level security, Ed25519 capability tokens, and exactly-once idempotency to make autonomous agents safe to run in regulated environments.

The frontend is not a dashboard. It is a **proof surface**: six interactive screens that demonstrate, with real backend events, that the security and resilience guarantees hold.

[![Go](https://shields.io)](https://go.dev)
[![React](https://shields.io)](https://react.dev)
[![PostgreSQL](https://shields.io)](https://postgresql.org)
[![License](https://shields.io)](LICENSE)

---

## Quickstart

**Prerequisites:** Go 1.22+, PostgreSQL 16+, Redis 7+, Node.js 20+.

```bash
git clone https://github.com
cd chronobunker-core
cp .env.example .env          # DB credentials, Groq API key, JWT secrets
./just db-migrate && ./just db-seed
bash scripts/generate_keys.sh
./just dev-api                # :8080 API, :9090 metrics
```

```bash
cd apps/reply-web
npm install && npm run dev    # :5173
```

Open `http://localhost:5173`. Log in as Acme Corp (tenant `11111111-...`).

## What the frontend proves

Six screens, each corresponding to a real backend guarantee. Every event is real—there is no simulation in production paths.

| Screen | What it proves | Backend layer |
|---|---|---|
| **Agent Monitor** | The system is observable. Runtime, latency, cost, breaker state. | Go runtime + Redis + `cost_events` |
| **Security Intercept** | Ed25519 blocks unauthorized tools. Every block is audited. | `security.Gateway` + `audit_entries` |
| **Breaker Lab** | The circuit breaker opens after 5 real failures. | `breaker.FSM` + Redis |
| **Tenant Isolation** | RLS blocks cross-tenant access at the database engine. | Postgres FORCE ROW LEVEL SECURITY |
| **Audit Trail** | The hash chain is cryptographically verifiable. | SHA-256 chain + Postgres triggers |
| **Red Team Harness** | The system tests itself against 5 OWASP scenarios. | Deterministic Go harness |

*Full details: `docs/red-team.md` and `docs/security-model.md`.*

## Red Team Harness

The cockpit runs an autonomous suite of 5 OWASP-mapped scenarios against the live backend. Each scenario has a deterministic PASS/FAIL assertion. Results are pushed to the browser over WebSocket.

| Scenario | OWASP | What it tests | Expected |
|---|---|---|---|
| **RT-001** | LLM01 | Prompt injection rejected | out of scope or fail-closed |
| **RT-002** | LLM06 | Unauthorized tool blocked | not allowed in session |
| **RT-003** | LLM02 | Cross-tenant access denied | null (RLS filtered) |
| **RT-004** | LLM10 | Idempotency enforced | cached: true on retry |
| **RT-005** | LLM10 | Circuit breaker trips | circuit-broken after 5 fails |

*Full details: `docs/red-team.md`.*

## Architecture

```text
Browser → JWT RS256 → GraphQL → Resolver → RLS-enforced query → Groq → Sanitizer → Cost ledger → Response
```

Every mutation is audited with a SHA-256 hash chain. The full topology is in `docs/architecture.md`.

## Security model (17 layers)

| Layer | Threat mitigated |
|---|---|
| **Postgres RLS** | Cross-tenant data leaks |
| **JWT RS256** | Token forgery (algorithm pinning, audience) |
| **Ed25519 capability tokens** | Unauthorized tool execution (5-min TTL, device-bound) |
| **3-phase idempotency** | Duplicate side effects (RESERVE → EXECUTE → COMMIT) |
| **Circuit breaker FSM** | Cascading LLM outages |
| **Output sanitization** | XSS / indirect prompt injection (`bluemonday` + `DOMPurify`) |
| **Canary tokens** | System prompt leakage |
| **Semantic intent gating** | Out-of-scope prompts (Groq JSON mode — OWASP LLM01) |
| **Depth + complexity limits** | GraphQL DoS |
| **Body limit** | Payload flooding (1 MiB) |
| **Rate limiting** | Abuse / brute force (per tenant + per IP) |
| **HTTP hardening** | Slowloris (`ReadHeaderTimeout`, graceful shutdown) |
| **Audit hash chain** | Log tampering + Postgres triggers |
| **Real-time audit** | Delayed incident detection (Postgres NOTIFY → WebSocket) |
| **DB role verification** | RLS bypass by misconfiguration (fail-fast on `BYPASSRLS`) |
| **Statement / lock timeouts** | Query starvation |
| **CORS + security headers** | Browser-based attacks |

*Full breakdown: `docs/security-model.md` and `docs/threat-model.md`.*

## Honest limitations

ChronoBunker is a working system, not a finished product. Key gaps:
- **Single-node:** No horizontal scaling, no multi-region, no leader election.
- **Groq single provider:** The `routing.yaml` references OpenAI/Anthropic/Gemini, but only the Groq adapter exists.
- **No SSO/SAML:** Enterprises require it. This is the first blocker.
- **No OpenTelemetry exporter:** Spans are currently discarded.
- **No semantic caching:** The `pgvector` extension is not enabled.
- **Device binding is stubbed:** The `DeviceID` claim is validated but the client does not sign requests.

*Full list: `docs/limitations.md`.*

## Testing

```bash
cd apps/agentops-backend
CHRONO_TEST_DB_URL="postgres://chrono_app:password@localhost:5432/chronobunker?sslmode=disable" \
CHRONO_REDIS_ADDR=localhost:6379 \
go test ./... -count=1
```

*Coverage by layer: see `docs/testing.md`.*

## Project structure

```text
chronobunker-core/
├── apps/
│   ├── agentops-backend/     # Go: router, gateway, breaker, audit, redteam
│   ├── reply-web/            # React: cockpit + proof surface
│   └── evals-engine/         # Python: LLM-as-judge (roadmap)
├── db/migrations/            # SQL: RLS, triggers, idempotency
├── policies/routing.yaml     # Model routing rules
└── docs/                     # Architecture, ADRs, threat model, runbooks
```

## Documentation

- **Architecture** — topology, data flow, components
- **Security model** — 17 layers, threat by threat
- **Threat model (STRIDE)** — formal analysis
- **Red team harness** — 5 scenarios, OWASP mapping
- **Limitations** — what is not done
- **Roadmap** — what comes next, ordered by value
- **Operations runbooks** — what to do when things fail
- **ADRs** — architecture decision records

## License

Core: GNU Affero General Public License v3.0 (AGPLv3). See `LICENSE`.

Commercial license: available for organizations that cannot accept AGPLv3 terms. Contact for pricing.

## Contributing

Contributions are welcome. Before submitting a PR: run `./just test` and `./just lint`. Open an issue first for major changes.

## Acknowledgements

Built with `gqlgen`, `go-redis`, `bluemonday`, `ECharts`, `Groq`, and PostgreSQL Row-Level Security.