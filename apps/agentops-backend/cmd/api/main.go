package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
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
if abs, err := filepath.Abs(pubKeyPath); err == nil {
	pubKeyPath = abs
}

	// The routing policy path is configurable so that the binary can be run
	// from any working directory. Defaults to policies/routing.yaml relative
	// to the working directory.
	policyPath := os.Getenv("CHRONO_ROUTING_POLICY_PATH")
	if policyPath == "" {
		policyPath = "policies/routing.yaml"
	}
	// Resolve to an absolute path so that the router works regardless of
	// where the binary is invoked from.
	if abs, err := filepath.Abs(policyPath); err == nil {
		policyPath = abs
	}

	log.Printf("routing policy: %s", policyPath)

	database, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	// Connection pool tuning.
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
	modelRouter, err := router.NewRouter(database, &router.MockProvider{}, breakerInstance, policyPath)
	if err != nil {
		log.Fatalf("init router: %v", err)
	}

	// Build the GraphQL server manually so that introspection can be
// conditionally enabled. NewDefaultServer always enables introspection.
es := graph.NewExecutableSchema(graph.Config{
	Resolvers: &graph.Resolver{
		AgentRepo: agent.NewRepository(database),
		Router:    modelRouter,
		DB:        database,
	},
})

srv := handler.New(es)
srv.AddTransport(transport.Options{})
srv.AddTransport(transport.POST{})
srv.AddTransport(transport.GET{})

// Introspection is only enabled in non-production environments.
if os.Getenv("CHRONO_ENV") != "production" {
	srv.Use(extension.Introspection{})
}

srv.Use(extension.FixedComplexityLimit(200))
srv.Use(otelgqlgen.Middleware())

	graphqlHandler := auth.Middleware(rateLimit.Middleware(srv))

	ipRateLimit := middleware.NewIPRateLimit(rdb, 20, 10*time.Second) // 20 req per 10s per IP

	mux := http.NewServeMux()
	if os.Getenv("CHRONO_ENV") != "production" {
    mux.Handle("/", ipRateLimit.Middleware(playground.Handler("ChronoBunker", "/query")))
}
	mux.Handle("/query", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		graphqlHandler.ServeHTTP(w, r)
	}))
mux.Handle("/playground", ipRateLimit.Middleware(playground.Handler("ChronoBunker", "/query")))
mux.Handle("/playground/", ipRateLimit.Middleware(playground.Handler("ChronoBunker", "/query")))
	mux.Handle("/healthz", ipRateLimit.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("healthz handler called: path=%q", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})))

	handlerWithHeaders := middleware.SecurityHeaders(mux)

	httpSrv := &http.Server{
		Addr:              ":8080",
		Handler:           handlerWithHeaders,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

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