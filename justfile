# ChronoBunker Core - Local Orchestration

set shell := ["bash", "-c"]
set dotenv-load := true

backend_dir := "apps/agentops-backend"
api_binary  := if os() == "windows" { "bin/chronobunker-api.exe" } else { "bin/chronobunker-api" }
worker_binary := if os() == "windows" { "bin/chronobunker-worker.exe" } else { "bin/chronobunker-worker" }
cli_binary  := if os() == "windows" { "bin/chronobunker-cli.exe" } else { "bin/chronobunker-cli" }

default:
    @just --list

check-infra:
    @bash scripts/check_ports.sh

db-migrate: check-infra
    @bash scripts/db_migrate.sh

db-seed: check-infra
    @bash scripts/db_seed.sh

build:
    @echo "Building binaries..."
    @mkdir -p bin
    @cd {{backend_dir}} && go build -o ../../{{api_binary}} ./cmd/api
    @cd {{backend_dir}} && go build -o ../../{{worker_binary}} ./cmd/worker
    @cd {{backend_dir}} && go build -o ../../{{cli_binary}} ./cmd/cli
    @echo "Binaries built: {{api_binary}}, {{worker_binary}}, {{cli_binary}}"

dev-api: check-infra build
    @echo "Starting API server..."
    -@./{{api_binary}}

dev-worker: check-infra build
    @echo "Starting worker..."
    -@./{{worker_binary}}

test:
    @echo "Running Go tests..."
    @cd {{backend_dir}} && go test ./... -count=1

lint:
    @bash scripts/lint.sh

clean:
    @rm -rf bin/