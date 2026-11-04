# Roadmap

Ordered by value to a prospective enterprise customer. Each item has an estimate in engineer-days.

## 1. OpenAI + Anthropic providers (Gemini done) (1.5 days for OpenAI + Anthropic)

The `Provider` interface is already in `internal/router/provider.go`. Each adapter is ~150 lines. The `routing.yaml` already lists the models. This unlocks the fallback chain that the policy promises but cannot deliver today.

**Why first:** without this, the fallback chain is a lie. A CTO will notice.

## 2. OpenTelemetry exporter (done)

The `otelgqlgen` middleware creates spans, but they are discarded. Add an OTLP exporter to send them to Jaeger or Tempo. This is what makes the system debuggable in production.

**Why second:** without this, the system is observable at the metric level but not at the trace level. A CTO will ask "how do I debug a slow request?".

## 3. Kubernetes manifests (2 days)

`deploy/k8s/deployment.yaml`, `service.yaml`, `statefulset.yaml` for Postgres, `configmap.yaml`, `secret.yaml`. Probes: `livenessProbe`, `readinessProbe`, `startupProbe`. The `/healthz` endpoint already exists.

## 4. SSO/SAML (5 days)

The first enterprise blocker. Requires an identity provider integration (Okta, Azure AD, Google Workspace). The `login` mutation would be replaced by a SAML assertion consumer.

**Why fourth:** no enterprise buys a platform without SSO. This is the single most requested feature in the roadmap.

## 5. Prompt registry with versioning and A/B testing (5 days)

Prompts are the new code. Treating them as config in a database column is a liability. A prompt registry with versioning, diff, rollback, and A/B testing is what production teams need.

**Why fifth:** without this, the system cannot be safely iterated. A prompt change is a production change.

## 6. Semantic caching with pgvector (3 days)

The `pgvector` extension is not enabled. The `ai_semantic_cache` table schema exists in the migration. The caching layer is ~300 lines of Go: embed the prompt, query the cache, return the cached response if similarity > 0.95.

**Why sixth:** a 60-90% cost reduction for repetitive workloads. This is a direct cost saving.

## 7. Multi-agent orchestration (15 days)

A planner agent that decomposes a task, workers that execute, and a conflict resolver that applies policy. This is where ChronoBunker stops being an AgentOps platform and becomes an autonomous operations platform.

**Why last:** it is the most ambitious item, and it requires all the infrastructure below it to be solid first.

## 8. Load tests with k6 (done)

A `k6` script that sends 1000 req/s to `/query` with a valid JWT, measures p50/p95/p99 latency, and verifies that the rate limiter and the circuit breaker behave correctly. Results documented in the README.

## 9. Chaos tests (2 days)

A `chaos_test.go` that kills the Kafka broker (or the Redis instance) mid-flight and verifies that the outbox retries and the reconciler self-heals. This is what proves the "effectively-once" claim.

## 10. Terraform for GCP (3 days)

`deploy/terraform/` with modules for Cloud Run, Cloud SQL, Memorystore, and Secret Manager. This is what makes the deployment reproducible.