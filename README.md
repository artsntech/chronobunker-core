# ChronoBunker Core

> Production-grade AgentOps orchestrator for enterprise AI workloads.

ChronoBunker is a multi-tenant, zero-trust runtime that makes AI agents safe to
run in production: cryptographic capability tokens, exactly-once side effects,
database-enforced tenant isolation, and graceful degradation when upstream
model providers fail.

## Status

Phase 0: foundation only. The repository builds, lints, and tests, but no
business logic is implemented yet.

## Architecture

(Diagram and detailed description will be added in Phase 1.)

## Quickstart

```bash
cp .env.example .env
# Edit .env with your local Postgres/Redis credentials
make check-infra
make db-migrate
make build
make dev-api

## Quickstart

make test      # run Go tests
make lint      # go vet + gofmt

## License

AGPL-3.0 with a commercial dual-license. See LICENSE.

## Security Decisions

### CSRF Mitigation
ChronoBunker does not use cookies for authentication. JWTs are transmitted
exclusively via the `Authorization: Bearer` header. Browsers do not attach
custom headers to cross-origin requests automatically, which eliminates the
CSRF vector that affects cookie-based GraphQL APIs.

### Field-Level Authorization
The GraphQL `Agent` type does not expose `system_prompt` or any internal
routing configuration. Fields that are not in the schema cannot be queried,
regardless of the caller's permissions. This is field-level authorization by
schema design: the safest field is the one that does not exist.

### Query Depth and Complexity
The server enforces both a complexity limit (200) and a depth limit (10).
The complexity limit bounds the total number of fields; the depth limit
bounds recursion independently, preventing stack exhaustion from deeply
nested queries.