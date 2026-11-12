package security_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/security"
)

const tenantAcme = "11111111-1111-1111-1111-111111111111"

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("CHRONO_TEST_DB_URL")
	if url == "" {
		t.Skip("CHRONO_TEST_DB_URL not set")
	}
	conn, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// registerDevice generates a fresh Ed25519 key pair, registers the public key
// for the given device under the tenant in ctx, and returns the private key
// so the caller can sign ToolRequests.
func registerDevice(t *testing.T, database *sql.DB, ctx context.Context, deviceID string) ed25519.PrivateKey {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	reg := security.NewDeviceRegistry(database)
	if err := reg.Register(ctx, deviceID, base64.RawURLEncoding.EncodeToString(pub)); err != nil {
		t.Fatalf("register device: %v", err)
	}
	return priv
}

// signToolCall builds the canonical SignedPayload that the Gateway reconstructs
// from ToolRequest and signs it with priv. It returns the base64url signature
// and the timestamp used, so the caller can pass both in the ToolRequest.
func signToolCall(t *testing.T, priv ed25519.PrivateKey, sessionID, toolName, deviceID string, params map[string]any) (string, int64) {
	t.Helper()
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	signed := security.SignedPayload{
		SessionID: sessionID,
		ToolName:  toolName,
		DeviceID:  deviceID,
		Params:    paramsJSON,
		Timestamp: time.Now().Unix(),
	}
	canonical, err := json.Marshal(signed)
	if err != nil {
		t.Fatalf("marshal signed payload: %v", err)
	}
	sig := ed25519.Sign(priv, canonical)
	return base64.RawURLEncoding.EncodeToString(sig), signed.Timestamp
}

// ── Test 1: cross-tenant resource binding is blocked ─────────────────────

func TestGatewayRejectsCrossTenantResourceBinding(t *testing.T) {
	database := testDB(t)
	tokens, _ := security.NewTokenService()

	ctx := db.WithTenantID(context.Background(), tenantAcme)
	priv := registerDevice(t, database, ctx, "device-test-1")

	// Token grants access only to order ORD-1234, with a $50 max.
	signed, err := tokens.Issue("sess-1", tenantAcme, "user-1", "device-test-1",
		[]string{"issue_refund"}, []string{"ORD-1234"}, 50.0)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	var executed int32
	gw := security.NewGateway(database, tokens, func(ctx context.Context, name string, params map[string]any) (any, error) {
		atomic.AddInt32(&executed, 1)
		return "refunded", nil
	})

	// Params reference ORD-9999, which is NOT in the token's ResourceIDs.
	params := map[string]any{"order_id": "ORD-9999", "amount": 10.0, "reason": "test"}
	sig, ts := signToolCall(t, priv, "sess-1", "issue_refund", "device-test-1", params)

	_, err = gw.Invoke(ctx, security.ToolRequest{
		SignedToken: signed,
		DeviceID:    "device-test-1",
		ToolName:    "issue_refund",
		Params:      params,
		Signature:   sig,
		Timestamp:   ts,
	})
	if err == nil {
		t.Fatal("expected cross-tenant resource binding to be rejected")
	}
	if atomic.LoadInt32(&executed) != 0 {
		t.Fatal("executor must not be called when binding is violated")
	}
}

// ── Test 2: idempotency returns cache on retry ──────────────────────────

func TestGatewayIsIdempotentOnRetry(t *testing.T) {
	database := testDB(t)
	tokens, _ := security.NewTokenService()

	sessionID := fmt.Sprintf("sess-idem-%d", time.Now().UnixNano())
	deviceID := "device-test-2"

	ctx := db.WithTenantID(context.Background(), tenantAcme)
	priv := registerDevice(t, database, ctx, deviceID)

	signed, err := tokens.Issue(sessionID, tenantAcme, "user-1", deviceID,
		[]string{"issue_refund"}, []string{"ORD-1234"}, 50.0)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	var calls int32
	gw := security.NewGateway(database, tokens, func(ctx context.Context, name string, params map[string]any) (any, error) {
		atomic.AddInt32(&calls, 1)
		return map[string]any{"refund_id": "R-1"}, nil
	})

	params := map[string]any{"order_id": "ORD-1234", "amount": 10.0, "reason": "test"}

	// First call: signs and executes.
	sig1, ts1 := signToolCall(t, priv, sessionID, "issue_refund", deviceID, params)
	resp1, err := gw.Invoke(ctx, security.ToolRequest{
		SignedToken: signed,
		DeviceID:    deviceID,
		ToolName:    "issue_refund",
		Params:      params,
		Signature:   sig1,
		Timestamp:   ts1,
	})
	if err != nil {
		t.Fatalf("first invoke: %v", err)
	}
	if !resp1.Executed {
		t.Errorf("first invoke should have executed, got Executed=%v Cached=%v", resp1.Executed, resp1.Cached)
	}

	// Second call: signs again (different timestamp), idempotency returns cache.
	sig2, ts2 := signToolCall(t, priv, sessionID, "issue_refund", deviceID, params)
	resp2, err := gw.Invoke(ctx, security.ToolRequest{
		SignedToken: signed,
		DeviceID:    deviceID,
		ToolName:    "issue_refund",
		Params:      params,
		Signature:   sig2,
		Timestamp:   ts2,
	})
	if err != nil {
		t.Fatalf("second invoke: %v", err)
	}
	if !resp2.Cached {
		t.Errorf("second invoke should have returned cached result, got Executed=%v Cached=%v", resp2.Executed, resp2.Cached)
	}

	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("executor called %d times, expected exactly 1", calls)
	}
}

// ── Test 3: DeviceID mismatch is rejected before signature check ────────

func TestGatewayRejectsDeviceMismatch(t *testing.T) {
	database := testDB(t)
	tokens, _ := security.NewTokenService()

	ctx := db.WithTenantID(context.Background(), tenantAcme)

	// Register device-A only. The request will arrive claiming device-B.
	priv := registerDevice(t, database, ctx, "device-A")

	signed, _ := tokens.Issue("sess-dev", tenantAcme, "user-1", "device-A",
		[]string{"issue_refund"}, []string{"ORD-1234"}, 50.0)

	var executed int32
	gw := security.NewGateway(database, tokens, func(ctx context.Context, name string, params map[string]any) (any, error) {
		atomic.AddInt32(&executed, 1)
		return "should not run", nil
	})

	params := map[string]any{"order_id": "ORD-1234", "amount": 10.0, "reason": "test"}
	// Sign with device-A's key, but claim device-B in the request. The Gateway
	// rejects on the DeviceID mismatch before it ever reads the signature.
	sig, ts := signToolCall(t, priv, "sess-dev", "issue_refund", "device-B", params)

	_, err := gw.Invoke(ctx, security.ToolRequest{
		SignedToken: signed,
		DeviceID:    "device-B",
		ToolName:    "issue_refund",
		Params:      params,
		Signature:   sig,
		Timestamp:   ts,
	})
	if err == nil {
		t.Fatal("expected device mismatch to be rejected")
	}
	if atomic.LoadInt32(&executed) != 0 {
		t.Fatal("executor must not be called on device mismatch")
	}
}

// ── Test 4: invalid signature is rejected even with correct JWT+DeviceID ─

func TestGatewayRejectsInvalidSignature(t *testing.T) {
	database := testDB(t)
	tokens, _ := security.NewTokenService()

	ctx := db.WithTenantID(context.Background(), tenantAcme)
	// Register a legitimate device key.
	priv := registerDevice(t, database, ctx, "device-sig")

	// An attacker steals the JWT and the DeviceID but does not have the private
	// key. They try to sign with their own key.
	attackerPub, attackerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate attacker key: %v", err)
	}
	_ = attackerPub // the attacker's public key is never registered

	signed, _ := tokens.Issue("sess-sig", tenantAcme, "user-1", "device-sig",
		[]string{"issue_refund"}, []string{"ORD-1234"}, 50.0)

	var executed int32
	gw := security.NewGateway(database, tokens, func(ctx context.Context, name string, params map[string]any) (any, error) {
		atomic.AddInt32(&executed, 1)
		return "should not run", nil
	})

	params := map[string]any{"order_id": "ORD-1234", "amount": 10.0, "reason": "test"}

	// Sign with the attacker's key, not the registered device key.
	paramsJSON, _ := json.Marshal(params)
	signedPayload := security.SignedPayload{
		SessionID: "sess-sig",
		ToolName:  "issue_refund",
		DeviceID:  "device-sig",
		Params:    paramsJSON,
		Timestamp: time.Now().Unix(),
	}
	canonical, _ := json.Marshal(signedPayload)
	attackerSig := ed25519.Sign(attackerPriv, canonical)

	_, err = gw.Invoke(ctx, security.ToolRequest{
		SignedToken: signed,
		DeviceID:    "device-sig",
		ToolName:    "issue_refund",
		Params:      params,
		Signature:   base64.RawURLEncoding.EncodeToString(attackerSig),
		Timestamp:   signedPayload.Timestamp,
	})
	if err == nil {
		t.Fatal("SECURITY: request with attacker signature was accepted")
	}
	if !errors.Is(err, security.ErrDeviceSignatureInvalid) {
		t.Fatalf("expected ErrDeviceSignatureInvalid, got: %v", err)
	}
	if atomic.LoadInt32(&executed) != 0 {
		t.Fatal("executor must not be called when signature is invalid")
	}

	// Control: the legitimate private key (registered device) is accepted.
	sig, ts := signToolCall(t, priv, "sess-sig", "issue_refund", "device-sig", params)
	_, err = gw.Invoke(ctx, security.ToolRequest{
		SignedToken: signed,
		DeviceID:    "device-sig",
		ToolName:    "issue_refund",
		Params:      params,
		Signature:   sig,
		Timestamp:   ts,
	})
	if err != nil {
		t.Fatalf("legitimate signature was rejected: %v", err)
	}
}
