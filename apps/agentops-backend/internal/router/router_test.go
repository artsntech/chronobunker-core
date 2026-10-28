package router_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"runtime"
	"path/filepath"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/router"
)

// mockProvider fails for the first N calls, then succeeds.
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

const tenantAcme = "11111111-1111-1111-1111-111111111111"

func TestRouterFallsBackOnProviderFailure(t *testing.T) {
	database := testDB(t)
	provider := &mockProvider{failures: 1} // fail first call, succeed second
	r, err := router.NewRouter(database, provider, policyPath(t))
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	ctx := db.WithTenantID(context.Background(), tenantAcme)
	completion, modelID, err := r.Route(ctx, "triage", "hello", "sess-test")
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	if completion.Text != "ok" {
		t.Errorf("expected ok, got %q", completion.Text)
	}
	// The first call failed, so the router must have used the fallback.
	if modelID == "groq/llama-3.1-8b" {
		t.Errorf("expected fallback model, got primary %q", modelID)
	}
}

func TestRouterRecordsCostEvent(t *testing.T) {
	database := testDB(t)
	provider := &mockProvider{failures: 0}
	r, err := router.NewRouter(database, provider, policyPath(t))
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	sessionID := fmt.Sprintf("sess-cost-%d", time.Now().UnixNano())

	ctx := db.WithTenantID(context.Background(), tenantAcme)
	_, _, err = r.Route(ctx, "triage", "hello", sessionID)
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

func policyPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("CHRONO_ROUTING_POLICY_PATH"); p != "" {
		return p
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	// thisFile is .../apps/agentops-backend/internal/router/router_test.go
	// Walk up to the project root, then into policies/.
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	return filepath.Join(root, "policies", "routing.yaml")
}