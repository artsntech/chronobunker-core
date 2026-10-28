#!/usr/bin/env bash
# Applies all SQL migrations in lexicographic order.
# Tracks applied migrations in the schema_migrations table so that
# re-running the script is safe and only new migrations are applied.
#
# Requires: psql in PATH, and the CHRONO_DB_* environment variables set.

set -euo pipefail

DB_URL="postgres://${CHRONO_DB_USER}:${CHRONO_DB_PASSWORD}@${CHRONO_DB_HOST}:${CHRONO_DB_PORT}/${CHRONO_DB_NAME}?sslmode=${CHRONO_DB_SSLMODE}"
MIGRATIONS_DIR="db/migrations"

if [ ! -d "$MIGRATIONS_DIR" ]; then
    echo "ERROR: migrations directory '$MIGRATIONS_DIR' does not exist" >&2
    exit 1
fi

# Ensure the tracking table exists.
psql "$DB_URL" -v ON_ERROR_STOP=1 -q <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     TEXT PRIMARY KEY,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
SQL

echo "Applying database migrations from $MIGRATIONS_DIR..."

for f in "$MIGRATIONS_DIR"/*.sql; do
    if [ ! -f "$f" ]; then
        echo "ERROR: no .sql files found in $MIGRATIONS_DIR" >&2
        exit 1
    fi
    version=$(basename "$f")

    # Check if this migration has already been applied.
    applied=$(psql "$DB_URL" -tAc "SELECT 1 FROM schema_migrations WHERE version = '$version'")
    if [ "$applied" = "1" ]; then
        echo "  -> $version (already applied, skipping)"
        continue
    fi

    echo "  -> $version"
    if ! psql "$DB_URL" -v ON_ERROR_STOP=1 -f "$f"; then
        echo "ERROR: migration '$f' failed" >&2
        exit 1
    fi

    # Record the migration as applied.
    psql "$DB_URL" -v ON_ERROR_STOP=1 -q -c \
        "INSERT INTO schema_migrations (version) VALUES ('$version')"
done

echo "Migrations applied."