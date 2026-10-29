package graph_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"
	"fmt"

	"github.com/99designs/gqlgen/graphql/handler"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/agent"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/security"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/graph"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/middleware"
)

func TestInvokeToolWithCapabilityToken(t *testing.T) {
	database := testDB(t)
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	auth, err := middleware.NewAuthFromKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("init auth: %v", err)
	}

	tokenService, _ := security.NewTokenService()
	gateway := security.NewGateway(database, tokenService, func(ctx context.Context, toolName string, params map[string]any) (any, error) {
		return map[string]any{"tool": toolName, "status": "executed"}, nil
	})

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{
			AgentRepo:    agent.NewRepository(database),
			DB:           database,
			TokenService: tokenService,
			Gateway:      gateway,
		},
	}))
	handler := auth.Middleware(srv)

	jwtToken := issueToken(t, privateKey, tenantAcme, time.Hour)

	// Unique session ID per run to avoid idempotency cache hits.
	sessionID := fmt.Sprintf("test-invoke-tool-%d", time.Now().UnixNano())

	resp := graphqlRequest(t, handler, jwtToken, `
		mutation InvokeTool($input: InvokeToolInput!) {
			invokeTool(input: $input) {
				executed
				cached
				result
			}
		}
	`, map[string]any{
		"input": map[string]any{
			"agentId":   agentAcmeID,
			"sessionId": sessionID,
			"deviceId":  "device-test",
			"toolName":  "search_help_center",
			"params":    map[string]any{"query": "how to track my order"},
		},
	})

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected response: %v", resp)
	}
	result, ok := data["invokeTool"].(map[string]any)
	if !ok {
		t.Fatalf("invokeTool missing in response: %v", resp)
	}
	if result["executed"] != true {
		t.Errorf("expected executed=true, got %v (cached=%v, result=%v)", result["executed"], result["cached"], result["result"])
	}
}

func TestInvokeToolRejectsUnauthorizedTool(t *testing.T) {
	database := testDB(t)
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	auth, _ := middleware.NewAuthFromKey(&privateKey.PublicKey)
	tokenService, _ := security.NewTokenService()
	gateway := security.NewGateway(database, tokenService, func(ctx context.Context, toolName string, params map[string]any) (any, error) {
		return map[string]any{"tool": toolName}, nil
	})

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{
			AgentRepo:    agent.NewRepository(database),
			DB:           database,
			TokenService: tokenService,
			Gateway:      gateway,
		},
	}))
	handler := auth.Middleware(srv)
	jwtToken := issueToken(t, privateKey, tenantAcme, time.Hour)

	resp := graphqlRequest(t, handler, jwtToken, `
		mutation InvokeTool($input: InvokeToolInput!) {
			invokeTool(input: $input) { executed }
		}
	`, map[string]any{
		"input": map[string]any{
			"agentId":   agentAcmeID,
			"sessionId": "test-invoke-tool-denied",
			"deviceId":  "device-test",
			"toolName":  "issue_refund",
			"params":    map[string]any{"order_id": "ORD-1234", "amount": 10.0, "reason": "test"},
		},
	})

	if _, hasErrors := resp["errors"]; !hasErrors {
		t.Fatalf("expected error for unauthorized tool, got: %v", resp)
	}
}