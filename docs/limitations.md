# Limitations

ChronoBunker is a working system, not a finished product. This document lists the gaps honestly.

## Infrastructure

- **Single-node.** No horizontal scaling, no multi-region, no leader election. The `audit.Hub` and the `breaker` assume one process. A production deployment needs Redis Cluster or Postgres advisory locks.
- **No TLS termination in the binary.** The Go server speaks HTTP. TLS must be terminated upstream (Nginx, Cloud Run, Fly.io).
- **No Kubernetes manifests.** Local development uses portable binaries (PostgreSQL, Redis, Jaeger). K8s is on the roadmap.
- **OTLP tracing verified locally with Jaeger, not in CI.** The exporter is wired in `main.go` and spans are visible in Jaeger (`http://localhost:16686`), but the CI pipeline does not yet spin up a Jaeger instance. A future step would add Jaeger as a service in the GitHub Actions workflow.

## AI / Agent layer

- **Groq and Gemini are the only implemented providers.** The `routing.yaml` references only these two. OpenAI and Anthropic are not implemented: the `Provider` interface is ready to accept an adapter, but there are no API keys available in the development environment to exercise the adapter against the real API. An unimplemented fallback to OpenAI would fail with `no provider for prefix "openai"`.
- **No semantic caching.** The `pgvector` extension is not enabled. Every request hits the LLM.
- **No prompt registry or A/B testing.** Prompts are hardcoded in the agent's `system_prompt` column.
- **Single-agent execution.** The system invokes one agent per request. No planner, no multi-agent orchestration.

## Security

- **No SSO/SAML in the Core.** Authentication is JWT-only. Enterprises require SSO. The Enterprise edition provides OIDC/SAML federation.
- **No SIEM export in the Core.** Audit events live in Postgres. A production deployment must export them to Splunk, Datadog, or Elastic. The Enterprise edition provides a CEF exporter.
- **Red team runs on push/PR, not as a pre-commit hook.** A developer can commit a regression locally without seeing the red team fail until the PR is pushed.
- **Device binding uses Ed25519 with browser-generated keys.** The browser generates a non-extractable Ed25519 key pair on first login and signs every `invokeTool` request. The server verifies the signature against the public key registered in the `device_keys` table. The signed payload is canonicalized (alphabetically sorted keys at every level) on both sides to ensure byte-identical serialization. Browsers without Web Crypto Ed25519 (Chrome < 137, Firefox < 129, Safari < 17) fall back to DeviceID-only validation; this degradation is documented in `device.ts`. Firefox does not persist `CryptoKey` in IndexedDB (Bugzilla #1348279, RESOLVED WONTFIX), so the key is regenerated on each reload and re-registered with the server.
- **No PII detection.** The output sanitizer strips HTML but does not detect PII in LLM responses.
- **No revocation list in the Core.** A stolen Ed25519 token is valid for its full 5-minute TTL. The Enterprise edition provides sub-millisecond Redis Pub/Sub revocation for API keys.

## Frontend

- **No role-based UI.** All authenticated users see all screens. RBAC is a roadmap item.
- **No replay or time machine.** The audit trail shows events, but there is no causal reconstruction.
- **`KNOWN_AGENT_IDS` is hardcoded.** The Tenant Isolation screen uses hardcoded agent IDs for the cross-tenant test.
- **No dark/light mode toggle.** The cockpit is dark-only.

o Firefox não persiste a chave privada no IndexedDB (Bugzilla WONTFIX), mas o device binding funciona durante a sessão.

## Testing

- **No chaos tests.** No `chaos_test.go` simulating Kafka/Redis failures.
- **No frontend unit tests.** The frontend is tested via the red team harness (E2E).
- **Fuzzing covers the sanitizer only.** The classifier and the router are not fuzzed.

## Deployment

- **No pre-built registry images.** Multi-stage `Dockerfile`s and `docker-compose.yml` are included for local and self-hosted builds, but pre-built images are not yet published to Docker Hub or GHCR.
- **No Terraform.** No infrastructure-as-code for GCP, AWS, or Oracle Cloud.
- **No multi-region.** No failover, no replication, no global load balancing. The Enterprise edition provides multi-region failover with distributed leader election.