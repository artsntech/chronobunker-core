# Limitations

ChronoBunker is a working system, not a finished product. This document lists the gaps honestly.

## Infrastructure

- **Single-node.** No horizontal scaling, no multi-region, no leader election. The `audit.Hub` and the `breaker` assume one process. A production deployment needs Redis Cluster or Postgres advisory locks.
- **No TLS termination in the binary.** The Go server speaks HTTP. TLS must be terminated upstream (Nginx, Cloud Run, Fly.io).
- **No Kubernetes manifests.** Local development uses portable binaries (PostgreSQL, Redis, Jaeger). K8s is on the roadmap.
- **OTLP tracing verified locally with Jaeger, not in CI.** The exporter is wired in `main.go` and spans are visible in Jaeger (`http://localhost:16686`), but the CI pipeline does not yet spin up a Jaeger instance. A future step would add Jaeger as a service in the GitHub Actions workflow.

## AI / Agent layer

- **Groq and Gemini are the only implemented providers.** The `routing.yaml` references OpenAI, Anthropic, and Gemini, but only the Groq adapter exists. A fallback to OpenAI would fail with `provider not implemented`.
- **No semantic caching.** The `pgvector` extension is not enabled. Every request hits the LLM.
- **No prompt registry or A/B testing.** Prompts are hardcoded in the agent's `system_prompt` column.
- **Single-agent execution.** The system invokes one agent per request. No planner, no multi-agent orchestration.

## Security

- **No SSO/SAML.** Authentication is JWT-only. Enterprises require SSO.
- **No SIEM export.** Audit events live in Postgres. A production deployment must export them to Splunk, Datadog, or Elastic.
- **No red-team CI.** The red team harness runs at runtime from the cockpit, not in CI on every commit.
- **Device binding is stubbed.** The `DeviceID` claim is validated, but the client does not generate an ephemeral key pair.
- **No PII detection.** The output sanitizer strips HTML but does not detect PII in LLM responses.
- **No revocation list.** A stolen Ed25519 token is valid for its full 5-minute TTL. There is no way to revoke it early.

## Frontend

- **No role-based UI.** All authenticated users see all screens. RBAC is a roadmap item.
- **No replay or time machine.** The audit trail shows events, but there is no causal reconstruction.
- **`KNOWN_AGENT_IDS` is hardcoded.** The Tenant Isolation screen uses hardcoded agent IDs for the cross-tenant test.
- **No dark/light mode toggle.** The cockpit is dark-only.

## Testing

- **No load tests.** No k6, no vegeta, no results in the README.
- **No chaos tests.** No `chaos_test.go` simulating Kafka/Redis failures.
- **No frontend unit tests.** The frontend is tested via the red team harness (E2E).
- **No fuzzing.** No `go test -fuzz`.

## Deployment

- **No Docker image published.** There is no `Dockerfile` and no CI pipeline that builds a container image. The local development flow uses the Go binary directly.
- **No Terraform.** No infrastructure-as-code for GCP, AWS, or Oracle Cloud.
- **No multi-region.** No failover, no replication, no global load balancing.