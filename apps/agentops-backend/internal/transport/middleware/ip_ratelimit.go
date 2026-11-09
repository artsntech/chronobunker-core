// Package middleware provides HTTP middleware for authentication, rate limiting,
// security headers, and other cross-cutting concerns.
package middleware

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// IPRateLimit enforces a per-IP request rate for unauthenticated endpoints.
// It uses a fixed-window counter: the first request of each window creates
// the key with a TTL equal to the window duration; subsequent requests in
// the same window increment it. When the counter exceeds the limit, the
// request is rejected with 429.
type IPRateLimit struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
}

// NewIPRateLimit constructs a Redis-backed per-IP limiter.
// limit is the maximum number of requests allowed in the given window.
func NewIPRateLimit(rdb *redis.Client, limit int, window time.Duration) *IPRateLimit {
	return &IPRateLimit{
		rdb:    rdb,
		limit:  limit,
		window: window,
	}
}

// Middleware enforces the rate limit.
func (rl *IPRateLimit) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if ip == "" {
			http.Error(w, `{"error":"cannot determine client IP"}`, http.StatusBadRequest)
			return
		}

		key := "ratelimit:ip:" + ip
		count, err := rl.rdb.Incr(r.Context(), key).Result()
		if err != nil {
			// Fail open: a Redis outage should not take down the health check.
			log.Printf("ip rate limit: incr failed: %v", err)
			next.ServeHTTP(w, r)
			return
		}
		if count == 1 {
			// The first request of the window creates the key. If EXPIRE
			// fails, the key persists indefinitely; the next test run will
			// see a stale counter. Log and continue.
			if err := rl.rdb.Expire(r.Context(), key, rl.window).Err(); err != nil {
				log.Printf("ip rate limit: expire failed: %v", err)
			}
		}

		remaining := rl.limit - int(count)
		if remaining < 0 {
			remaining = 0
		}
		w.Header().Set("RateLimit-Limit", strconv.Itoa(rl.limit))
		w.Header().Set("RateLimit-Remaining", strconv.Itoa(remaining))

		if int(count) > rl.limit {
			w.Header().Set("Retry-After", strconv.Itoa(int(rl.window.Seconds())))
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the client IP from X-Forwarded-For or RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
