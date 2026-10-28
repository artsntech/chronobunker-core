#!/usr/bin/env bash
# Loads development seed data. Requires the CHRONO_DB_* environment variables set.

set -euo pipefail

DB_URL="postgres://${CHRONO_DB_USER}:${CHRONO_DB_PASSWORD}@${CHRONO_DB_HOST}:${CHRONO_DB_PORT}/${CHRONO_DB_NAME}?sslmode=${CHRONO_DB_SSLMODE}"

SEED_DIR="db/seed"

if [ ! -d "$SEED_DIR" ]; then
    echo "ERROR: seed directory '$SEED_DIR' does not exist" >&2
    exit 1
fi

echo "Loading seed data from $SEED_DIR..."

for f in "$SEED_DIR"/*.sql; do
    if [ ! -f "$f" ]; then
        echo "No seed files found in $SEED_DIR (skipping)."
        exit 0
    fi
    echo "  -> $f"
    if ! psql "$DB_URL" -v ON_ERROR_STOP=1 -f "$f"; then
        echo "ERROR: seed file '$f' failed" >&2
        exit 1
    fi
done

echo "Seed data loaded."