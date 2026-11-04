# ADR 001: Modular Monorepo over Microservices

## Status
Accepted

## Context
ChronoBunker has three distinct runtime concerns: a Go HTTP/GraphQL API, two React frontends (the operations cockpit and the customer-facing Bunker Store), and the shared infrastructure (PostgreSQL with RLS, Redis for the breaker FSM, and a model registry).

The two architectural options were:
1. **Microservices**: separate deployable units for the API, the agent runtime, the tool gateway, and the frontend. Each with its own database connection, its own CI pipeline, and its own deployment.
2. **Modular monorepo**: a single deployable Go binary with clearly separated packages, plus a separately deployed frontend, plus a separately deployed evals engine.

## Decision
We chose the **modular monorepo**.

The Go backend is a single binary with `internal/` packages that enforce boundaries at compile time. The `internal/agent`, `internal/router`, `internal/breaker`, `internal/security`, and `internal/redteam` packages do not import each other except through explicit interfaces. The `cmd/api` binary wires them together.

The two frontends (`apps/reply-web` and `apps/bunker-store`) are separate deployables, but they live in the same repository because they evolve together.

## Consequences

**Positive:**
- **Atomic deploys:** A change to the breaker FSM and the resolver that uses it can be committed, tested, and deployed as one unit. No distributed version skew.
- **No network boundaries inside the backend:** The tool gateway calls the idempotency engine in-process. The cost is a function call, not a network round-trip.
- **Shared migrations:** The `db/migrations/` folder is the single source of truth. No distributed schema coordination.
- **Simpler CI:** One Go test suite, two frontends build. No service mesh, no Kubernetes orchestration required for the backend.

**Negative:**
- **Single point of failure:** If the Go binary crashes, the entire backend is down. There is no horizontal scaling of individual components.
- **Resource contention:** The HTTP server, the audit hub, and the red team harness share the same process. A runaway harness can starve the HTTP handlers.
- **Deployment coupling:** A change to the frontend requires redeploying the frontend, but a change to the backend requires redeploying the entire backend. There is no partial rollout.

## Mitigations
- The `internal/` package boundaries prevent accidental coupling between domains.
- The audit hub runs in a separate goroutine with its own context.
- The red team harness is gated behind a mutation that is only callable by authenticated admins.

## When to revisit
If the system needs to handle more than 10,000 req/s or needs multi-region failover, the modular monolith becomes a bottleneck. At that point, extract the `internal/router` and `internal/security` packages into separate services behind a message bus.