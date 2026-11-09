//go:build ignore

// mint_token is a development-only utility that prints a JWT for the given
// tenant ID. It signs with the RS256 private key at the path given as the
// second argument, or falls back to ../../secrets/jwt_private.pem relative
// to the apps/agentops-backend working directory.
//
// Usage (from apps/agentops-backend/):
//
//	go run scripts/mint_token.go <tenant_id> [private_key_path]
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	if len(os.Args) < 2 || len(os.Args) > 4 {
		fmt.Fprintln(os.Stderr, "usage: go run scripts/mint_token.go <tenant_id> [device_id] [private_key_path]")
		os.Exit(1)
	}
	tenantID := os.Args[1]
	deviceID := ""
	if len(os.Args) >= 3 {
		deviceID = os.Args[2]
	}
	keyPath := "../../secrets/jwt_private.pem"
	if len(os.Args) == 4 {
		keyPath = os.Args[3]
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read private key %q: %v\n", keyPath, err)
		os.Exit(1)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse private key: %v\n", err)
		os.Exit(1)
	}

	claims := jwt.MapClaims{
		"tenant_id": tenantID,
		"device_id": deviceID,
		"iss":       "chronobunker",
		"aud":       "chronobunker-api",
		"iat":       time.Now().Unix(),
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sign token: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(signed)
}
