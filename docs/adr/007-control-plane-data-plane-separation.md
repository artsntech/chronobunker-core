# ADR 007: Control Plane / Data Plane Separation

## Status
Accepted

## Context
ChronoBunker serves two distinct audiences with fundamentally different needs:

1. **Operators** (engineering, security, SRE) need to observe, control, and audit the system: runtime telemetry, breaker state, kill switches, audit trail verification, red team results.
2. **Customers** (end users of the Bunker Store) need to browse products, place orders, track shipments, and interact with a support agent.

These audiences have different authentication scopes, different UI paradigms, and different security requirements. Combining them into a single frontend would mean:
- The customer bundle ships with admin-only code (kill switch, audit chain verifier, red team harness).
- The operator UI is constrained by customer-facing design decisions (light mode, marketing copy).
- A compromise of the customer-facing app could expose admin functionality.

## Decision
We chose to build **two separate React frontends** that share the same backend:

| Frontend | Role | Port | Audience | UI Paradigm |
|---|---|---|---|---|
| `apps/reply-web` (Cockpit) | Control Plane | 5173 | Operators | Dark, monospace, telemetry-dense |
| `apps/bunker-store` | Data Plane | 5174 | Customers | Light, clean, commerce-focused |

Both frontends:
- Use the same `Mutation.login` to obtain a JWT RS256 token.
- Send `Authorization: Bearer <token>` to the same `/query` endpoint.
- Are subject to the same RLS policies, rate limits, and audit trail.

The **only** difference is the UI surface and the set of GraphQL operations each frontend calls. The backend does not know or care which frontend originated a request.

## Consequences

**Positive:**
- **Security boundary by surface.** The customer bundle does not contain admin mutations. Even if a customer reverse-engineers the GraphQL schema, the backend enforces `allowedTools` and RLS—the frontend is not the security boundary.
- **Independent deploys.** The cockpit can be redeployed without touching the store, and vice versa.
- **Audience-appropriate UX.** The cockpit is dark and telemetry-dense; the store is light and commerce-focused. No design compromise.
- **Shared backend investment.** RLS, Ed25519, idempotency, and the audit trail are built once and used by both.

**Negative:**
- **Duplicated auth logic.** Both frontends implement the same login flow and token storage. This is ~50 lines of duplicated code per frontend.
- **Two build pipelines.** CI must build and deploy two React apps. This is a configuration cost, not a code cost.
- **CORS allowlist.** The backend must allow both `localhost:5173` and `localhost:5174` (and their production equivalents).

## Why not a single frontend with role-based routing?
A single frontend that conditionally renders admin screens based on the JWT claims would:
- Ship admin code to customer browsers (even if not rendered).
- Require the backend to expose admin mutations to all authenticated users (relying on RLS/authorization at the resolver level).
- Blur the line between "operator error" and "customer error" in incident response.

The separation is a defense-in-depth measure: the customer's browser never downloads code that can call `killAgent` or `runRedTeam`.

## When to revisit
If the operational overhead of two frontends becomes prohibitive (e.g., a solo developer maintaining both), a single frontend with strict server-side authorization and code-splitting could be reconsidered. But for an enterprise product, the separation is the correct default.