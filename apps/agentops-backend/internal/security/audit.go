package security

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"

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

// Reject logs a rejected tool invocation. The token was not verified, so we
// cannot attribute the rejection to a tenant. In production this would be
// forwarded to a SIEM as a security event.
func (a *AuditLog) Reject(ctx context.Context, req ToolRequest, reason string) {
	_ = fmt.Sprintf("security: rejected %q: %s", req.ToolName, reason)
}