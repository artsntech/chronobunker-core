// Package telemetry provides Prometheus metrics for the ChronoBunker backend.
package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestsTotal counts every HTTP request by method, path, and status.
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chronobunker_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	// BreakerState is a gauge: 0=closed, 1=half_open, 2=open.
	BreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "chronobunker_breaker_state",
			Help: "Circuit breaker state per target (0=closed, 1=half_open, 2=open).",
		},
		[]string{"target_type", "target_id"},
	)

	// RouterCostUSDTotal is a counter of total cost in USD by tenant and model.
	RouterCostUSDTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "chronobunker_router_cost_usd_total",
			Help: "Total cost in USD spent on LLM calls.",
		},
		[]string{"tenant_id", "model_id"},
	)

	// IdempotencyCacheHitsTotal counts cache hits in the idempotency engine.
	IdempotencyCacheHitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "chronobunker_idempotency_cache_hits_total",
			Help: "Total number of idempotency cache hits.",
		},
	)
)
