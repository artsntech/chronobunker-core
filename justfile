# ChronoBunker Core - Local Orchestration
# Cross-platform task runner. Works on Windows, Linux, and macOS.
# Install: https://github.com/casey/just

set shell := ["bash", "-c"]
set dotenv-load := true

backend_dir := "apps/agentops-backend"
api_binary  := "bin/chronobunker-api"
worker_binary := "bin/chronobunker-worker"
cli_binary  := "bin/chronobunker-cli"

# Show available recipes
default:
    @just --list

# Validate that Postgres, Redis (and Kafka if enabled) are reachable
check-infra:
    @bash scripts/check_ports.sh

# Apply all SQL migrations in order
db-migrate: check-infra
    @echo "Applying database migrations..."
    @for f in db/migrations/*.sql; do \
        echo "  -> $$f"; \
        psql "postgres://{{env_var_or_default('CHRONO_DB_USER', 'chrono_app')}}:{{env_var_or_default('CHRONO_DB_PASSWORD', '')}}@{{env_var_or_default('CHRONO_DB_HOST', 'localhost')}}:{{env_var_or_default('CHRONO_DB_PORT', '5432')}}/{{env_var_or_default('CHRONO_DB_NAME', 'chronobunker')}}?sslmode={{env_var_or_default('CHRONO_DB_SSLMODE', 'disable')}}" -f "$$f" || exit 1; \
    done
    @echo "Migrations applied."

# Load seed data (development only)
db-seed: check-infra
    @echo "Loading seed data..."
    @for f in db/seed/*.sql; do \
        echo "  -> $$f"; \
        psql "postgres://{{env_var_or_default('CHRONO_DB_USER', 'chrono_app')}}:{{env_var_or_default('CHRONO_DB_PASSWORD', '')}}@{{env_var_or_default('CHRONO_DB_HOST', 'localhost')}}:{{env_var_or_default('CHRONO_DB_PORT', '5432')}}/{{env_var_or_default('CHRONO_DB_NAME', 'chronobunker')}}?sslmode={{env_var_or_default('CHRONO_DB_SSLMODE', 'disable')}}" -f "$$f" || exit 1; \
    done
    @echo "Seed data loaded."

# Compile all three binaries
build:
    @echo "Building binaries..."
    @mkdir -p bin
    @cd {{backend_dir}} && go build -o ../../{{api_binary}} ./cmd/api
    @cd {{backend_dir}} && go build -o ../../{{worker_binary}} ./cmd/worker
    @cd {{backend_dir}} && go build -o ../../{{cli_binary}} ./cmd/cli
    @echo "Binaries built: {{api_binary}}, {{worker_binary}}, {{cli_binary}}"

# Build and run the API server
dev-api: check-infra build
    @echo "Starting API server..."
    @./{{api_binary}}

# Build and run the outbox/Kafka worker
dev-worker: check-infra build
    @echo "Starting worker..."
    @./{{worker_binary}}

# Run all Go tests with the race detector
test:
    @echo "Running Go tests..."
    @cd {{backend_dir}} && go test ./... -count=1

# Run go vet and gofmt checks
lint:
    @bash scripts/lint.sh

# Remove compiled binaries
clean:
    @echo "Cleaning compiled binaries..."
    @rm -rf bin/
    @echo "Clean complete."