package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
)

type Status string

const (
	StatusDraft      Status = "draft"
	StatusCanary     Status = "canary"
	StatusProduction Status = "production"
	StatusKilled     Status = "killed"
)

type Agent struct {
	ID           string
	TenantID     string
	Name         string
	Department   string
	Status       Status
	HealthScore  float64
	AllowedTools []string
	TargetModel  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

var ErrNotFound = errors.New("agent: not found")

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context) ([]*Agent, error) {
	var agents []*Agent
	err := db.WithTenantTx(ctx, r.db, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, tenant_id, name, department, status, health_score,
			       allowed_tools, target_model, created_at, updated_at
			FROM agents
			ORDER BY created_at DESC`)
		if err != nil {
			return fmt.Errorf("list agents: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			a := &Agent{}
			if err := rows.Scan(
				&a.ID, &a.TenantID, &a.Name, &a.Department,
				&a.Status, &a.HealthScore,
				pq.Array(&a.AllowedTools), &a.TargetModel,
				&a.CreatedAt, &a.UpdatedAt,
			); err != nil {
				return fmt.Errorf("scan agent: %w", err)
			}
			agents = append(agents, a)
		}
		return rows.Err()
	})
	return agents, err
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Agent, error) {
	var a *Agent
	err := db.WithTenantTx(ctx, r.db, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT id, tenant_id, name, department, status, health_score,
			       allowed_tools, target_model, created_at, updated_at
			FROM agents
			WHERE id = $1`, id)

		a = &Agent{}
		err := row.Scan(
			&a.ID, &a.TenantID, &a.Name, &a.Department,
			&a.Status, &a.HealthScore,
			pq.Array(&a.AllowedTools), &a.TargetModel,
			&a.CreatedAt, &a.UpdatedAt,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *Repository) Create(ctx context.Context, name, department string) (*Agent, error) {
	tenantID, err := db.TenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var a *Agent
	err = db.WithTenantTx(ctx, r.db, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			INSERT INTO agents (tenant_id, name, department, status)
			VALUES ($1, $2, $3, 'draft')
			RETURNING id, tenant_id, name, department, status, health_score,
			          allowed_tools, target_model, created_at, updated_at`,
			tenantID, name, department)

		a = &Agent{}
		return row.Scan(
			&a.ID, &a.TenantID, &a.Name, &a.Department,
			&a.Status, &a.HealthScore,
			pq.Array(&a.AllowedTools), &a.TargetModel,
			&a.CreatedAt, &a.UpdatedAt,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}
	return a, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id string, status Status) error {
	return db.WithTenantTx(ctx, r.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE agents SET status = $1, updated_at = now()
			WHERE id = $2`, status, id)
		if err != nil {
			return fmt.Errorf("update agent status: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}
