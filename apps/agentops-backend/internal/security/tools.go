package security

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
)

// ToolDef is the definition of a tool loaded from the tools table.
type ToolDef struct {
	Name             string
	Description      string
	InputSchema      []byte
	RiskLevel        string
	RequiresApproval bool
}

// loadToolDef reads a tool definition from the tools table.
// The tools table is global (no RLS), but we use WithTenantTx for consistency
// with the rest of the gateway and to avoid leaking connection state.
func (g *Gateway) loadToolDef(ctx context.Context, name string) (*ToolDef, error) {
	var def ToolDef
	err := db.WithTenantTx(ctx, g.db, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT name, description, input_schema, risk_level, requires_approval
			FROM tools WHERE name = $1`, name).
			Scan(&def.Name, &def.Description, &def.InputSchema, &def.RiskLevel, &def.RequiresApproval)
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tool %q not found", name)
		}
		return nil, err
	}
	return &def, nil
}

// Avoid unused import warnings if json is not referenced elsewhere.
var _ = json.Marshal