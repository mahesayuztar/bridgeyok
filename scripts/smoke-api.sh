#!/usr/bin/env bash

set -Eeuo pipefail

smokePort="${API_SMOKE_PORT:-18080}"
smokeOrigin="${API_SMOKE_ORIGIN:-http://localhost:3000}"
smokeBaseUrl="http://127.0.0.1:${smokePort}"
smokeTempDir="$(mktemp -d)"
smokeBinary="${smokeTempDir}/bridgeyok-api"
smokeLog="${smokeTempDir}/api.log"
smokeHeaders="${smokeTempDir}/headers"
smokeBody="${smokeTempDir}/body"
smokePid=""

cleanup() {
  if [[ -n "${smokePid}" ]] && kill -0 "${smokePid}" 2>/dev/null; then
    kill -TERM "${smokePid}" 2>/dev/null || true
    wait "${smokePid}" 2>/dev/null || true
  fi
  rm -rf "${smokeTempDir}"
}

fail() {
  if [[ -f "${smokeLog}" ]]; then
    sed -n '1,200p' "${smokeLog}" >&2
  fi
  exit 1
}

trap cleanup EXIT

go build -o "${smokeBinary}" ./apps/api/cmd/api
APP_ENV=test \
API_HOST=127.0.0.1 \
PORT="${smokePort}" \
ALLOWED_ORIGINS="${smokeOrigin}" \
LOG_LEVEL=info \
"${smokeBinary}" >"${smokeLog}" 2>&1 &
smokePid=$!

for _attempt in {1..30}; do
  if curl --silent --fail "${smokeBaseUrl}/health/live" >"${smokeBody}"; then
    break
  fi
  if ! kill -0 "${smokePid}" 2>/dev/null; then
    fail
  fi
  sleep 1
done

grep -q '"status":"ok"' "${smokeBody}" || fail
curl --silent --fail "${smokeBaseUrl}/health/ready" | grep -q '"status":"ready"' || fail
curl --silent --fail --dump-header "${smokeHeaders}" --output "${smokeBody}" --header "Origin: ${smokeOrigin}" "${smokeBaseUrl}/health/live" || fail
grep -qi "^Access-Control-Allow-Origin: ${smokeOrigin}" "${smokeHeaders}" || fail
curl --silent --output "${smokeBody}" --write-out '%{http_code}' --header "Origin: https://invalid.example" "${smokeBaseUrl}/health/live" | grep -q '^403$' || fail

kill -TERM "${smokePid}"
wait "${smokePid}" || fail
smokePid=""
grep -q '"msg":"http server stopped"' "${smokeLog}" || fail
