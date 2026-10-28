package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// CapabilityToken is the set of permissions granted to an agent for a single
// session. Signed with Ed25519.
type CapabilityToken struct {
	SessionID    string   `json:"sid"`
	TenantID     string   `json:"tid"`
	UserID       string   `json:"uid"`
	AllowedTools []string `json:"tools"`
	ResourceIDs  []string `json:"resources"`
	MaxAmount    float64  `json:"max_amt"`
	IssuedAt     int64    `json:"iat"`
	ExpiresAt    int64    `json:"exp"`
}

// TokenService issues and verifies capability tokens.
type TokenService struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// NewTokenService generates an ephemeral Ed25519 key pair.
func NewTokenService() (*TokenService, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("security: generate key: %w", err)
	}
	return &TokenService{privateKey: priv, publicKey: pub}, nil
}

// NewTokenServiceFromKeys constructs a TokenService from an existing private key.
func NewTokenServiceFromKeys(priv ed25519.PrivateKey) (*TokenService, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("security: invalid private key size")
	}
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("security: cannot derive public key")
	}
	return &TokenService{privateKey: priv, publicKey: pub}, nil
}

// PublicKey returns the public key for distribution to verifiers.
func (ts *TokenService) PublicKey() ed25519.PublicKey {
	return ts.publicKey
}

// Issue creates a signed token with a 5-minute TTL.
func (ts *TokenService) Issue(sessionID, tenantID, userID string, allowedTools, resourceIDs []string, maxAmount float64) (string, error) {
	now := time.Now()
	tok := CapabilityToken{
		SessionID:    sessionID,
		TenantID:     tenantID,
		UserID:       userID,
		AllowedTools: allowedTools,
		ResourceIDs:  resourceIDs,
		MaxAmount:    maxAmount,
		IssuedAt:     now.Unix(),
		ExpiresAt:    now.Add(5 * time.Minute).Unix(),
	}
	payload, err := json.Marshal(tok)
	if err != nil {
		return "", fmt.Errorf("security: marshal token: %w", err)
	}
	sig := ed25519.Sign(ts.privateKey, payload)
	return base64Encode(payload) + "." + base64Encode(sig), nil
}

// Verify parses and validates a signed token.
func (ts *TokenService) Verify(signed string) (*CapabilityToken, error) {
	payloadB64, sigB64, ok := splitToken(signed)
	if !ok {
		return nil, errors.New("security: malformed token")
	}
	payload, err := base64Decode(payloadB64)
	if err != nil {
		return nil, fmt.Errorf("security: decode payload: %w", err)
	}
	sig, err := base64Decode(sigB64)
	if err != nil {
		return nil, fmt.Errorf("security: decode signature: %w", err)
	}
	if !ed25519.Verify(ts.publicKey, payload, sig) {
		return nil, errors.New("security: invalid signature")
	}
	var tok CapabilityToken
	if err := json.Unmarshal(payload, &tok); err != nil {
		return nil, fmt.Errorf("security: unmarshal claims: %w", err)
	}
	if time.Now().Unix() > tok.ExpiresAt {
		return nil, errors.New("security: token expired")
	}
	return &tok, nil
}