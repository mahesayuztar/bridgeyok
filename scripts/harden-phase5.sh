#!/usr/bin/env bash
set -Eeuo pipefail
repositoryRoot="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repositoryRoot}"
hardeningMode="${1:-all}"
if [[ "${hardeningMode}" != all && "${hardeningMode}" != process ]]; then
  echo "Usage: $0 [all|process]" >&2
  exit 2
fi
hardeningDir="$(mktemp -d)"
hardeningContainer="bridgeyok-hardening-$(date +%s)-$$"
cleanup() {
  docker rm -f "${hardeningContainer}" >/dev/null 2>&1 || true
  rm -rf "${hardeningDir}"
}
trap cleanup EXIT
hardeningPassword="$(openssl rand -hex 24)"
hardeningPort="$(python3 -c 'import socket; probe=socket.socket(); probe.bind(("127.0.0.1",0)); print(probe.getsockname()[1]); probe.close()')"
docker run -d --name "${hardeningContainer}" -e "POSTGRES_PASSWORD=${hardeningPassword}" -e POSTGRES_DB=phase5 -p "127.0.0.1:${hardeningPort}:5432" postgres:17 >/dev/null
for _attempt in {1..60}; do
  if docker exec "${hardeningContainer}" pg_isready -U postgres >/dev/null 2>&1; then break; fi
  sleep 1
done
export TEST_DATABASE_URL="postgres://postgres:${hardeningPassword}@127.0.0.1:${hardeningPort}/phase5?sslmode=disable"
DATABASE_URL="${TEST_DATABASE_URL}" make migrate-up
export PHASE5_TEST_API_BINARY="${hardeningDir}/api"
export PHASE5_DATABASE_CONTAINER="${hardeningContainer}"
go build -race -o "${PHASE5_TEST_API_BINARY}" ./apps/api/cmd/api
export PHASE5_ROLLBACK_BINARY="${hardeningDir}/api-rollback"
mkdir "${hardeningDir}/rollback-source"
hardeningRollbackCommit="$(git rev-parse --verify "${PHASE5_ROLLBACK_REF:-HEAD}^{commit}")"
git archive "${hardeningRollbackCommit}" | tar -x -C "${hardeningDir}/rollback-source"
(cd "${hardeningDir}/rollback-source" && go build -race -o "${PHASE5_ROLLBACK_BINARY}" ./apps/api/cmd/api)
echo "Rollback baseline: ${hardeningRollbackCommit}"
if [[ "${hardeningMode}" == all ]]; then
  go test -race -tags=integration ./apps/api/internal/database -run '^TestMatchLobbyCapacityRollbackAndDiscovery$' -count=1 -v
fi
go test -race -tags=integration ./apps/api/internal/httpapi -run '^TestMatchProcessHardening$' -count=1 -v -timeout=6m
