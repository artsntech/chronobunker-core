// Package middleware provides HTTP middleware for authentication, rate limiting,
// security headers, and other cross-cutting concerns.
package middleware

import "net/http"

// SecurityHeaders adds defensive HTTP headers to every response.
// These headers mitigate common browser-based attacks (XSS, clickjacking,
// MIME sniffing) and are cheap to add.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-XSS-Protection", "1; mode=block")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// The API is JSON-only. A restrictive CSP prevents any accidental
		// HTML rendering from being exploited.
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
