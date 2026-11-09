package middleware

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/telemetry"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			attrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Duration("duration", time.Since(start)),
				slog.String("remote_addr", r.RemoteAddr),
			}

			if tenantID, err := db.TenantIDFromContext(r.Context()); err == nil {
				attrs = append(attrs, slog.String("tenant_id", tenantID))
			}

			logger.Info("http_request", attrs...)

			// Increment the Prometheus counter for HTTP requests.
			telemetry.HTTPRequestsTotal.WithLabelValues(
				r.Method,
				r.URL.Path,
				strconv.Itoa(rec.status),
			).Inc()
		})
	}
}

// Hijack delegates to the underlying ResponseWriter so that WebSocket
// upgrades can hijack the TCP connection. Without this, any middleware
// that wraps the writer blocks the WebSocket handshake.
func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}
