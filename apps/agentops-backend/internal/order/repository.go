// Package order provides the repository for Bunker Store orders.
package order

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
)

// Order is a customer purchase from the Bunker Store.
type Order struct {
	ID           string
	TenantID     string
	CustomerName string
	Product      string
	Quantity     int
	TotalUSD     float64
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Repository provides CRUD operations for orders. All methods require a
// context carrying a tenant ID and must be called inside db.WithTenantTx.
type Repository struct {
	db *sql.DB
}

// NewRepository constructs a Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// List returns all orders visible to the current tenant.
func (r *Repository) List(ctx context.Context) ([]*Order, error) {
	var orders []*Order
	err := db.WithTenantTx(ctx, r.db, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, tenant_id, customer_name, product, quantity, total_usd, status, created_at, updated_at
			FROM orders
			ORDER BY created_at DESC`)
		if err != nil {
			return fmt.Errorf("list orders: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			o := &Order{}
			if err := rows.Scan(&o.ID, &o.TenantID, &o.CustomerName, &o.Product,
				&o.Quantity, &o.TotalUSD, &o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
				return fmt.Errorf("scan order: %w", err)
			}
			orders = append(orders, o)
		}
		return rows.Err()
	})
	return orders, err
}

// GetByID returns a single order, or nil if not found or not visible.
func (r *Repository) GetByID(ctx context.Context, id string) (*Order, error) {
	var o *Order
	err := db.WithTenantTx(ctx, r.db, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT id, tenant_id, customer_name, product, quantity, total_usd, status, created_at, updated_at
			FROM orders WHERE id = $1`, id)
		o = &Order{}
		err := row.Scan(&o.ID, &o.TenantID, &o.CustomerName, &o.Product,
			&o.Quantity, &o.TotalUSD, &o.Status, &o.CreatedAt, &o.UpdatedAt)
		if err == sql.ErrNoRows {
			o = nil
			return nil
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return o, nil
}