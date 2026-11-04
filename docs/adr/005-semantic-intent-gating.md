# ADR 005: Semantic Intent Gating over Regex Prompt Blocking

## Status
Accepted

## Context
The first version of prompt validation used regex patterns to block known injection attempts (`ignore previous instructions`, etc.). This failed due to trivial bypasses (plurals, casing), infinite evasion paraphrases, and false positives on legitimate queries.

The OWASP LLM01 guidance is explicit: prompt injection is a semantic problem, not a syntax problem. String matching is not the solution.

## Decision
We chose semantic intent classification using a fast, cheap LLM (`openai/gpt-oss-20b` on Groq, with JSON mode):

```go
func (c *IntentClassifier) Classify(ctx context.Context, prompt, domain string) (*ClassifyResult, error) {
    // system prompt includes the agent's domain
    // the classifier returns {"intent": "...", "allowed": bool, "reason": "..."}
}
```

The classifier is called before the main LLM. If `allowed: false`, the request is rejected with `out of scope: <reason>`. The main LLM is never called. The domain scope is derived from the agent's department (e.g. `support` vs `chaos`).

## Consequences

**Positive:**
- **Semantically robust:** The classifier rejects paraphrases, translations, and novel injections.
- **Domain-aware:** The agent only responds within its declared operational scope.
- **Cost-efficient:** A classification call is ~$0.00002 compared to ~$0.0001 for a full completion. Saves 80% on bad requests.
- **Fail-closed:** If the classifier goes down, the system fails closed safely.

**Negative:**
- **Latency:** The classifier adds ~500-900ms to every single request, roughly doubling the overall response latency.
- **Provider dependency:** The classifier depends on Groq as primary and Gemini as fallback. If both are down, classification fails closed.
- **Bypass via manipulation:** A highly advanced prompt might still trick the classifier LLM.

## Mitigations
- **Parallel running:** In a future optimization, the classifier and main LLM can run in parallel for known-safe intents.
- **Provider fallback (implemented):** Groq is the primary classifier; Gemini is the fallback. If Groq fails, the classifier continues on Gemini.

## When to revisit
If the classifier's false positive rate is too high, the domain description needs to be refined. If false negatives are high, the classifier needs a stronger model or a supplementary rule-based layer.