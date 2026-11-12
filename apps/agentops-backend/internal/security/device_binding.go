// Package security provides cryptographic device binding primitives.
package security

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
)

// ErrDeviceNotRegistered is returned when a device has no registered public key.
var ErrDeviceNotRegistered = errors.New("security: device has no registered public key")

// ErrDeviceSignatureInvalid is returned when a signed request fails verification.
var ErrDeviceSignatureInvalid = errors.New("security: device signature verification failed")

// ErrDeviceSignatureExpired is returned when the signed payload timestamp is too old.
var ErrDeviceSignatureExpired = errors.New("security: device signature timestamp expired")

// SignedPayload is the canonical payload signed by the device for each tool call.
// The order of fields is fixed and the JSON is compact to ensure deterministic
// serialization across the browser and the server.
type SignedPayload struct {
	SessionID string          `json:"session_id"`
	ToolName  string          `json:"tool_name"`
	DeviceID  string          `json:"device_id"`
	Params    json.RawMessage `json:"params"`
	Timestamp int64           `json:"ts"`
}

// DeviceRegistry stores and verifies Ed25519 public keys registered by devices.
type DeviceRegistry struct {
	db *sql.DB
}

// NewDeviceRegistry constructs a DeviceRegistry.
func NewDeviceRegistry(db *sql.DB) *DeviceRegistry {
	return &DeviceRegistry{db: db}
}

// Register persists the device's Ed25519 public key. It is called once at login.
// The publicKey is the base64url-encoded 32-byte Ed25519 public key.
func (r *DeviceRegistry) Register(ctx context.Context, deviceID, publicKeyB64 string) error {
	tenantID, err := db.TenantIDFromContext(ctx)
	if err != nil {
		return err
	}

	raw, err := base64.RawURLEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return fmt.Errorf("security: decode public key: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return fmt.Errorf("security: invalid ed25519 public key size: %d", len(raw))
	}

	return db.WithTenantTx(ctx, r.db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO device_keys (device_id, tenant_id, public_key)
			VALUES ($1, $2, $3)
			ON CONFLICT (device_id) DO UPDATE
			SET public_key = EXCLUDED.public_key,
			    last_seen_at = now()`,
			deviceID, tenantID, publicKeyB64)
		return err
	})
}

// Verify checks the signature on a signed tool invocation.
//
// The signature must be a base64url-encoded 64-byte Ed25519 signature over the
// canonical JSON of SignedPayload (with the Params field preserved as-is).
// The timestamp must be within maxSkew of the current server time.
func (r *DeviceRegistry) Verify(ctx context.Context, signed SignedPayload, signatureB64 string) error {
	tenantID, err := db.TenantIDFromContext(ctx)
	if err != nil {
		return err
	}

	// 1. Check the timestamp skew before touching the database.
	if maxSkew := 30 * time.Second; time.Since(time.Unix(signed.Timestamp, 0)) > maxSkew {
		return ErrDeviceSignatureExpired
	}

	// 2. Load the public key for this device (RLS-enforced).
	var publicKeyB64 string
	err = db.WithTenantTx(ctx, r.db, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `
			SELECT public_key FROM device_keys WHERE device_id = $1`,
			signed.DeviceID).Scan(&publicKeyB64)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDeviceNotRegistered
	}
	if err != nil {
		return fmt.Errorf("security: load device key: %w", err)
	}

	pub, err := base64.RawURLEncoding.DecodeString(publicKeyB64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("security: stored public key is invalid")
	}

	sig, err := base64.RawURLEncoding.DecodeString(signatureB64)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return ErrDeviceSignatureInvalid
	}

	// 3. Reconstruct the canonical payload bytes. Go's json.Marshal orders
	//    map keys alphabetically, which matches the browser's canonicalJSON.
	//    Using a map[string]any instead of the struct ensures the top-level
	//    keys are sorted, not in struct declaration order.
	canonicalPayload := map[string]any{
		"session_id": signed.SessionID,
		"tool_name":  signed.ToolName,
		"device_id":  signed.DeviceID,
		"params":     signed.Params, // json.RawMessage is preserved byte-for-byte
		"ts":         signed.Timestamp,
	}
	canonical, err := json.Marshal(canonicalPayload)
	if err != nil {
		return fmt.Errorf("security: marshal payload: %w", err)
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), canonical, sig) {
		return ErrDeviceSignatureInvalid
	}

	// 4. Update last_seen_at (best-effort, outside the verification path).
	_ = db.WithTenantTx(ctx, r.db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE device_keys SET last_seen_at = now() WHERE device_id = $1`,
			signed.DeviceID)
		return err
	})
	_ = tenantID // used implicitly by WithTenantTx

	return nil
}

// Fingerprint returns a short hex digest of the public key, useful for logging
// without leaking the key itself.
func Fingerprint(publicKeyB64 string) string {
	raw, _ := base64.RawURLEncoding.DecodeString(publicKeyB64)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}