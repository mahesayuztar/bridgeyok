SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

GOOSE_VERSION := v3.27.3
GOOSE_BUILD_TAGS := no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb
SQLC_VERSION := v1.31.1
DATABASE_URL ?= postgresql://bridgeyok:bridgeyok@localhost:5432/bridgeyok?sslmode=disable
MIGRATION_DATABASE_URL ?= $(DATABASE_URL)

.PHONY: help db-up db-stop db-down db-logs migrate-up migrate-down migrate-status migrate-validate db-fixture generate-db test-api test-api-integration build-api run-api bootstrap

help:
	@echo "make bootstrap             Start PostgreSQL, migrate, generate, and verify the fixture"
	@echo "make run-api               Run the API against local PostgreSQL"
	@echo "make test-api              Run API unit tests with the race detector"
	@echo "make test-api-integration  Run database integration tests"
	@echo "make db-stop               Stop local PostgreSQL without deleting data"

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

test-api:
	go test -race ./apps/api/...

test-api-integration:
	TEST_DATABASE_URL="$(DATABASE_URL)" go test -race -tags=integration ./apps/api/internal/database

build-api:
	go build -o bin/bridgeyok-api ./apps/api/cmd/api

run-api:
	DATABASE_URL="$(DATABASE_URL)" ALLOWED_ORIGINS="$${ALLOWED_ORIGINS:-http://localhost:3000}" go run ./apps/api/cmd/api

bootstrap: db-up migrate-up generate-db db-fixture
