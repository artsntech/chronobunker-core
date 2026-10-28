package security

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
)

// IdempotencyEngine implements the 3-phase RESERVE / EXECUTE / COMMIT pattern.
type IdempotencyEngine struct {
	db *sql.DB
}

// NewIdempotencyEngine constructs an IdempotencyEngine.
func NewIdempotencyEngine(db *sql.DB) *IdempotencyEngine {
	return &IdempotencyEngine{db: db}
}

// Execute runs the tool with exactly-once semantics.
func (e *IdempotencyEngine) Execute(
	ctx context.Context,
	key string,
	tok *CapabilityToken,
	req ToolRequest,
	execute Executor,
) (any, bool, error) {
	tenantID, err := db.TenantIDFromContext(ctx)
	if err != nil {
		return nil, false, err
	}

	paramsJSON, _ := json.Marshal(req.Params)

	var existingStatus string
	var existingResponse []byte
	err = db.WithTenantTx(ctx, e.db, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO idempotency_keys (key, tenant_id, session_id, tool_name, status, params)
			VALUES ($1, $2, $3, $4, 'reserved', $5)
			ON CONFLICT (key) DO NOTHING`,
			key, tenantID, tok.SessionID, req.ToolName, paramsJSON)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 1 {
			existingStatus = "reserved_new"
			return nil
		}
		return tx.QueryRowContext(ctx, `
			SELECT status, response FROM idempotency_keys WHERE key = $1`, key).
			Scan(&existingStatus, &existingResponse)
	})
	if err != nil {
		return nil, false, fmt.Errorf("idempotency: reserve: %w", err)
	}

	switch existingStatus {
	case "committed":
		var cached any
		if len(existingResponse) > 0 {
			_ = json.Unmarshal(existingResponse, &cached)
		}
		return cached, true, nil
	case "executed":
		return nil, false, errors.New("idempotency: previous execution is in-flight")
	case "reserved_new":
		// proceed
	case "failed":
		err = db.WithTenantTx(ctx, e.db, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
				UPDATE idempotency_keys SET status = 'reserved', updated_at = now() WHERE key = $1`, key)
			return err
		})
		if err != nil {
			return nil, false, err
		}
	default:
		return nil, false, fmt.Errorf("idempotency: unexpected status %q", existingStatus)
	}

	// Phase 2: EXECUTE
	result, execErr := execute(ctx, req.ToolName, req.Params)
	if execErr != nil {
		_ = db.WithTenantTx(ctx, e.db, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `
				UPDATE idempotency_keys SET status = 'failed', error = $1, updated_at = now()
				WHERE key = $2`, execErr.Error(), key)
			return err
		})
		return nil, false, execErr
	}

	// Phase 3: COMMIT
	resultJSON, _ := json.Marshal(result)
	err = db.WithTenantTx(ctx, e.db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE idempotency_keys
			SET status = 'committed', response = $1, updated_at = now()
			WHERE key = $2 AND status = 'reserved'`, resultJSON, key)
		return err
	})
	if err != nil {
		return nil, false, fmt.Errorf("idempotency: commit: %w", err)
	}

	return result, false, nil
}