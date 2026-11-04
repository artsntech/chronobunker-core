// Package router implements the ChronoBunker model router.
package router

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/breaker"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/security"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/telemetry"
)

// Provider is the interface that every LLM provider must implement.
type Provider interface {
	Complete(ctx context.Context, modelID string, prompt string) (*Completion, error)
}

// Completion is the result of a single LLM call.
type Completion struct {
	Text         string
	InputTokens  int
	OutputTokens int
}

// Router routes completions through a fallback chain.
type Router struct {
	db       *sql.DB
	providers map[string]Provider // "groq" → GroqProvider, "gemini" → GeminiProvider
	policy   RoutingPolicy
	cache    *modelCache
	breaker  *breaker.Breaker
	sanitizer *security.OutputSanitizer // NEW
}

// ModelSpec is the cost metadata for a single model.
type ModelSpec struct {
	ID              string
	Provider        string
	InputCostPer1M  float64
	OutputCostPer1M float64
	ContextWindow   int
}

// RoutingPolicy is the YAML structure.
type RoutingPolicy struct {
	Version int                    `yaml:"version"`
	Routes  map[string]RouteConfig `yaml:"routes"`
}

// RouteConfig is a single route entry.
type RouteConfig struct {
	Primary        string   `yaml:"primary"`
	Fallback       []string `yaml:"fallback"`
	MaxCostPerCall float64  `yaml:"max_cost_per_call"`
}

// NewRouter loads the policy from YAML and initializes the model cache.
func NewRouter(db *sql.DB, providers map[string]Provider, br *breaker.Breaker, sanitizer *security.OutputSanitizer, policyPath string) (*Router, error) {
	policyBytes, err := os.ReadFile(policyPath)
	if err != nil {
		return nil, fmt.Errorf("router: read policy: %w", err)
	}
	var policy RoutingPolicy
	if err := yaml.Unmarshal(policyBytes, &policy); err != nil {
		return nil, fmt.Errorf("router: parse policy: %w", err)
	}

	return &Router{
		db:        db,
		providers: providers,
		policy:    policy,
		cache:     newModelCache(db),
		breaker:   br,
		sanitizer: sanitizer,
	}, nil
}

// Route tries the agent's target model first (if set), then the routing
// policy's primary and fallback chain. It returns the completion, the model
// ID that succeeded, the cost in USD, and an error if every model failed.
func (r *Router) Route(ctx context.Context, taskType, targetModel, prompt, sessionID string) (*Completion, string, float64, error) {
	route, ok := r.policy.Routes[taskType]
	if !ok {
		return nil, "", 0, fmt.Errorf("router: no route for task type %q", taskType)
	}

	// Build the chain: the agent's target model (if set) first, then the
	// policy's primary and fallback. Deduplicate so the same model is not
	// tried twice.
	chain := make([]string, 0, 2+len(route.Fallback))
	seen := make(map[string]bool)
	addModel := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			chain = append(chain, id)
		}
	}
	addModel(targetModel)
	addModel(route.Primary)
	for _, fb := range route.Fallback {
		addModel(fb)
	}

	var lastErr error
	for _, modelID := range chain {
		// Check the breaker before attempting the model.
		if r.breaker != nil {
			allowed, err := r.breaker.Allow(ctx, breaker.TargetModel, modelID)
			if err != nil {
				log.Printf("router: breaker check failed for %q: %v", modelID, err)
			} else if !allowed {
				log.Printf("router: breaker OPEN for %q, skipping", modelID)
				lastErr = fmt.Errorf("model %q circuit-broken", modelID)
				continue
			}
		}

		// Look up the model's cost metadata.
		spec, err := r.cache.Get(ctx, modelID)
		if err != nil {
			log.Printf("router: model %q not in registry: %v", modelID, err)
			lastErr = err
			if r.breaker != nil {
				_ = r.breaker.RecordResult(ctx, breaker.TargetModel, modelID, false)
			}
			continue
		}

		// Pick the provider by model ID prefix.
		provider, err := r.providerForModel(modelID)
		if err != nil {
			log.Printf("router: %v", err)
			lastErr = err
			if r.breaker != nil {
				_ = r.breaker.RecordResult(ctx, breaker.TargetModel, modelID, false)
			}
			continue
		}

		start := time.Now()
		completion, err := provider.Complete(ctx, modelID, prompt)
		latency := time.Since(start)

		if err != nil {
			log.Printf("router: model %q failed: %v", modelID, err)
			lastErr = err
			if r.breaker != nil {
				_ = r.breaker.RecordResult(ctx, breaker.TargetModel, modelID, false)
			}
			continue
		}

		// Sanitize the LLM output.
		if r.sanitizer != nil {
			cleanText, canaryLeak := r.sanitizer.Sanitize(completion.Text)
			if canaryLeak {
				log.Printf("SECURITY: canary leak detected from model %q; aborting", modelID)
				lastErr = fmt.Errorf("canary leak detected")
				if r.breaker != nil {
					_ = r.breaker.RecordResult(ctx, breaker.TargetModel, modelID, false)
				}
				continue
			}
			completion.Text = cleanText
		}

		cost := (float64(completion.InputTokens)*spec.InputCostPer1M +
			float64(completion.OutputTokens)*spec.OutputCostPer1M) / 1_000_000

		if route.MaxCostPerCall > 0 && cost > route.MaxCostPerCall {
			log.Printf("router: model %q exceeded max cost per call ($%.6f > $%.6f), trying fallback",
				modelID, cost, route.MaxCostPerCall)
			lastErr = fmt.Errorf("cost %.6f exceeds limit %.6f", cost, route.MaxCostPerCall)
			if r.breaker != nil {
				_ = r.breaker.RecordResult(ctx, breaker.TargetModel, modelID, true)
			}
			continue
		}

		if r.breaker != nil {
			_ = r.breaker.RecordResult(ctx, breaker.TargetModel, modelID, true)
		}

		// Emit the cost metric.
		if tenantID, err := db.TenantIDFromContext(ctx); err == nil {
			telemetry.RouterCostUSDTotal.WithLabelValues(tenantID, modelID).Add(cost)
		}

		if err := r.recordCost(ctx, sessionID, modelID, completion, cost, latency); err != nil {
			log.Printf("router: failed to record cost event: %v", err)
		}

		return completion, modelID, cost, nil
	}

	return nil, "", 0, fmt.Errorf("router: all models failed for task %q: %w", taskType, lastErr)
}

func (r *Router) recordCost(ctx context.Context, sessionID, modelID string, c *Completion, cost float64, latency time.Duration) error {
	tenantID, err := db.TenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	return db.WithTenantTx(ctx, r.db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO cost_events (tenant_id, session_id, model_id, input_tokens, output_tokens, cost_usd, latency_ms)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			tenantID, sessionID, modelID, c.InputTokens, c.OutputTokens, cost, latency.Milliseconds())
		return err
	})
}

// providerForModel returns the Provider for a modelID like "groq/openai/gpt-oss-120b" or "gemini/gemini-2.5-flash".
func (r *Router) providerForModel(modelID string) (Provider, error) {
	parts := strings.SplitN(modelID, "/", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("router: invalid model id %q", modelID)
	}
	prefix := parts[0]
	p, ok := r.providers[prefix]
	if !ok {
		return nil, fmt.Errorf("router: no provider for prefix %q", prefix)
	}
	return p, nil
}