SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

GOOSE_VERSION := v3.27.3
GOOSE_BUILD_TAGS := no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb
SQLC_VERSION := v1.31.1
OAPI_CODEGEN_VERSION := v2.8.0
GOVULNCHECK_VERSION := v1.7.0
DATABASE_URL ?= postgresql://bridgeyok:bridgeyok@localhost:5432/bridgeyok?sslmode=disable
MIGRATION_DATABASE_URL ?= $(DATABASE_URL)

.PHONY: help install db-up db-stop db-down db-logs migrate-up migrate-down migrate-status migrate-validate db-fixture generate generate-db generate-contracts generate-api test-api test-api-integration vet-api security-api build-api run-api smoke-api gate-local gate-local-down bootstrap

help:
	@echo "make bootstrap             Start PostgreSQL, migrate, generate, and verify the fixture"
	@echo "make run-api               Run the API against local PostgreSQL"
	@echo "make test-api              Run API unit tests with the race detector"
	@echo "make test-api-integration  Run database integration tests"
	@echo "make smoke-api             Verify HTTP, CORS, readiness, and graceful shutdown"
	@echo "make gate-local            Run HTTPS/WSS deploy and rollback gates with Supabase"
	@echo "make gate-local-down       Stop the local gate without changing Supabase"
	@echo "make db-stop               Stop local PostgreSQL without deleting data"

install:
	corepack pnpm install --frozen-lockfile

db-up:
	docker compose up -d --wait postgres

db-stop:
	docker compose stop postgres

db-down:
	docker compose down

db-logs:
	docker compose logs -f postgres

migrate-up:
	cd apps/api && go run -tags="$(GOOSE_BUILD_TAGS)" github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir ../../db/migrations postgres "$(MIGRATION_DATABASE_URL)" up

migrate-down:
	cd apps/api && go run -tags="$(GOOSE_BUILD_TAGS)" github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir ../../db/migrations postgres "$(MIGRATION_DATABASE_URL)" down

migrate-status:
	cd apps/api && go run -tags="$(GOOSE_BUILD_TAGS)" github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir ../../db/migrations postgres "$(MIGRATION_DATABASE_URL)" status

migrate-validate:
	cd apps/api && go run -tags="$(GOOSE_BUILD_TAGS)" github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir ../../db/migrations validate

db-fixture:
	docker compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"' < db/fixtures/phase1.sql

generate-db:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

generate-api:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) -config packages/contracts/oapi-codegen.yaml packages/contracts/openapi.yaml

generate-contracts: generate-api
	corepack pnpm --filter @bridgeyok/contracts generate

generate: generate-db generate-contracts

test-api:
	go test -race ./apps/api/...

test-api-integration:
	TEST_DATABASE_URL="$(DATABASE_URL)" go test -race -tags=integration ./apps/api/internal/database

vet-api:
	go vet ./apps/api/...

security-api:
	cd apps/api && go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

build-api:
	go build -o bin/bridgeyok-api ./apps/api/cmd/api

run-api:
	DATABASE_URL="$(DATABASE_URL)" ALLOWED_ORIGINS="$${ALLOWED_ORIGINS:-http://localhost:3000}" go run ./apps/api/cmd/api

smoke-api:
	DATABASE_URL="$(DATABASE_URL)" ./scripts/smoke-api.sh

gate-local:
	./scripts/local-gate.sh run

gate-local-down:
	./scripts/local-gate.sh down

bootstrap: install db-up migrate-up generate db-fixture
