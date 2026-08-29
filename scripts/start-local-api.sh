#!/usr/bin/env bash

set -Eeuo pipefail

: "${DATABASE_URL:?DATABASE_URL is required}"

export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING="${DATABASE_URL}"

./bin/goose -dir ./db/migrations up
exec ./bin/bridgeyok-api
