package graph_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/agent"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/router"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/graph"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/graph/model"
)

// mockProvider returns a fixed completion without calling an LLM.
type mockProvider struct{}

func (m *mockProvider) Complete(ctx context.Context, modelID, prompt string) (*router.Completion, error) {
	return &router.Completion{
		Text:         "mocked completion for: " + prompt,
		InputTokens:  10,
		OutputTokens: 5,
	}, nil
}

// TestInvokeAgentResolver calls the resolver directly with a mocked router.
// It proves that the resolver passes the prompt to the router and returns
// the completion and model ID.
func TestInvokeAgentResolver(t *testing.T) {
	database := testDB(t) // defined in e2e_test.go

	modelRouter, err := router.NewRouter(
		database,
		map[string]router.Provider{"groq": &mockProvider{}},
		nil, // breaker
		nil, // sanitizer
		policyPath(t),
	)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	resolver := &graph.Resolver{
		AgentRepo: agent.NewRepository(database),
		Router:    modelRouter,
		DB:        database,
	}

	ctx := db.WithTenantID(context.Background(), "11111111-1111-1111-1111-111111111111")
	agents, err := resolver.AgentRepo.List(ctx)
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	if len(agents) == 0 {
		t.Fatal("no agents found; run db-seed first")
	}

	// Pick the Acme Triage Agent explicitly. agents[0] may be the Chaos Agent,
	// which points at a non-existent model and would fail the test.
	var acmeAgent *agent.Agent
	for _, a := range agents {
		if a.Name == "Acme Triage Agent" {
			acmeAgent = a
			break
		}
	}
	if acmeAgent == nil {
		t.Fatal("Acme Triage Agent not found; run db-seed first")
	}

	input := model.InvokeAgentInput{
		AgentID:   acmeAgent.ID,
		Prompt:    "hello from test",
		SessionID: "test-session",
	}

	result, err := resolver.Mutation().InvokeAgent(ctx, input)
	if err != nil {
		t.Fatalf("invoke agent: %v", err)
	}

	if result.Completion == "" {
		t.Error("expected non-empty completion")
	}
	if result.ModelID == "" {
		t.Error("expected non-empty model ID")
	}
	if result.CostUsd < 0 {
		t.Error("expected non-negative cost")
	}
}

func policyPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("CHRONO_ROUTING_POLICY_PATH"); p != "" {
		return p
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "..")
	return filepath.Join(root, "policies", "routing.yaml")
}
