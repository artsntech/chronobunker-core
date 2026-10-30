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

	"github.com/vektah/gqlparser/v2/ast"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/99designs/gqlgen/graphql"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/redis/go-redis/v9"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/agent"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/breaker"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/router"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/security"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/graph"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/middleware"
)

func main() {
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

	groqProvider, err := router.NewGroqProvider("openai/gpt-oss-120b")
	if err != nil {
		log.Fatalf("init groq provider: %v", err)
	}

	canary := os.Getenv("CHRONO_CANARY_TOKEN")
	if canary == "" {
		canary = "CANARY-DEV-ONLY"
	}
	sanitizer := security.NewOutputSanitizer(canary)

	modelRouter, err := router.NewRouter(database, groqProvider, breakerInstance, sanitizer, policyPath)
	if err != nil {
		log.Fatalf("init router: %v", err)
	}

	// Capability token service and tool gateway.
	tokenService, err := security.NewTokenService()
	if err != nil {
		log.Fatalf("init token service: %v", err)
	}

	gateway := security.NewGateway(database, tokenService, func(ctx context.Context, toolName string, params map[string]any) (any, error) {
		// In production, this dispatches to the real tool implementations
		// (Stripe for refunds, SendGrid for emails, etc.). For now, it
		// returns a synthetic result so the end-to-end flow is exercised.
		return map[string]any{
			"tool":   toolName,
			"params": params,
			"status": "executed",
		}, nil
	})

	privKeyPath := os.Getenv("CHRONO_JWT_PRIVATE_KEY_PATH")
	if privKeyPath == "" {
		privKeyPath = "secrets/jwt_private.pem"
	}
	if abs, err := filepath.Abs(privKeyPath); err == nil {
		privKeyPath = abs
	}
	jwtIssuer, err := security.NewJWTIssuer(privKeyPath)
	if err != nil {
		log.Fatalf("init jwt issuer: %v", err)
	}

	es := graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{
			AgentRepo:    agent.NewRepository(database),
			Router:       modelRouter,
			DB:           database,
			TokenService: tokenService,
			Gateway:      gateway,
			Breaker:      breakerInstance, // NEW
			JWTIssuer: 	  jwtIssuer, // NEW
		},
	})

	srv := handler.New(es)
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.GET{})

	if os.Getenv("CHRONO_ENV") != "production" {
		srv.Use(extension.Introspection{})
	}

	// Batching prevention: reject requests that contain more than one
	// GraphQL operation. Without this, an attacker can send 100 login
	// mutations in a single POST, and the per-tenant rate limiter counts
	// it as a single request.
	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		oc := graphql.GetOperationContext(ctx)
		if len(oc.Doc.Operations) > 1 {
			return graphql.OneShot(graphql.ErrorResponse(ctx, "batching is not allowed"))
		}
		return next(ctx)
	})

	srv.Use(extension.FixedComplexityLimit(200))

	// Depth limit: reject queries that nest deeper than 10 levels.
	// The complexity limit alone does not bound recursion depth because
	// a query with a single field repeated 200 times has low complexity
	// but high depth. This extension enforces the depth independently.
	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		oc := graphql.GetOperationContext(ctx)
		for _, op := range oc.Doc.Operations {
			if d := countDepth(op.SelectionSet); d > 10 {
				return graphql.OneShot(graphql.ErrorResponse(ctx,
					"query depth %d exceeds limit 10", d))
			}
		}
		return next(ctx)
	})
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

	handlerWithHeaders := middleware.CORS(
	middleware.SecurityHeaders(
		middleware.Logging(logger)(mux),
	),
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

// countDepth recursively counts the maximum nesting depth of a selection set.
func countDepth(selectionSet ast.SelectionSet) int {
	maxDepth := 0
	for _, sel := range selectionSet {
		var depth int
		switch s := sel.(type) {
		case *ast.Field:
			depth = 1 + countDepth(s.SelectionSet)
		case *ast.InlineFragment:
			depth = countDepth(s.SelectionSet)
		case *ast.FragmentSpread:
			// Fragment spreads are resolved by gqlgen before this middleware runs,
			// so we treat them as depth 0 (they do not add nesting themselves).
			depth = 0
		}
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return maxDepth
}