#!/usr/bin/env bash
# Mints a development JWT for the given tenant ID.
# Usage: ./scripts/mint_token.sh <tenant_id>

set -euo pipefail

TENANT_ID="${1:?usage: mint_token.sh <tenant_id>}"

if [ ! -f secrets/jwt_private.pem ]; then
    echo "ERROR: secrets/jwt_private.pem not found. Run scripts/generate_keys.sh first." >&2
    exit 1
fi

# Use the Go binary to mint the token, so the claim shape stays in sync
# with the middleware.Claims struct.
cd apps/agentops-backend
CHRONO_TENANT_ID="$TENANT_ID" go run ./cmd/cli mint-token
cd ../..