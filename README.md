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