#!/usr/bin/env bash
# Generates an RSA 4096-bit key pair for JWT signing.
# The private key mints tokens. The public key validates them.
# In production, the private key never leaves the auth service.

set -euo pipefail

SECRETS_DIR="secrets"
PRIVATE_KEY="$SECRETS_DIR/jwt_private.pem"
PUBLIC_KEY="$SECRETS_DIR/jwt_public.pem"

mkdir -p "$SECRETS_DIR"

if [ -f "$PRIVATE_KEY" ]; then
    echo "Private key already exists at $PRIVATE_KEY. Skipping generation."
    exit 0
fi

echo "Generating RSA 4096-bit key pair..."
openssl genpkey -algorithm RSA -out "$PRIVATE_KEY" -pkeyopt rsa_keygen_bits:4096
openssl rsa -pubout -in "$PRIVATE_KEY" -out "$PUBLIC_KEY"

chmod 600 "$PRIVATE_KEY"
chmod 644 "$PUBLIC_KEY"

echo "Keys generated:"
echo "  Private: $PRIVATE_KEY (keep secret)"
echo "  Public:  $PUBLIC_KEY  (safe to distribute)"