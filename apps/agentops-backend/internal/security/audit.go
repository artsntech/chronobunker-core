package security

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
)

const genesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// AuditLog records every tool decision in an append-only hash chain.
type AuditLog struct {
	db *sql.DB
}

// NewAuditLog constructs an AuditLog.
func NewAuditLog(db *sql.DB) *AuditLog {
	return &AuditLog{db: db}
}

// appendEntry is the single writer for the audit trail. It acquires a
// per-tenant advisory lock, reads the previous row_hash, computes the new
// row_hash, and inserts the new row—all within one transaction. This
// guarantees that concurrent writers for the same tenant serialize and the
// hash chain never forks.
func (a *AuditLog) appendEntry(
	ctx context.Context,
	tenantID string,
	sessionID string,
	toolName string,
	params any,
	result any,
	decision string,
	errMsg string,
) error {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("audit: marshal params: %w", err)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("audit: marshal result: %w", err)
	}

	return db.WithTenantTx(ctx, a.db, func(tx *sql.Tx) error {
		// 1. Acquire a per-tenant advisory lock for the duration of the
		// transaction. Concurrent writers for the same tenant block here
		// until the previous transaction commits or rolls back.
		// hashtext() maps the tenant UUID to a stable int4 lock key.
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, tenantID); err != nil {
			return fmt.Errorf("audit: advisory lock: %w", err)
		}

		// 2. Read the previous row_hash for THIS tenant only. The ORDER BY
		// id DESC and the tenant filter are both required: without the
		// filter, a misconfigured RLS context could leak another tenant's
		// hash; without the order, we would chain to the wrong row.
		var prevHash string
		err := tx.QueryRowContext(ctx, `
			SELECT row_hash FROM audit_entries
			WHERE tenant_id = $1
			ORDER BY id DESC
			LIMIT 1`, tenantID).Scan(&prevHash)
		if err == sql.ErrNoRows {
			prevHash = genesisHash
		} else if err != nil {
			return fmt.Errorf("audit: read prev_hash: %w", err)
		}

		// 3. Compute the new row_hash over (prev_hash || tenant || session ||
		// tool || params || result || decision || errMsg).
		h := sha256.New()
		h.Write([]byte(prevHash))
		h.Write([]byte(tenantID))
		h.Write([]byte(sessionID))
		h.Write([]byte(toolName))
		h.Write(paramsJSON)
		h.Write(resultJSON)
		h.Write([]byte(decision))
		if errMsg != "" {
			h.Write([]byte(errMsg))
		}
		rowHash := hex.EncodeToString(h.Sum(nil))

		// 4. Insert the new row within the same transaction. The advisory
		// lock is released automatically on COMMIT/ROLLBACK.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO audit_entries (tenant_id, session_id, tool_name, params, result, decision, prev_hash, row_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			tenantID, sessionID, toolName, paramsJSON, resultJSON, decision, prevHash, rowHash,
		); err != nil {
			return fmt.Errorf("audit: insert: %w", err)
		}
		return nil
	})
}

// Record appends a successful tool invocation to the audit trail.
func (a *AuditLog) Record(ctx context.Context, tok *CapabilityToken, req ToolRequest, result any, decision, errMsg string) {
	tenantID, err := db.TenantIDFromContext(ctx)
	if err != nil {
		// No tenant context means the request was rejected before reaching
		// the gateway. Nothing to attribute the entry to.
		return
	}

	if err := a.appendEntry(ctx, tenantID, tok.SessionID, req.ToolName, req.Params, result, decision, errMsg); err != nil {
		log.Printf("audit: record failed: %v", err)
	}
}

// Reject records a blocked tool invocation in the immutable audit trail.
// The token was not verified, so the tenant comes from the context. If
// there is no tenant context, the event cannot be attributed and is dropped.
func (a *AuditLog) Reject(ctx context.Context, req ToolRequest, reason string) {
	tenantID, err := db.TenantIDFromContext(ctx)
	if err != nil {
		log.Printf("security: rejected %q without tenant context: %s", req.ToolName, reason)
		return
	}

	sessionID := "rejected-" + time.Now().Format("20060102150405.000")
	result := map[string]string{
		"error":  reason,
		"status": "blocked",
	}

	if err := a.appendEntry(ctx, tenantID, sessionID, req.ToolName, req.Params, result, "blocked", reason); err != nil {
		log.Printf("audit: reject failed: %v", err)
	}
}

// RecordAction records a non-tool action (e.g., order creation) in the
// audit trail. It is used when the action does not go through the Tool
// Gateway and therefore has no capability token.
func (a *AuditLog) RecordAction(ctx context.Context, sessionID, action string, params any, result any, decision string) error {
	tenantID, err := db.TenantIDFromContext(ctx)
	if err != nil {
		return err
	}
	return a.appendEntry(ctx, tenantID, sessionID, action, params, result, decision, "")
}