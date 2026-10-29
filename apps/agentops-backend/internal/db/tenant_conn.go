// Package db provides database access primitives with tenant isolation
// enforced at the connection level via PostgreSQL Row-Level Security.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// TenantID is the type used to store the current tenant identifier in
// the request context. Using a private type prevents accidental collisions
// with other context keys.
type tenantContextKey struct{}

// WithTenantID returns a new context carrying the given tenant identifier.
// It is called by the HTTP middleware after authenticating the request.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenantID)
}

// TenantIDFromContext returns the tenant identifier stored in the context.
// It returns an error if the context does not carry a tenant, which would
// indicate a programming error (a handler reached the database without
// passing through the tenant middleware).
func TenantIDFromContext(ctx context.Context) (string, error) {
	tenantID, ok := ctx.Value(tenantContextKey{}).(string)
	if !ok || tenantID == "" {
		return "", errors.New("db: no tenant id in context")
	}
	return tenantID, nil
}

// Querier is the subset of *sql.DB and *sql.Tx that the repository layer
// needs. It allows the same repository code to run inside or outside a
// transaction without changes.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// WithTenantTx opens a transaction, sets the tenant context for the
// duration of the transaction, and passes the transaction to fn. If fn
// returns an error, the transaction is rolled back. Otherwise it is
// committed.
//
// The SET LOCAL statement is scoped to the transaction, so it cannot leak
// to other requests that reuse the same connection from the pool.
func WithTenantTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tenantID, err := TenantIDFromContext(ctx)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Set the tenant context for RLS.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.tenant_id = '%s'", tenantID)); err != nil {
		return fmt.Errorf("db: set tenant context: %w", err)
	}

	// Per-transaction statement timeout. This bounds the wall-clock time of
	// any single query inside the transaction. A query that exceeds this
	// limit is aborted by Postgres, freeing the connection.
	if _, err := tx.ExecContext(ctx, "SET LOCAL statement_timeout = '10s'"); err != nil {
		return fmt.Errorf("db: set statement timeout: %w", err)
	}

	// Per-transaction lock timeout. A query that waits for a lock longer
	// than this is aborted.
	if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout = '5s'"); err != nil {
		return fmt.Errorf("db: set lock timeout: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}