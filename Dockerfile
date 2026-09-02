FROM golang:1.27.0-bookworm AS builder

WORKDIR /workspace

# Go workspace
COPY go.work go.work.sum ./

# Download dependency API terlebih dahulu agar layer bisa di-cache
COPY apps/api/go.mod apps/api/go.sum ./apps/api/
RUN cd apps/api && go mod download

# Source API
COPY apps/api ./apps/api

# Build binaries
RUN mkdir -p /out && \
    CGO_ENABLED=0 go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/bridgeyok-api \
        ./apps/api/cmd/api && \
    CGO_ENABLED=0 go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/wsprobe \
        ./apps/api/cmd/wsprobe && \
    CGO_ENABLED=0 go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/wscheck \
        ./apps/api/cmd/wscheck && \
    CGO_ENABLED=0 GOBIN=/out go install \
        -trimpath \
        -ldflags="-s -w" \
        -tags="no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb" \
        github.com/pressly/goose/v3/cmd/goose@v3.27.3


FROM debian:bookworm-slim AS runtime

ARG RELEASE_ID=back4app

LABEL io.bridgeyok.release="${RELEASE_ID}"

RUN apt-get update && \
    apt-get install --yes --no-install-recommends \
        ca-certificates \
        curl && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd --system bridgeyok && \
    useradd \
        --system \
        --gid bridgeyok \
        --home-dir /workspace \
        bridgeyok && \
    mkdir -p \
        /workspace/bin \
        /workspace/db/migrations \
        /workspace/scripts && \
    chown -R bridgeyok:bridgeyok /workspace

WORKDIR /workspace

COPY --from=builder --chown=bridgeyok:bridgeyok \
    /out/bridgeyok-api ./bin/bridgeyok-api

COPY --from=builder --chown=bridgeyok:bridgeyok \
    /out/wsprobe ./bin/wsprobe

COPY --from=builder --chown=bridgeyok:bridgeyok \
    /out/wscheck ./bin/wscheck

COPY --from=builder --chown=bridgeyok:bridgeyok \
    /out/goose ./bin/goose

COPY --chown=bridgeyok:bridgeyok \
    db/migrations ./db/migrations

COPY --chown=bridgeyok:bridgeyok \
    scripts/start-local-api.sh ./scripts/start-api.sh

RUN chmod +x ./scripts/start-api.sh

USER bridgeyok

EXPOSE 8080

HEALTHCHECK \
    --interval=10s \
    --timeout=3s \
    --start-period=20s \
    --retries=3 \
    CMD ["curl", "--fail", "--silent", "--show-error", \
         "http://127.0.0.1:8080/health/ready"]

ENTRYPOINT ["./scripts/start-api.sh"]
