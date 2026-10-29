// Package router implements the ChronoBunker model router.
package router

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

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
	provider Provider
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
func NewRouter(db *sql.DB, provider Provider, br *breaker.Breaker, sanitizer *security.OutputSanitizer, policyPath string) (*Router, error) {
	policyBytes, err := os.ReadFile(policyPath)
	if err != nil {
		return nil, fmt.Errorf("router: read policy: %w", err)
	}
	var policy RoutingPolicy
	if err := yaml.Unmarshal(policyBytes, &policy); err != nil {
		return nil, fmt.Errorf("router: parse policy: %w", err)
	}

	return &Router{
		db:       db,
		provider: provider,
		policy:   policy,
		cache:    newModelCache(db),
		breaker:  br,
		sanitizer: sanitizer,
	}, nil
}

// Route tries the primary model, then each fallback in order.
func (r *Router) Route(ctx context.Context, taskType, prompt, sessionID string) (*Completion, string, float64, error) {
	route, ok := r.policy.Routes[taskType]
	if !ok {
		return nil, "", 0, fmt.Errorf("router: no route for task type %q", taskType)
	}

	chain := append([]string{route.Primary}, route.Fallback...)

	var lastErr error
	for _, modelID := range chain {
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

		spec, err := r.cache.Get(ctx, modelID)
		if err != nil {
			log.Printf("router: model %q not in registry: %v", modelID, err)
			lastErr = err
			continue
		}

		start := time.Now()
		completion, err := r.provider.Complete(ctx, modelID, prompt)
		latency := time.Since(start)

		if err != nil {
			log.Printf("router: model %q failed: %v", modelID, err)
			lastErr = err
			if r.breaker != nil {
				_ = r.breaker.RecordResult(ctx, breaker.TargetModel, modelID, false)
			}
			continue
		}

		// Sanitize the LLM output before returning it to the caller.
		if r.sanitizer != nil {
			cleanText, canaryLeak := r.sanitizer.Sanitize(completion.Text)
			if canaryLeak {
				log.Printf("SECURITY: canary leak detected from model %q; aborting", modelID)
				lastErr = fmt.Errorf("canary leak detected")
				continue // try fallback model
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

		// Emit the cost metric. The tenant ID comes from the request context.
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