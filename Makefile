SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

GOOSE_VERSION := v3.27.3
GOOSE_BUILD_TAGS := no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb
SQLC_VERSION := v1.31.1
OAPI_CODEGEN_VERSION := v2.8.0
GOVULNCHECK_VERSION := v1.7.0
GOLANGCI_LINT_VERSION := v2.13.2
DATABASE_URL ?=
MIGRATION_DATABASE_URL ?= $(DATABASE_URL)
export DATABASE_URL
export MIGRATION_DATABASE_URL

.PHONY: help install require-database-url migrate-up migrate-down migrate-status migrate-validate generate generate-db generate-contracts generate-api test-api test-api-integration test-engine test-engine-stress test-engine-fixtures fuzz-engine vet-api vet-engine lint-api lint-engine security-api security-engine gate-engine build-api run-api smoke-api gate-local gate-local-down bootstrap

help:
	@echo "make bootstrap             Install, migrate Supabase, and generate sources"
	@echo "make run-api               Run the local API against Supabase"
	@echo "make test-api              Run API unit tests with the race detector"
	@echo "make gate-engine           Run the complete local Phase 2 engine gate"
	@echo "make test-engine-stress    Run the manual 10,000-board race stress test"
	@echo "make test-engine-fixtures  Run test-only fixture serialization tests"
	@echo "make fuzz-engine           Run bounded card, decision, and fixture fuzzing"
	@echo "make test-api-integration  Run database integration tests"
	@echo "make smoke-api             Verify HTTP, CORS, readiness, and graceful shutdown"
	@echo "make gate-local            Run HTTPS/WSS deploy and rollback gates with Supabase"
	@echo "make gate-local-down       Stop the local gate without changing Supabase"

install:
	corepack pnpm install --frozen-lockfile

require-database-url:
	@if [[ -z "$${DATABASE_URL}" ]]; then echo "DATABASE_URL is required; use the Supabase session-pooler URL with sslmode=require." >&2; exit 1; fi

migrate-up: require-database-url
	@cd apps/api && go run -tags="$(GOOSE_BUILD_TAGS)" github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir ../../db/migrations postgres "$${MIGRATION_DATABASE_URL}" up

migrate-down: require-database-url
	@if [[ "$${ALLOW_MIGRATION_DOWN:-}" != "1" ]]; then echo "Migration down is disabled for Supabase safety; only isolated CI may set ALLOW_MIGRATION_DOWN=1." >&2; exit 1; fi
	@cd apps/api && go run -tags="$(GOOSE_BUILD_TAGS)" github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir ../../db/migrations postgres "$${MIGRATION_DATABASE_URL}" down

migrate-status: require-database-url
	@cd apps/api && go run -tags="$(GOOSE_BUILD_TAGS)" github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir ../../db/migrations postgres "$${MIGRATION_DATABASE_URL}" status

migrate-validate:
	cd apps/api && go run -tags="$(GOOSE_BUILD_TAGS)" github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir ../../db/migrations validate

generate-db:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

generate-api:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) -config packages/contracts/oapi-codegen.yaml packages/contracts/openapi.yaml

generate-contracts: generate-api
	corepack pnpm --filter @bridgeyok/contracts generate

generate: generate-db generate-contracts

test-api:
	go test -race ./apps/api/...

test-engine:
	go test -race -timeout=15m ./apps/api/internal/bridge
	go test -cover -timeout=10m ./apps/api/internal/bridge

test-engine-stress:
	BRIDGE_ENGINE_STRESS=1 go test -race -timeout=15m ./apps/api/internal/bridge -run '^TestRandomizedLegalGames$$' -count=1

test-engine-fixtures:
	go test -race -cover -tags=testfixture ./apps/api/internal/bridgefixture

fuzz-engine:
	go test ./apps/api/internal/bridge -run '^$$' -fuzz '^FuzzParseCard$$' -fuzztime=2s -parallel=2
	go test ./apps/api/internal/bridge -run '^$$' -fuzz '^FuzzDecide$$' -fuzztime=2s -parallel=2
	go test -tags=testfixture ./apps/api/internal/bridgefixture -run '^$$' -fuzz '^FuzzUnmarshal$$' -fuzztime=2s -parallel=2

test-api-integration: require-database-url
	@TEST_DATABASE_URL="$${DATABASE_URL}" go test -race -tags=integration ./apps/api/internal/database

vet-api:
	go vet ./apps/api/...

vet-engine:
	go vet ./apps/api/internal/bridge
	go vet -tags=testfixture ./apps/api/internal/bridgefixture

lint-api:
	cd apps/api && GOWORK=off go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

lint-engine:
	cd apps/api && GOWORK=off go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --build-tags=testfixture ./internal/bridge ./internal/bridgefixture

security-api:
	cd apps/api && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

security-engine:
	cd apps/api && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) -tags=testfixture ./internal/bridge ./internal/bridgefixture

gate-engine: vet-engine lint-engine test-engine test-engine-fixtures fuzz-engine security-engine

build-api:
	go build -o bin/bridgeyok-api ./apps/api/cmd/api

run-api: require-database-url
	@ALLOWED_ORIGINS="$${ALLOWED_ORIGINS:-http://localhost:3000}" go run ./apps/api/cmd/api

smoke-api: require-database-url
	@./scripts/smoke-api.sh

gate-local:
	@env -u DATABASE_URL -u MIGRATION_DATABASE_URL ./scripts/local-gate.sh run

gate-local-down:
	@env -u DATABASE_URL -u MIGRATION_DATABASE_URL ./scripts/local-gate.sh down

bootstrap: install migrate-up generate
