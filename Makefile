# ChronoBunker Core - Local Orchestration
# All targets assume a portable local environment (no Docker).
# On Windows, use Git Bash, WSL, or run the .bat equivalents directly.

SHELL := /bin/bash
.DEFAULT_GOAL := help

# --- Configuration ---
BACKEND_DIR := apps/agentops-backend
API_BINARY  := bin/chronobunker-api
WORKER_BINARY := bin/chronobunker-worker
CLI_BINARY  := bin/chronobunker-cli

# Load .env if present
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: help
help:
	@echo "ChronoBunker Core - Available targets:"
	@echo ""
	@echo "  make check-infra     Validate that Postgres, Redis (and Kafka if enabled) are reachable"
	@echo "  make db-migrate      Apply all SQL migrations in order"
	@echo "  make db-seed         Load seed data (development only)"
	@echo "  make build           Compile all three binaries (api, worker, cli)"
	@echo "  make dev-api         Build and run the HTTP/GraphQL server"
	@echo "  make dev-worker      Build and run the outbox/Kafka worker"
	@echo "  make test            Run all Go tests"
	@echo "  make lint            Run go vet and gofmt checks"
	@echo "  make clean           Remove compiled binaries"

.PHONY: check-infra
check-infra:
	@bash scripts/check_ports.sh

.PHONY: db-migrate
db-migrate: check-infra
	@echo "Applying database migrations..."
	@for f in db/migrations/*.sql; do \
		echo "  -> $$f"; \
		psql "postgres://$(CHRONO_DB_USER):$(CHRONO_DB_PASSWORD)@$(CHRONO_DB_HOST):$(CHRONO_DB_PORT)/$(CHRONO_DB_NAME)?sslmode=$(CHRONO_DB_SSLMODE)" -f "$$f" || exit 1; \
	done
	@echo "Migrations applied."

.PHONY: db-seed
db-seed: check-infra
	@echo "Loading seed data..."
	@for f in db/seed/*.sql; do \
		echo "  -> $$f"; \
		psql "postgres://$(CHRONO_DB_USER):$(CHRONO_DB_PASSWORD)@$(CHRONO_DB_HOST):$(CHRONO_DB_PORT)/$(CHRONO_DB_NAME)?sslmode=$(CHRONO_DB_SSLMODE)" -f "$$f" || exit 1; \
	done
	@echo "Seed data loaded."

.PHONY: build
build:
	@echo "Building binaries..."
	@mkdir -p bin
	@cd $(BACKEND_DIR) && go build -o ../../$(API_BINARY) ./cmd/api
	@cd $(BACKEND_DIR) && go build -o ../../$(WORKER_BINARY) ./cmd/worker
	@cd $(BACKEND_DIR) && go build -o ../../$(CLI_BINARY) ./cmd/cli
	@echo "Binaries built: $(API_BINARY), $(WORKER_BINARY), $(CLI_BINARY)"

.PHONY: dev-api
dev-api: check-infra build
	@echo "Starting API server..."
	@./$(API_BINARY)

.PHONY: dev-worker
dev-worker: check-infra build
	@echo "Starting worker..."
	@./$(WORKER_BINARY)

.PHONY: test
test:
	@echo "Running Go tests..."
	@cd $(BACKEND_DIR) && go test ./... -race -count=1

.PHONY: lint
lint:
	@echo "Running linters..."
	@cd $(BACKEND_DIR) && go vet ./...
	@cd $(BACKEND_DIR) && test -z "$$(gofmt -l .)" || (echo "gofmt found unformatted files:" && gofmt -l . && exit 1)
	@echo "Lint passed."

.PHONY: clean
clean:
	@echo "Cleaning compiled binaries..."
	@rm -rf bin/
	@echo "Clean complete."