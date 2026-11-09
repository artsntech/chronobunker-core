package agent_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/agent"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
)

// testDB opens a connection to the integration test database. The test
// is skipped if CHRONO_TEST_DB_URL is not set, so that `go test ./...`
// works without a database in CI for the unit-only jobs.
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

const (
	tenantAcme   = "11111111-1111-1111-1111-111111111111"
	tenantGlobex = "22222222-2222-2222-2222-222222222222"
)

// TestTenantIsolation is the most important test in the project. It proves
// that a query issued in the context of tenant A cannot see agents that
// belong to tenant B, even though both rows exist in the same table.
func TestTenantIsolation(t *testing.T) {
	database := testDB(t)
	repo := agent.NewRepository(database)

	// Tenant Acme lists its own agents.
	acmeCtx := db.WithTenantID(context.Background(), tenantAcme)
	acmeAgents, err := repo.List(acmeCtx)
	if err != nil {
		t.Fatalf("acme list: %v", err)
	}
	if len(acmeAgents) == 0 {
		t.Fatal("expected acme to have at least one agent; run db-seed first")
	}
	for _, a := range acmeAgents {
		if a.TenantID != tenantAcme {
			t.Errorf("acme context leaked an agent from tenant %s", a.TenantID)
		}
	}

	// Tenant Globex lists its own agents.
	globexCtx := db.WithTenantID(context.Background(), tenantGlobex)
	globexAgents, err := repo.List(globexCtx)
	if err != nil {
		t.Fatalf("globex list: %v", err)
	}
	if len(globexAgents) == 0 {
		t.Fatal("expected globex to have at least one agent; run db-seed first")
	}
	for _, a := range globexAgents {
		if a.TenantID != tenantGlobex {
			t.Errorf("globex context leaked an agent from tenant %s", a.TenantID)
		}
	}

	// Cross-tenant access attempt: Acme tries to fetch a Globex agent by
	// ID. It must fail with ErrNotFound, not return the row.
	globexAgentID := globexAgents[0].ID
	_, err = repo.GetByID(acmeCtx, globexAgentID)
	if err == nil {
		t.Fatal("SECURITY: acme context was able to read a globex agent; RLS is not enforced")
	}
	if !errors.Is(err, agent.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Missing tenant context must fail loudly.
	_, err = repo.List(context.Background())
	if err == nil {
		t.Fatal("list without tenant context should have failed")
	}
}
