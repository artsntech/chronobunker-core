#!/usr/bin/env bash
# Applies all SQL migrations in lexicographic order.
# Requires: psql in PATH, and the CHRONO_DB_* environment variables set.
# Exits non-zero on the first migration failure.

set -euo pipefail

# Build the connection string from environment variables.
DB_URL="postgres://${CHRONO_DB_USER}:${CHRONO_DB_PASSWORD}@${CHRONO_DB_HOST}:${CHRONO_DB_PORT}/${CHRONO_DB_NAME}?sslmode=${CHRONO_DB_SSLMODE}"

MIGRATIONS_DIR="db/migrations"

if [ ! -d "$MIGRATIONS_DIR" ]; then
    echo "ERROR: migrations directory '$MIGRATIONS_DIR' does not exist" >&2
    exit 1
fi

echo "Applying database migrations from $MIGRATIONS_DIR..."

for f in "$MIGRATIONS_DIR"/*.sql; do
    if [ ! -f "$f" ]; then
        echo "ERROR: no .sql files found in $MIGRATIONS_DIR" >&2
        exit 1
    fi
    echo "  -> $f"
    if ! psql "$DB_URL" -v ON_ERROR_STOP=1 -f "$f"; then
        echo "ERROR: migration '$f' failed" >&2
        exit 1
    fi
done

echo "Migrations applied."