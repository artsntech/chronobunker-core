package router_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/router"
)

type mockProvider struct {
	failures int32
	calls    int32
}

func (m *mockProvider) Complete(ctx context.Context, modelID, prompt string) (*router.Completion, error) {
	atomic.AddInt32(&m.calls, 1)
	if atomic.AddInt32(&m.failures, -1) >= 0 {
		return nil, errors.New("simulated provider failure")
	}
	return &router.Completion{Text: "ok", InputTokens: 10, OutputTokens: 5}, nil
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("CHRONO_TEST_DB_URL")
	if url == "" {
		t.Skip("CHRONO_TEST_DB_URL not set")
	}
	conn, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
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
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	return filepath.Join(root, "policies", "routing.yaml")
}

const tenantAcme = "11111111-1111-1111-1111-111111111111"

func TestRouterFallsBackOnProviderFailure(t *testing.T) {
	database := testDB(t)
	provider := &mockProvider{failures: 1}
	r, err := router.NewRouter(database, map[string]router.Provider{"groq": provider}, nil, nil, policyPath(t))
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	ctx := db.WithTenantID(context.Background(), tenantAcme)
	completion, modelID, _, err := r.Route(ctx, "triage", "", "hello", "sess-test")
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if completion.Text != "ok" {
		t.Errorf("expected ok, got %q", completion.Text)
	}
	if modelID == "groq/llama-3.1-8b" {
		t.Errorf("expected fallback model, got primary %q", modelID)
	}
}

func TestRouterRecordsCostEvent(t *testing.T) {
	database := testDB(t)
	provider := &mockProvider{failures: 0}
	r, err := router.NewRouter(database, map[string]router.Provider{"groq": provider}, nil, nil, policyPath(t))
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	sessionID := fmt.Sprintf("sess-cost-%d", time.Now().UnixNano())
	ctx := db.WithTenantID(context.Background(), tenantAcme)
	_, _, _, err = r.Route(ctx, "triage", "", "hello", sessionID)
	if err != nil {
		t.Fatalf("route: %v", err)
	}

	var count int
	err = db.WithTenantTx(ctx, database, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM cost_events WHERE session_id = $1`, sessionID).Scan(&count)
	})
	if err != nil {
		t.Fatalf("query cost_events: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 cost event, got %d", count)
	}
}