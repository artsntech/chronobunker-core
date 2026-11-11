package graph

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/agent"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/breaker"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/order"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/redteam"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/security"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/graph/model"
)

// ── Query resolvers ──────────────────────────────────────────────

func (r *queryResolver) Agents(ctx context.Context) ([]*agent.Agent, error) {
	return r.AgentRepo.List(ctx)
}

func (r *queryResolver) Agent(ctx context.Context, id string) (*agent.Agent, error) {
	a, err := r.AgentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, agent.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get agent: %w", err)
	}
	return a, nil
}

func (r *queryResolver) Costs(ctx context.Context, from time.Time, to time.Time) (*model.CostSummary, error) {
	tenantID, err := db.TenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var totalUSD float64
	var eventCount int
	err = db.WithTenantTx(ctx, r.DB, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(cost_usd), 0), COUNT(*)
			FROM cost_events
			WHERE created_at >= $1 AND created_at <= $2`,
			from, to).Scan(&totalUSD, &eventCount)
	})
	if err != nil {
		return nil, fmt.Errorf("aggregate costs: %w", err)
	}

	return &model.CostSummary{
		TenantID:   tenantID,
		TotalUsd:   totalUSD,
		EventCount: eventCount,
		From:       from,
		To:         to,
	}, nil
}

// ── Mutation resolvers ───────────────────────────────────────────

func (r *mutationResolver) CreateAgent(ctx context.Context, input model.CreateAgentInput) (*agent.Agent, error) {
	return r.AgentRepo.Create(ctx, input.Name, input.Department)
}

func (r *mutationResolver) UpdateAgentStatus(ctx context.Context, input model.UpdateAgentStatusInput) (*agent.Agent, error) {
	status := agent.Status(input.Status)
	if err := r.AgentRepo.UpdateStatus(ctx, input.ID, status); err != nil {
		return nil, err
	}
	return r.AgentRepo.GetByID(ctx, input.ID)
}

func (r *mutationResolver) InvokeAgent(ctx context.Context, input model.InvokeAgentInput) (*model.InvokeAgentResult, error) {
	agentRecord, err := r.AgentRepo.GetByID(ctx, input.AgentID)
	if err != nil {
		return nil, fmt.Errorf("load agent: %w", err)
	}

	// Domain validation: the agent only handles its declared scope.
	// This is the second layer of defense (the frontend is the first).
	if r.Classifier != nil {
		domain := domainForDepartment(agentRecord.Department)
		result, err := r.Classifier.Classify(ctx, input.Prompt, domain)
		if err != nil {
			// Fail closed: if the classifier is down, reject rather than
			// letting an unvalidated prompt through.
			return nil, fmt.Errorf("intent classification unavailable: %w", err)
		}
		if !result.Allowed {
			return nil, fmt.Errorf("out of scope: %s", result.Reason)
		}
	}

	taskType := "triage"
	if agentRecord.Department == "chaos" {
		taskType = "chaos"
	} else if agentRecord.Department != "support" {
		taskType = "reasoning"
	}

	start := time.Now()
	completion, modelID, costUSD, err := r.Router.Route(ctx, taskType, agentRecord.TargetModel, input.Prompt, input.SessionID)
	if err != nil {
		return nil, fmt.Errorf("route: %w", err)
	}
	latency := time.Since(start)

	return &model.InvokeAgentResult{
		Completion: completion.Text,
		ModelID:    modelID,
		CostUsd:    costUSD,
		LatencyMs:  int(latency.Milliseconds()),
	}, nil
}

func (r *mutationResolver) InvokeTool(ctx context.Context, input model.InvokeToolInput) (*model.InvokeToolResult, error) {
	agentRecord, err := r.AgentRepo.GetByID(ctx, input.AgentID)
	if err != nil {
		return nil, fmt.Errorf("load agent: %w", err)
	}

	signedToken, err := r.TokenService.Issue(
		input.SessionID,
		agentRecord.TenantID,
		"user-from-jwt",
		input.DeviceID,
		agentRecord.AllowedTools,
		nil,
		50.0,
	)
	if err != nil {
		return nil, fmt.Errorf("issue token: %w", err)
	}

	toolReq := security.ToolRequest{
		SignedToken: signedToken,
		DeviceID:    input.DeviceID,
		ToolName:    input.ToolName,
		Params:      input.Params,
	}
	// Optional cryptographic device binding: the browser signs the canonical
	// payload with its registered Ed25519 private key. If the browser does not
	// support Web Crypto Ed25519, these fields are nil and the Gateway falls
	// back to DeviceID-only validation (documented in limitations.md).
	if input.Signature != nil {
		toolReq.Signature = *input.Signature
	}
	if input.Timestamp != nil {
		toolReq.Timestamp = int64(*input.Timestamp)
	}

	toolResult, err := r.Gateway.Invoke(ctx, toolReq)
	if err != nil {
		return nil, err
	}

	resultMap, ok := toolResult.Result.(map[string]any)
	if !ok {
		resultMap = map[string]any{"value": toolResult.Result}
	}

	return &model.InvokeToolResult{
		Result:   resultMap,
		Cached:   toolResult.Cached,
		Executed: toolResult.Executed,
	}, nil
}

func (r *mutationResolver) Login(ctx context.Context, tenantID string) (*model.LoginResult, error) {
	deviceID := fmt.Sprintf("web-device-%d", time.Now().UnixNano())

	token, err := r.JWTIssuer.Issue(tenantID)
	if err != nil {
		return nil, fmt.Errorf("issue jwt: %w", err)
	}

	return &model.LoginResult{
		Token:    token,
		TenantID: tenantID,
		DeviceID: deviceID,
	}, nil
}

// ── Agent field resolvers ────────────────────────────────────────

func (r *agentResolver) BreakerState(ctx context.Context, obj *agent.Agent) (string, error) {
	state, err := r.Breaker.State(ctx, breaker.TargetModel, obj.TargetModel)
	if err != nil {
		return "CLOSED", nil // fail-safe: assume closed on Redis error
	}
	return string(state), nil
}

func (r *queryResolver) AuditEntries(ctx context.Context, limit int) ([]*model.AuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var entries []*model.AuditEntry
	err := db.WithTenantTx(ctx, r.DB, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
    SELECT id, tenant_id, session_id, tool_name, decision,
           prev_hash, row_hash, created_at
    FROM audit_entries
    ORDER BY id DESC
    LIMIT $1`, limit)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			e := &model.AuditEntry{}
			var toolName sql.NullString
			var createdAt time.Time
			if err := rows.Scan(&e.ID, &e.TenantID, &e.SessionID, &toolName,
				&e.Decision, &e.PrevHash, &e.RowHash, &createdAt); err != nil {
				return err
			}
			if toolName.Valid {
				e.ToolName = &toolName.String
			}
			e.CreatedAt = createdAt
			entries = append(entries, e)
		}
		return rows.Err()
	})
	return entries, err
}

func (r *subscriptionResolver) AuditEntryAdded(ctx context.Context) (<-chan *model.AuditEntry, error) {
	tenantID, err := db.TenantIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	hubCh := r.AuditHub.Subscribe(tenantID)
	out := make(chan *model.AuditEntry, 16)

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				r.AuditHub.Unsubscribe(hubCh)
				return
			case entry, ok := <-hubCh:
				if !ok {
					return
				}
				var toolName *string
				if entry.ToolName != "" {
					toolName = &entry.ToolName
				}
				select {
				case out <- &model.AuditEntry{
					ID:        entry.ID,
					TenantID:  entry.TenantID,
					SessionID: entry.SessionID,
					ToolName:  toolName,
					Decision:  entry.Decision,
					PrevHash:  entry.PrevHash,
					RowHash:   entry.RowHash,
					CreatedAt: entry.CreatedAt,
				}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, nil
}

func domainForDepartment(dept string) string {
	switch dept {
	case "support":
		return `Order tracking, proof approval, file format questions (CMYK, DPI, bleed), refund requests, delivery estimates, product specifications.`
	case "chaos":
		return `Chaos testing and failure injection for infrastructure resilience.`
	default:
		return `General order-related customer support.`
	}
}

func (r *subscriptionResolver) RedTeamStream(ctx context.Context) (<-chan *model.RedTeamResult, error) {
	ch := make(chan *redteam.ScenarioResult, 16)
	r.RedTeamHarness.Subscribe(ch)
	out := make(chan *model.RedTeamResult, 16)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				r.RedTeamHarness.Unsubscribe(ch)
				return
			case r, ok := <-ch:
				if !ok {
					return
				}
				out <- &model.RedTeamResult{
					ID:            r.ID,
					Name:          r.Name,
					OwaspCategory: r.OWASPCategory,
					Status:        r.Status,
					Detail:        r.Detail,
					DurationMs:    r.DurationMs,
					RanAt:         r.RanAt,
				}
			}
		}
	}()
	return out, nil
}

func (r *mutationResolver) RunRedTeam(ctx context.Context) (bool, error) {
	go r.RedTeamHarness.RunAll(context.Background())
	return true, nil
}

func (r *queryResolver) Orders(ctx context.Context) ([]*order.Order, error) {
	return r.OrderRepo.List(ctx)
}

func (r *queryResolver) Order(ctx context.Context, id string) (*order.Order, error) {
	return r.OrderRepo.GetByID(ctx, id)
}

func (r *mutationResolver) CreateOrder(ctx context.Context, input model.CreateOrderInput) (*order.Order, error) {
	// Product catalog with prices. In a real product, this would come from
	// a database table, not a hardcoded map.
	prices := map[string]float64{
		"Bunker Starter Plan":    99.00,
		"Bunker Pro Plan":        299.00,
		"Bunker Enterprise Plan": 1299.00,
		"Chrono Agent License":   149.00,
	}
	price, ok := prices[input.Product]
	if !ok {
		return nil, fmt.Errorf("unknown product: %s", input.Product)
	}
	if input.Quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	total := price * float64(input.Quantity)

	// Customer name derived from tenant. In production, it would be a JWT claim.
	customerName := "Customer"
	if tenantID, err := db.TenantIDFromContext(ctx); err == nil {
		switch tenantID {
		case "11111111-1111-1111-1111-111111111111":
			customerName = "Acme Customer"
		case "22222222-2222-2222-2222-222222222222":
			customerName = "Globex Customer"
		}
	}

	return r.OrderRepo.Create(ctx, customerName, input.Product, input.Quantity, total)
}

func (r *mutationResolver) LaunchMission(ctx context.Context, input model.LaunchMissionInput) (*model.MissionResult, error) {
	agentRecord, err := r.AgentRepo.GetByID(ctx, "5ca2ccb5-061f-4042-9222-9c076776a2e3")
	if err != nil {
		return nil, fmt.Errorf("load agent: %w", err)
	}

	missionID := fmt.Sprintf("mission-%d", time.Now().Unix())
	signedToken, err := r.TokenService.Issue(
		missionID,
		agentRecord.TenantID,
		"mission-runner",
		"mission-device",
		input.AllowedTools,
		[]string{"ORD-1234"}, // the order the mission can touch
		input.BudgetUsd,
	)
	if err != nil {
		return nil, fmt.Errorf("issue token: %w", err)
	}

	// Canonical params for each tool, matching the JSON Schema in the tools table.
	toolParams := map[string]map[string]any{
		"send_email": {
			"to":        "customer@example.com",
			"template":  "abandoned_cart",
			"variables": map[string]any{"discount": "10%"},
		},
		"get_order": {
			"order_id": "ORD-1234",
		},
		"search_help_center": {
			"query": "abandoned cart recovery",
		},
		"issue_refund": {
			"order_id": "ORD-1234",
			"amount":   100.0,
			"reason":   "mission demo",
		},
	}

	deviceID := "mission-device"
	executed, blocked := 0, 0

	for _, tool := range input.AllowedTools {
		params, ok := toolParams[tool]
		if !ok {
			log.Printf("mission: no canonical params for tool %q, skipping", tool)
			blocked++
			continue
		}

		log.Printf("mission: trying tool %q with params %+v", tool, params)
		_, err := r.Gateway.Invoke(ctx, security.ToolRequest{
			SignedToken: signedToken,
			DeviceID:    deviceID,
			ToolName:    tool,
			Params:      params,
		})
		if err != nil {
			log.Printf("mission: tool %q blocked: %v", tool, err)
			blocked++
			continue
		}
		executed++
	}

	return &model.MissionResult{
		MissionID:       missionID,
		Status:          "completed",
		ActionsExecuted: executed,
		ActionsBlocked:  blocked,
		TotalSpentUsd:   input.BudgetUsd,
	}, nil
}

func (r *mutationResolver) RegisterDeviceKey(ctx context.Context, input model.RegisterDeviceKeyInput) (bool, error) {
	if r.Devices == nil {
		return false, fmt.Errorf("device registry not configured")
	}
	if err := r.Devices.Register(ctx, input.DeviceID, input.PublicKey); err != nil {
		return false, fmt.Errorf("register device key: %w", err)
	}
	return true, nil
}