#!/usr/bin/env bash

set -Eeuo pipefail

repositoryRoot="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repositoryRoot}"

mkdir -p bin
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/bridgeyok-api ./apps/api/cmd/api
CGO_ENABLED=0 GOBIN="${repositoryRoot}/bin" go install -trimpath -ldflags="-s -w" -tags="no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb" github.com/pressly/goose/v3/cmd/goose@v3.27.3
