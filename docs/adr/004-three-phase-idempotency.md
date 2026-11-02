# ADR 004: Three-Phase Idempotency over Single-Phase Retry

## Status
Accepted

## Context
AI agents execute side effects: refunds, emails, database writes. If the network fails mid-execution, the client retries. The retry must not execute the side effect twice.

The naive approach is to check if the operation already happened, then execute if not:

```go
if !alreadyExecuted(key) {
    execute()
    markExecuted(key)
}
```

This has a race condition: two concurrent retries can both pass the `alreadyExecuted` check and both execute.

## Decision
We chose the three-phase **RESERVE → EXECUTE → COMMIT** pattern:

- **Phase 1: RESERVE.** Insert a row in `idempotency_keys` with the deterministic key and `status = 'reserved'`. The key is `hash(session_id + tool_name + canonical_json(params))`. The insert uses `ON CONFLICT DO NOTHING`, which is atomic in Postgres. If the insert succeeds, this worker owns execution. If it fails, read the existing status (`reserved`, `executed`, `committed`, `failed`) to handle it safely.
- **Phase 2: EXECUTE.** Call the external service. If it fails, update status to `failed` and return.
- **Phase 3: COMMIT.** Update status to `committed` and store the response.

A reconciler (`internal/idempotency/reconciler.go`) runs in the background to resolve rows stuck in `reserved` or `executed` for more than 2 minutes.

## Consequences

**Positive:**
- **Exactly-once semantics:** A retry with the same intent returns the cached response, not a duplicate execution.
- **Crash-safe:** If the worker crashes between EXECUTE and COMMIT, the reconciler picks up the orphaned row.
- **Atomic protection:** `ON CONFLICT DO NOTHING` prevents any race conditions between the initial check and insertion.

**Negative:**
- **Storage overhead:** Every tool invocation stores a row in `idempotency_keys`. Needs a cleanup job with a 7-day TTL.
- **Complexity:** Three phases, four states, and a background reconciler.
- **Deterministic key fragility:** Parameters must be strictly canonicalized (e.g. `json.Marshal` in Go maps) otherwise the key changes.

## When to revisit
If the system needs to support more than 1000 tool invocations per second, the Postgres row locks become a bottleneck. At that point, use Redis for the RESERVE phase and Postgres only for the COMMIT log.