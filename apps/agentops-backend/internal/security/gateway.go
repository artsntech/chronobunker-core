package security

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
)

// ToolRequest is a request from an agent to invoke a tool.
type ToolRequest struct {
	SignedToken string
	DeviceID    string  // NEW: must match the token's DeviceID claim
	ToolName    string
	Params      map[string]any
}

// ToolResponse is the result of a tool invocation.
type ToolResponse struct {
	Result   any
	Cached   bool
	Executed bool
}

// Executor performs the external side effect for a tool invocation.
type Executor func(ctx context.Context, toolName string, params map[string]any) (any, error)

// Gateway validates and executes tool invocations with exactly-once semantics.
type Gateway struct {
	db      *sql.DB
	tokens  *TokenService
	execute Executor
	idm     *IdempotencyEngine
	audit   *AuditLog
}

// NewGateway constructs a Gateway.
func NewGateway(db *sql.DB, tokens *TokenService, execute Executor) *Gateway {
	return &Gateway{
		db:      db,
		tokens:  tokens,
		execute: execute,
		idm:     NewIdempotencyEngine(db),
		audit:   NewAuditLog(db),
	}
}

// Invoke validates the capability token, checks the tool schema, enforces
// resource binding, and executes the tool with exactly-once semantics.
func (g *Gateway) Invoke(ctx context.Context, req ToolRequest) (*ToolResponse, error) {
	// ── Step 1: Verify the cryptographic capability token.
	tok, err := g.tokens.Verify(req.SignedToken)
	if err != nil {
		g.audit.Reject(ctx, req, "invalid_token")
		return nil, fmt.Errorf("gateway: %w", err)
	}

	ctxTenant, err := db.TenantIDFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("gateway: no tenant context: %w", err)
	}
	if tok.TenantID != ctxTenant {
		g.audit.Reject(ctx, req, "tenant_mismatch")
		return nil, errors.New("gateway: token tenant does not match request tenant")
	}
	// After verifying the token:
	if tok.DeviceID != "" && req.DeviceID != tok.DeviceID {
		g.audit.Reject(ctx, req, "device_mismatch")
		return nil, errors.New("gateway: device ID does not match session")
	}

	// ── Step 2: Tool must be in the token's allowed list.
	if !slices.Contains(tok.AllowedTools, req.ToolName) {
		g.audit.Reject(ctx, req, "tool_not_allowed")
		return nil, fmt.Errorf("gateway: tool %q not allowed in this session", req.ToolName)
	}

	// ── Step 3: Validate input against the tool's JSON Schema.
	toolDef, err := g.loadToolDef(ctx, req.ToolName)
	if err != nil {
		return nil, fmt.Errorf("gateway: load tool: %w", err)
	}
	if err := validateSchema(toolDef.InputSchema, req.Params); err != nil {
		g.audit.Reject(ctx, req, "schema_violation")
		return nil, fmt.Errorf("gateway: schema violation: %w", err)
	}

	// ── Step 4: Resource binding.
	if err := checkResourceBinding(tok, req.Params); err != nil {
		g.audit.Reject(ctx, req, "resource_binding_violation")
		return nil, err
	}

	// ── Step 5: Amount check.
	if amount, ok := req.Params["amount"].(float64); ok && tok.MaxAmount > 0 {
		if amount > tok.MaxAmount {
			g.audit.Reject(ctx, req, "amount_exceeds_limit")
			return nil, fmt.Errorf("gateway: amount %.2f exceeds session limit %.2f", amount, tok.MaxAmount)
		}
	}

	// ── Step 6: Execute with exactly-once semantics.
	key := buildIdempotencyKey(tok.SessionID, req.ToolName, req.Params)
	result, cached, err := g.idm.Execute(ctx, key, tok, req, g.execute)
	if err != nil {
		g.audit.Record(ctx, tok, req, nil, "error", err.Error())
		return nil, err
	}

	g.audit.Record(ctx, tok, req, result, "allowed", "")
	return &ToolResponse{Result: result, Cached: cached, Executed: !cached}, nil
}

// buildIdempotencyKey computes a deterministic key from the session ID,
// tool name, and canonicalized parameters.
func buildIdempotencyKey(sessionID, toolName string, params map[string]any) string {
	canonical, _ := json.Marshal(params)
	h := sha256.New()
	h.Write([]byte(sessionID))
	h.Write([]byte(toolName))
	h.Write(canonical)
	return hex.EncodeToString(h.Sum(nil))
}