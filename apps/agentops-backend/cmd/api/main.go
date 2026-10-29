package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	_ "github.com/lib/pq"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/redis/go-redis/v9"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/agent"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/breaker"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/router"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/graph"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/middleware"
)

func main() {
	dbURL := os.Getenv("CHRONO_DB_URL")
	if dbURL == "" {
		log.Fatal("CHRONO_DB_URL is required")
	}
	rdbAddr := os.Getenv("CHRONO_REDIS_ADDR")
	if rdbAddr == "" {
		log.Fatal("CHRONO_REDIS_ADDR is required")
	}
	pubKeyPath := os.Getenv("CHRONO_JWT_PUBLIC_KEY_PATH")
	if pubKeyPath == "" {
		log.Fatal("CHRONO_JWT_PUBLIC_KEY_PATH is required")
	}

	database, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	// Connection pool tuning. MaxOpenConns bounds the number of concurrent
	// database connections; without it, a traffic spike exhausts the Postgres
	// max_connections limit and every tenant suffers.
	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(25)
	database.SetConnMaxLifetime(5 * time.Minute)
	database.SetConnMaxIdleTime(1 * time.Minute)

	// Verify the role cannot bypass RLS.
	var (
		currentUser string
		canBypass   bool
	)
	err = database.QueryRow(`
		SELECT current_user,
		       (SELECT rolbypassrls FROM pg_roles WHERE rolname = current_user)
	`).Scan(&currentUser, &canBypass)
	if err != nil {
		log.Fatalf("verify database role: %v", err)
	}
	if currentUser == "postgres" || canBypass {
		log.Fatalf("SECURITY: database role %q can bypass RLS. Use a non-superuser role without BYPASSRLS.", currentUser)
	}
	log.Printf("database role verified: %s (RLS enforced)", currentUser)

	rdb := redis.NewClient(&redis.Options{Addr: rdbAddr})
	defer rdb.Close()

	auth, err := middleware.NewAuth(pubKeyPath)
	if err != nil {
		log.Fatalf("init auth: %v", err)
	}
	rateLimit := middleware.NewRateLimit(rdb, 100)

	breakerInstance := breaker.New(rdb, database, breaker.DefaultConfig())
	modelRouter, err := router.NewRouter(database, &router.MockProvider{}, breakerInstance, "policies/routing.yaml")
	if err != nil {
		log.Fatalf("init router: %v", err)
	}

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{
			AgentRepo: agent.NewRepository(database),
			Router:    modelRouter,
			DB:        database,
		},
	}))
	srv.Use(otelgqlgen.Middleware())

	// Chain: security headers -> auth -> rate limit -> graphql.
	/*graphqlHandler := middleware.SecurityHeaders(
		auth.Middleware(rateLimit.Middleware(srv)),
	)*/
	graphqlHandler := auth.Middleware(rateLimit.Middleware(srv))

	mux := http.NewServeMux()
// graphqlHandler already includes SecurityHeaders, auth, and rate limit.
mux.Handle("/query", graphqlHandler)
mux.Handle("/", playground.Handler("ChronoBunker", "/query"))
mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
})

// Wrap the entire mux with SecurityHeaders so that every response, including
// /healthz and /playground, carries the defensive headers. The graphqlHandler
// also has SecurityHeaders inside it, but that is idempotent: setting the same
// headers twice is harmless.
handlerWithHeaders := middleware.SecurityHeaders(mux)

httpSrv := &http.Server{
	Addr:              ":8080",
	Handler:           handlerWithHeaders,
		ReadHeaderTimeout: 5 * time.Second,   // guards against Slowloris
		ReadTimeout:       15 * time.Second,  // bounds the entire request read
		WriteTimeout:      15 * time.Second,  // bounds the response write
		IdleTimeout:       60 * time.Second,  // keep-alive idle timeout
		MaxHeaderBytes:    1 << 20,           // 1 MiB header cap
	}

	// Graceful shutdown: on SIGINT or SIGTERM, stop accepting new connections,
	// wait for in-flight requests to finish (up to 15 seconds), then exit.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Println("chronobunker-api listening on :8080")
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutdown signal received; draining connections...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	} else {
		log.Println("server shut down cleanly")
	}
}