# ADR 006: Circuit Breaker FSM over Naive Retry

## Status
Accepted

## Context
LLM providers fail with rate limits, outages, or latency spikes. The naive approach of retrying with exponential backoff introduces wasted latency, unnecessary token costs, and can trigger cascading failures via provider overloads.

## Decision
We chose a circuit breaker FSM with three states:
- **CLOSED:** Normal operation. Requests pass through. Failures increment a counter.
- **OPEN:** Breaker has tripped. Requests are short-circuited instantly (~1ms) without calling the provider. Transitions to HALF_OPEN after a cooldown.
- **HALF_OPEN:** Testing recovery with a limited number of requests.

The state is stored centrally in **Redis** so multiple server replicas share it and it survives restarts. 

Transitions are verified atomically with an `opened_at` timestamp via Redis `TxPipeline`. If Redis itself fails, the system **fails open** to prevent taking down the entire platform on a caching layer hiccup.

## Consequences

**Positive:**
- **Immediate failure:** An OPEN breaker protects system latency and limits outage costs.
- **Contained blast radius:** Breakers are bound per-model, not globally.
- **Automatic recovery:** The FSM tests recovery safely without manual operations.
- **Observable:** Exposed as a Prometheus gauge (`chronobunker_breaker_state`) and in the cockpit.

**Negative:**
- **State complexity:** Hysteresis thresholds, exponential backoff, and 4 transitions increase bug surface.
- **Redis dependency:** If Redis is down, the breaker is blind.
- **Recovery delay:** Fixed cooldowns (e.g. 5 minutes) might delay using a provider that recovered in seconds.

## Why hysteresis
Without hysteresis (e.g. trip at 0.6 error rate, recover at 0.9 success), the breaker oscillates wildly on single failure fluctuations. Hysteresis ensures sustained evidence before state changes.

## When to revisit
If the system needs per-endpoint breakers (e.g. separate breakers for `search_help_center` and `issue_refund`), the FSM needs to support multiple target instances. The `TargetType` field is already in the schema to support this.