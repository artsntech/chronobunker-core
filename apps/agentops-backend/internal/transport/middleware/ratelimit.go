package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
)

// RateLimit enforces a per-tenant request rate using Redis-backed GCRA.
// The limit is keyed by tenant_id, not by IP, because tenants are the
// billing and abuse-prevention boundary in a multi-tenant system.
type RateLimit struct {
	limiter *redis_rate.Limiter
	limit   redis_rate.Limit
}

// NewRateLimit constructs a Redis-backed limiter. The limit applies per
// tenant per second. Adjust according to the pricing tier in production.
func NewRateLimit(rdb *redis.Client, perSecond int) *RateLimit {
	return &RateLimit{
		limiter: redis_rate.NewLimiter(rdb),
		limit:   redis_rate.PerSecond(perSecond),
	}
}

// Middleware enforces the rate limit. It must run after Auth.Middleware so
// that the tenant ID is already in the context.
func (rl *RateLimit) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, err := db.TenantIDFromContext(r.Context())
		if err != nil {
			http.Error(w, `{"error":"rate limit: missing tenant context"}`, http.StatusInternalServerError)
			return
		}

		key := "ratelimit:tenant:" + tenantID
		res, err := rl.limiter.Allow(r.Context(), key, rl.limit)
		if err != nil {
			http.Error(w, `{"error":"rate limiter unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("RateLimit-Remaining", strconv.Itoa(res.Remaining))

		if res.Allowed == 0 {
			retryAfter := int(res.RetryAfter / time.Second)
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}