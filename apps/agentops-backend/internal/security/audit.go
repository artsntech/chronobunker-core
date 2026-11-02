package security

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"time"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
)

// AuditLog records every tool decision in an append-only hash chain.
type AuditLog struct {
	db *sql.DB
}

// NewAuditLog constructs an AuditLog.
func NewAuditLog(db *sql.DB) *AuditLog {
	return &AuditLog{db: db}
}

// Record appends an entry to the audit trail.
func (a *AuditLog) Record(ctx context.Context, tok *CapabilityToken, req ToolRequest, result any, decision, errMsg string) {
	tenantID, err := db.TenantIDFromContext(ctx)
	if err != nil {
		// No tenant context means the request was rejected before reaching
		// the gateway. Log to stderr and return; there is no tenant to
		// attribute the audit entry to.
		return
	}

	paramsJSON, _ := json.Marshal(req.Params)
	resultJSON, _ := json.Marshal(result)

	var prevHash string
	_ = db.WithTenantTx(ctx, a.db, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
			SELECT row_hash FROM audit_entries ORDER BY id DESC LIMIT 1`).Scan(&prevHash)
		if err == sql.ErrNoRows {
			prevHash = "0000000000000000000000000000000000000000000000000000000000000000"
			return nil
		}
		return err
	})

	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write([]byte(tenantID))
	h.Write([]byte(tok.SessionID))
	h.Write([]byte(req.ToolName))
	h.Write(paramsJSON)
	h.Write(resultJSON)
	h.Write([]byte(decision))
	if errMsg != "" {
		h.Write([]byte(errMsg))
	}
	rowHash := hex.EncodeToString(h.Sum(nil))

	_ = db.WithTenantTx(ctx, a.db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO audit_entries (tenant_id, session_id, tool_name, params, result, decision, prev_hash, row_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			tenantID, tok.SessionID, req.ToolName, paramsJSON, resultJSON, decision, prevHash, rowHash)
		return err
	})
}

// Reject records a blocked tool invocation in the immutable audit trail.
// The token was not verified, so the tenant is taken from the context (the
// JWT middleware injects it before the Gateway is reached). If there is no
// tenant context, the event cannot be attributed and is dropped.
func (a *AuditLog) Reject(ctx context.Context, req ToolRequest, reason string) {
	tenantID, err := db.TenantIDFromContext(ctx)
	if err != nil {
		// No tenant: cannot attribute the rejection. Log to stderr and return.
		log.Printf("security: rejected %q without tenant context: %s", req.ToolName, reason)
		return
	}

	// Build a minimal token-like structure for the hash chain.
	// The session ID is unknown at rejection time, so use a synthetic marker.
	sessionID := "rejected-" + time.Now().Format("20060102150405.000")

	paramsJSON, _ := json.Marshal(req.Params)
	resultJSON, _ := json.Marshal(map[string]string{
		"error":  reason,
		"status": "blocked",
	})

	var prevHash string
	_ = db.WithTenantTx(ctx, a.db, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
			SELECT row_hash FROM audit_entries ORDER BY id DESC LIMIT 1`).Scan(&prevHash)
		if err == sql.ErrNoRows {
			prevHash = "0000000000000000000000000000000000000000000000000000000000000000"
			return nil
		}
		return err
	})

	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write([]byte(tenantID))
	h.Write([]byte(sessionID))
	h.Write([]byte(req.ToolName))
	h.Write(paramsJSON)
	h.Write(resultJSON)
	h.Write([]byte("blocked"))
	rowHash := hex.EncodeToString(h.Sum(nil))

	_ = db.WithTenantTx(ctx, a.db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO audit_entries (tenant_id, session_id, tool_name, params, result, decision, prev_hash, row_hash)
			VALUES ($1, $2, $3, $4, $5, 'blocked', $6, $7)`,
			tenantID, sessionID, req.ToolName, paramsJSON, resultJSON, prevHash, rowHash)
		return err
	})
}

// RecordAction records a non-tool action (e.g., order creation) in the
// audit trail. It is used when the action does not go through the Tool
// Gateway and therefore has no capability token.
func (a *AuditLog) RecordAction(ctx context.Context, sessionID, action string, params any, result any, decision string) error {
	tenantID, err := db.TenantIDFromContext(ctx)
	if err != nil {
		return err
	}

	paramsJSON, _ := json.Marshal(params)
	resultJSON, _ := json.Marshal(result)

	var prevHash string
	_ = db.WithTenantTx(ctx, a.db, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `
			SELECT row_hash FROM audit_entries ORDER BY id DESC LIMIT 1`).Scan(&prevHash)
		if err == sql.ErrNoRows {
			prevHash = "0000000000000000000000000000000000000000000000000000000000000000"
			return nil
		}
		return err
	})

	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write([]byte(tenantID))
	h.Write([]byte(sessionID))
	h.Write([]byte(action))
	h.Write(paramsJSON)
	h.Write(resultJSON)
	h.Write([]byte(decision))
	rowHash := hex.EncodeToString(h.Sum(nil))

	return db.WithTenantTx(ctx, a.db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO audit_entries (tenant_id, session_id, tool_name, params, result, decision, prev_hash, row_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			tenantID, sessionID, action, paramsJSON, resultJSON, decision, prevHash, rowHash)
		return err
	})
}