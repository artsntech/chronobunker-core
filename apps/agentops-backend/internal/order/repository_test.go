package order_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/order"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/security"
)

const (
	tenantAcme   = "11111111-1111-1111-1111-111111111111"
	tenantGlobex = "22222222-2222-2222-2222-222222222222"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("CHRONO_TEST_DB_URL")
	if url == "" {
		t.Skip("CHRONO_TEST_DB_URL not set; skipping integration test")
	}
	conn, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// TestOrderRLS proves that an order created by one tenant is not visible
// to another tenant, even when the ID is known.
func TestOrderRLS(t *testing.T) {
	database := testDB(t)
	auditLog := security.NewAuditLog(database)
	repo := order.NewRepository(database, auditLog)

	acmeCtx := db.WithTenantID(context.Background(), tenantAcme)
	globexCtx := db.WithTenantID(context.Background(), tenantGlobex)

	acmeOrder, err := repo.Create(acmeCtx, "Acme Customer", "Bunker Pro Plan", 1, 299.00)
	if err != nil {
		t.Fatalf("acme create: %v", err)
	}

	found, err := repo.GetByID(acmeCtx, acmeOrder.ID)
	if err != nil {
		t.Fatalf("acme get own order: %v", err)
	}
	if found == nil || found.ID != acmeOrder.ID {
		t.Fatalf("acme could not read its own order")
	}

	notFound, err := repo.GetByID(globexCtx, acmeOrder.ID)
	if err != nil {
		t.Fatalf("globex get acme order: unexpected error: %v", err)
	}
	if notFound != nil {
		t.Fatalf("SECURITY: globex read acme order; expected nil, got %+v", notFound)
	}

	globexOrders, err := repo.List(globexCtx)
	if err != nil {
		t.Fatalf("globex list: %v", err)
	}
	for _, o := range globexOrders {
		if o.ID == acmeOrder.ID {
			t.Fatalf("SECURITY: globex list contains acme order")
		}
	}
}
