#!/usr/bin/env bash
# Runs Go linters for the ChronoBunker backend.
# Exits non-zero if vet or gofmt find issues.

set -euo pipefail

BACKEND_DIR="apps/agentops-backend"

echo "Running go vet..."
cd "$BACKEND_DIR"
go vet ./...

echo "Running gofmt check..."
UNFORMATTED=$(gofmt -l .)
if [ -n "$UNFORMATTED" ]; then
    echo "gofmt found unformatted files:"
    echo "$UNFORMATTED"
    echo ""
    echo "Run 'gofmt -w .' inside $BACKEND_DIR to fix."
    exit 1
fi

echo "Lint passed."