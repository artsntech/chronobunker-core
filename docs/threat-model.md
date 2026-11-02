
---

## `docs/threat-model.md`

```markdown
# Threat Model (STRIDE)

ChronoBunker's threat model covers the AI agent, the tool execution layer, and the multi-tenant data store.

## Spoofing

| Threat | Mitigation |
|---|---|
| An attacker forges a JWT to impersonate a tenant | RS256 signature with algorithm pinning, audience validation, expiration required |
| An attacker reuses a stolen capability token on a different device | Device binding: the token's `DeviceID` must match the request's |
| An attacker replays a captured tool invocation | Deterministic idempotency key: the second invocation returns cache, not execution |

## Tampering

| Threat | Mitigation |
|---|---|
| An attacker modifies historical audit entries | SHA-256 hash chain + Postgres trigger blocking `UPDATE`/`DELETE` |
| A prompt-injected LLM modifies tool parameters (e.g., `order_id`) | Tool Gateway validates resource binding: the `order_id` must be in the token's `ResourceIDs` |
| A prompt-injected LLM inflates a refund amount | Tool Gateway validates `amount <= MaxAmount` from the token |
| An attacker modifies the LLM output to inject XSS | `bluemonday` allowlist on the backend + `DOMPurify` on the frontend |

## Repudiation

| Threat | Mitigation |
|---|---|
| A tenant denies having invoked a tool | Every invocation is recorded in `audit_entries` with the tenant ID, session ID, and hash chain |
| An operator denies having approved a refund | The approval is recorded in the audit trail with the operator's identity |

## Information Disclosure

| Threat | Mitigation |
|---|---|
| Cross-tenant data leak via a missing `WHERE` clause | Postgres RLS enforced at the engine level |
| System prompt leakage via prompt injection | Canary token in the system prompt; any leak aborts the response |
| PII in LLM output | Output sanitizer strips HTML; PII detection is a roadmap item |
| Tenant ID in error messages | Error messages are generic; the tenant ID is only in the audit log |

## Denial of Service

| Threat | Mitigation |
|---|---|
| GraphQL recursive query DoS | Depth limit (10) + complexity limit (200) |
| Body flooding | 1 MiB `MaxBytesReader` |
| Request flooding | 100 req/s per tenant + 20 req/10s per IP |
| Slowloris | `ReadHeaderTimeout: 5s` |
| Cascading provider outage | Circuit breaker FSM |
| Query starvation | `statement_timeout = 10s`, `lock_timeout = 5s` |

## Elevation of Privilege

| Threat | Mitigation |
|---|---|
| A prompt-injected LLM invokes a tool outside `allowedTools` | Tool Gateway rejects with `tool not allowed in this session` |
| A tenant attempts cross-tenant access via a known ID | RLS returns zero rows; the resolver returns `null` |
| An admin role bypasses RLS | `main.go` fail-fast on `BYPASSRLS` role |
| A compromised runtime role disables triggers | The audit trigger is `BEFORE UPDATE OR DELETE`, not policy-based; only a superuser DDL statement can disable it |

## Out of scope (documented)

- **Physical access** to the database server.
- **Supply chain attacks** on the Go module dependencies (partially mitigated by `go.sum`).
- **Model provider compromise** (Groq, OpenAI). The system assumes the LLM is hostile, but it does not verify the provider's infrastructure.
- **Side-channel attacks** on the Ed25519 signature verification.