// Package middleware provides HTTP middleware for authentication and rate limiting.
package middleware

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/db"
)

// Claims is the JWT payload issued by the ChronoBunker auth service.
type Claims struct {
	TenantID string `json:"tenant_id"`
	jwt.RegisteredClaims
}

// Auth validates bearer tokens and injects the tenant ID into the request context.
type Auth struct {
	publicKey *rsa.PublicKey
}

// NewAuth loads the RSA public key from disk. It fails fast if the key is
// missing or malformed, so the server never starts without a working validator.
func NewAuth(publicKeyPath string) (*Auth, error) {
	keyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("auth: read public key: %w", err)
	}
	pub, err := jwt.ParseRSAPublicKeyFromPEM(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("auth: parse public key: %w", err)
	}
	return &Auth{publicKey: pub}, nil
}

// NewAuthFromKey is used by tests to inject an in-memory public key.
func NewAuthFromKey(pub *rsa.PublicKey) (*Auth, error) {
	if pub == nil {
		return nil, fmt.Errorf("auth: nil public key")
	}
	return &Auth{publicKey: pub}, nil
}

// Middleware validates the Authorization header and stores the tenant ID in
// the context using db.WithTenantID, so downstream resolvers retrieve it
// without touching the raw token.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, `{"error":"invalid authorization header format"}`, http.StatusUnauthorized)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(
			parts[1],
			claims,
			func(t *jwt.Token) (any, error) {
				// Critical: pin the signing method. Without this, an attacker
				// can switch alg from RS256 to HS256 and sign with the public
				// key bytes, bypassing verification entirely.
				if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return a.publicKey, nil
			},
			jwt.WithValidMethods([]string{"RS256"}),
			jwt.WithIssuer("chronobunker"),
			jwt.WithExpirationRequired(),
		)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}
		if !token.Valid {
			http.Error(w, `{"error":"token is not valid"}`, http.StatusUnauthorized)
			return
		}
		if claims.TenantID == "" {
			http.Error(w, `{"error":"token carries no tenant_id claim"}`, http.StatusUnauthorized)
			return
		}

		ctx := db.WithTenantID(r.Context(), claims.TenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}