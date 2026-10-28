package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/redis/go-redis/v9"
	_ "github.com/lib/pq"
	"database/sql"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/agent"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/graph"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/middleware"
)

func main() {
	// Fail fast: every dependency must be available before serving traffic.
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

	rdb := redis.NewClient(&redis.Options{Addr: rdbAddr})
	defer rdb.Close()

	auth, err := middleware.NewAuth(pubKeyPath)
	if err != nil {
		log.Fatalf("init auth: %v", err)
	}
	rateLimit := middleware.NewRateLimit(rdb, 100) // 100 req/s per tenant

	// Build the GraphQL server with OpenTelemetry tracing.
	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{
			AgentRepo: agent.NewRepository(database),
		},
	}))
	srv.Use(otelgqlgen.Middleware())

	// Chain: auth -> rate limit -> graphql.
	// Order matters: the rate limiter needs the tenant ID that auth injects.
	graphqlHandler := auth.Middleware(rateLimit.Middleware(srv))

	mux := http.NewServeMux()
	mux.Handle("/query", graphqlHandler)
	mux.Handle("/", playground.Handler("ChronoBunker", "/query"))

	// Health check is intentionally unauthenticated and not rate limited,
	// so load balancers and orchestrators can probe it without credentials.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	fmt.Println("chronobunker-api listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}