package security_test

import (
	"context"
	"database/sql"
	"os"
	"sync/atomic"
	"testing"

	_ "github.com/lib/pq"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/security"
)

const tenantAcme = "11111111-1111-1111-1111-111111111111"

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

func TestGatewayRejectsCrossTenantResourceBinding(t *testing.T) {
	database := testDB(t)
	tokens, _ := security.NewTokenService()

	// Token grants access only to order ORD-1234, with a $50 max.
	signed, err := tokens.Issue("sess-1", tenantAcme, "user-1",
		[]string{"issue_refund"}, []string{"ORD-1234"}, 50.0)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	var executed int32
	gw := security.NewGateway(database, tokens, func(ctx context.Context, name string, params map[string]any) (any, error) {
		atomic.AddInt32(&executed, 1)
		return "refunded", nil
	})

	ctx := db.WithTenantID(context.Background(), tenantAcme)
	_, err = gw.Invoke(ctx, security.ToolRequest{
		SignedToken: signed,
		ToolName:    "issue_refund",
		Params:      map[string]any{"order_id": "ORD-9999", "amount": 10.0, "reason": "test"},
	})
	if err == nil {
		t.Fatal("expected cross-tenant resource binding to be rejected")
	}
	if atomic.LoadInt32(&executed) != 0 {
		t.Fatal("executor must not be called when binding is violated")
	}
}

func TestGatewayIsIdempotentOnRetry(t *testing.T) {
	database := testDB(t)
	tokens, _ := security.NewTokenService()
	signed, _ := tokens.Issue("sess-2", tenantAcme, "user-1",
		[]string{"issue_refund"}, []string{"ORD-1234"}, 50.0)

	var calls int32
	gw := security.NewGateway(database, tokens, func(ctx context.Context, name string, params map[string]any) (any, error) {
		atomic.AddInt32(&calls, 1)
		return map[string]any{"refund_id": "R-1"}, nil
	})

	ctx := db.WithTenantID(context.Background(), tenantAcme)
	req := security.ToolRequest{
		SignedToken: signed,
		ToolName:    "issue_refund",
		Params:      map[string]any{"order_id": "ORD-1234", "amount": 10.0, "reason": "test"},
	}

	// First call: executes the side effect.
	resp1, err := gw.Invoke(ctx, req)
	if err != nil {
		t.Fatalf("first invoke: %v", err)
	}
	if !resp1.Executed {
		t.Error("first invoke should have executed")
	}

	// Retry with the exact same params: must return the cached result,
	// not execute the side effect again.
	resp2, err := gw.Invoke(ctx, req)
	if err != nil {
		t.Fatalf("second invoke: %v", err)
	}
	if !resp2.Cached {
		t.Error("second invoke should have returned cached result")
	}

	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("executor called %d times, expected exactly 1", calls)
	}
}