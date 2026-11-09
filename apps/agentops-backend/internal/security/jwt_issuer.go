package security

import (
	"crypto/rsa"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/transport/middleware"
)

// JWTIssuer mints RS256 session tokens for the frontend.
type JWTIssuer struct {
	privateKey *rsa.PrivateKey
}

// NewJWTIssuer loads the RSA private key from disk.
func NewJWTIssuer(privateKeyPath string) (*JWTIssuer, error) {
	keyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("jwt issuer: read private key: %w", err)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("jwt issuer: parse private key: %w", err)
	}
	return &JWTIssuer{privateKey: key}, nil
}

// Issue mints a short-lived RS256 JWT with the tenant claim.
func (j *JWTIssuer) Issue(tenantID string) (string, error) {
	claims := &middleware.Claims{
		TenantID: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "chronobunker",
			Audience:  jwt.ClaimStrings{"chronobunker-api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(j.privateKey)
}
