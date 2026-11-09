// internal/db/tenant_conn_test.go
package db_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
)

func TestStatementTimeoutAbortsSlowQuery(t *testing.T) {
	url := os.Getenv("CHRONO_TEST_DB_URL")
	if url == "" {
		t.Skip("CHRONO_TEST_DB_URL not set")
	}
	conn, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	ctx := db.WithTenantID(context.Background(), "11111111-1111-1111-1111-111111111111")
	err = db.WithTenantTx(ctx, conn, func(tx *sql.Tx) error {
		// pg_sleep(15) exceeds the 10s statement_timeout.
		_, err := tx.ExecContext(ctx, "SELECT pg_sleep(15)")
		return err
	})
	if err == nil {
		t.Fatal("expected statement_timeout to abort the query")
	}
	if !contains(err.Error(), "statement timeout") && !contains(err.Error(), "canceling statement") {
		t.Fatalf("expected statement timeout error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
