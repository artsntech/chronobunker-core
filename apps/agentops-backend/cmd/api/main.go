package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"log/slog"
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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/redis/go-redis/v9"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/agent"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/breaker"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/router"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/graph"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/middleware"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/security"
)

func main() {
	// Structured logging: JSON in production, text in development.
	var logHandler slog.Handler
	if os.Getenv("CHRONO_ENV") == "production" {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

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

	policyPath := os.Getenv("CHRONO_ROUTING_POLICY_PATH")
	if policyPath == "" {
		policyPath = "policies/routing.yaml"
	}
	if abs, err := filepath.Abs(policyPath); err == nil {
		policyPath = abs
	}
	log.Printf("JWT public key: %s", pubKeyPath)
	log.Printf("routing policy: %s", policyPath)

	database, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	database.SetMaxOpenConns(25)
	database.SetMaxIdleConns(25)
	database.SetConnMaxLifetime(5 * time.Minute)
	database.SetConnMaxIdleTime(1 * time.Minute)

	

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
	//modelRouter, err := router.NewRouter(database, &router.MockProvider{}, breakerInstance, policyPath)
	canary := os.Getenv("CHRONO_CANARY_TOKEN")
if canary == "" {
	canary = "CANARY-DEV-ONLY" // development fallback
}
sanitizer := security.NewOutputSanitizer(canary)

modelRouter, err := router.NewRouter(database, &router.MockProvider{}, breakerInstance, sanitizer, policyPath)
	if err != nil {
		log.Fatalf("init router: %v", err)
	}

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

	if os.Getenv("CHRONO_ENV") != "production" {
		srv.Use(extension.Introspection{})
	}

	srv.Use(extension.FixedComplexityLimit(200))
	srv.Use(otelgqlgen.Middleware())

	graphqlHandler := auth.Middleware(rateLimit.Middleware(srv))

	ipRateLimit := middleware.NewIPRateLimit(rdb, 20, 10*time.Second)

	mux := http.NewServeMux()
	if os.Getenv("CHRONO_ENV") != "production" {
		mux.Handle("/", ipRateLimit.Middleware(playground.Handler("ChronoBunker", "/query")))
		mux.Handle("/playground", ipRateLimit.Middleware(playground.Handler("ChronoBunker", "/query")))
		mux.Handle("/playground/", ipRateLimit.Middleware(playground.Handler("ChronoBunker", "/query")))
	}
	mux.Handle("/query", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		graphqlHandler.ServeHTTP(w, r)
	}))
	mux.Handle("/healthz", ipRateLimit.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})))

	handlerWithHeaders := middleware.SecurityHeaders(
		middleware.Logging(logger)(mux),
	)

	httpSrv := &http.Server{
		Addr:              ":8080",
		Handler:           handlerWithHeaders,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		log.Println("metrics listening on :9090")
		if err := http.ListenAndServe(":9090", metricsMux); err != nil {
			log.Printf("metrics server: %v", err)
		}
	}()

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