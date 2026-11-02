package middleware

import "net/http"

// allowedOrigins is the allowlist of frontend origins that may call the API.
// Add new frontends here (e.g., mobile, staging) instead of editing the
// conditionals below.
var allowedOrigins = map[string]bool{
	"http://localhost:5173": true, // Cockpit (admin)
	"http://localhost:5174": true, // Bunker Store (customer)
}

// AllowedOrigins returns the list of allowed origins for WebSocket
// OriginPatterns configuration.
func AllowedOrigins() []string {
	out := make([]string, 0, len(allowedOrigins))
	for origin := range allowedOrigins {
		// Strip the scheme for WebSocket OriginPatterns.
		out = append(out, origin[len("http://"):])
	}
	return out
}

// CORS adds the required Access-Control headers for the allowed origins.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}