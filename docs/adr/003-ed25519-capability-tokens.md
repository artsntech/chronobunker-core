# ADR 003: Ed25519 Capability Tokens over Session-Based Authorization

## Status
Accepted

## Context
AI agents need to execute tools (refunds, emails, database mutations) on behalf of users. The authorization must answer: "can this agent, in this session, on behalf of this user, invoke this tool, on this resource, for this amount?"

The naive approach is to pass the user's session token to the agent and let it act on the user's behalf. This is catastrophic. If the agent is prompt-injected, it can invoke any tool the user can invoke. The blast radius is the user's entire account.

The alternative is **capability tokens**: ephemeral, cryptographically signed tokens that grant specific permissions for a specific session.

## Decision
We chose **Ed25519 capability tokens** with the following properties:

```go
type CapabilityToken struct {
    SessionID    string
    TenantID     string
    UserID       string
    DeviceID     string   // device binding
    AllowedTools []string // e.g. ["search_help_center", "get_order"]
    ResourceIDs  []string // e.g. ["ORD-12345"]
    MaxAmount    float64  // e.g. 50.00
    ExpiresAt    int64    // 5 minutes from issuance
}
```

The token is signed with an ephemeral Ed25519 key pair generated at server startup. The private key never leaves the process. The public key is used to verify.

The Tool Gateway (`internal/security/gateway.go`) validates:
- **Signature:** the token was signed by the server.
- **Expiration:** the token has not expired.
- **Tenant match:** the token's `TenantID` matches the JWT's `tenant_id`.
- **Tool scope:** the requested tool is in `AllowedTools`.
- **Resource binding:** any resource ID in the params is in `ResourceIDs`.
- **Amount limit:** if the tool takes an amount, it is ≤ `MaxAmount`.
- **Device binding:** the `DeviceID` in the request matches the token's `DeviceID`.

If any check fails, the tool is not executed and the attempt is recorded in `audit_entries` with `decision = 'blocked'`.

## Consequences

**Positive:**
- **Minimal blast radius:** If a token is stolen, it expires in 5 minutes, is scoped to specific tools, and bound to a device.
- **No shared secret:** The private key stays on the issuer. The public key can be distributed to any validator.
- **Auditable:** Every validation, success or failure, is recorded in `audit_entries`.
- **Fast crypto:** Ed25519 is faster than RSA and has smaller keys (32 bytes).

**Negative:**
- **Token rotation fragility:** If the key pair is compromised, all issued tokens must be invalidated. There is no revocation list.
- **Key management:** In production, the private key must come from a secure manager like GCP Secret Manager, not disk.
- **No refresh:** A token expires in 5 minutes. Long-running operations require re-issuance.

## When to revisit
If the system needs to support long-running operations (e.g., a multi-step refund that takes minutes), the token TTL needs to be extended, or a refresh mechanism needs to be added.