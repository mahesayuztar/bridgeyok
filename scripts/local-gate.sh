#!/usr/bin/env bash

set -Eeuo pipefail

repositoryRoot="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repositoryRoot}"

gateAction="${1:-run}"
gateEnvFile="${LOCAL_GATE_ENV_FILE:-.env.local-gate}"
httpsPort="${LOCAL_GATE_HTTPS_PORT:-8443}"
webOrigin="https://bridgeyok.localhost:${httpsPort}"
apiOrigin="https://api.bridgeyok.localhost:${httpsPort}"
gateTempDir="$(mktemp -d)"
caFile="${gateTempDir}/root.crt"
headersFile="${gateTempDir}/headers"
bodyFile="${gateTempDir}/body"
stackTouched=false

composeForRelease() {
  local releaseId="$1"
  shift
  LOCAL_GATE_RELEASE="${releaseId}" docker compose --env-file "${gateEnvFile}" -f deploy/local/compose.yaml "$@"
}

finish() {
  local exitCode=$?
  if [[ ${exitCode} -ne 0 && "${stackTouched}" == "true" ]]; then
    composeForRelease "${activeRelease:-current}" logs --no-color --tail 200 api web wsprobe edge || true
  fi
  rm -rf "${gateTempDir}"
  exit "${exitCode}"
}

requireLocalTools() {
  local toolName
  for toolName in docker curl grep; do
    if ! command -v "${toolName}" >/dev/null 2>&1; then
      printf 'Required tool is unavailable: %s\n' "${toolName}" >&2
      return 1
    fi
  done
  docker compose version >/dev/null
}

waitForStack() {
  local releaseId="$1"
  for _attempt in {1..120}; do
    if composeForRelease "${releaseId}" cp edge:/data/caddy/pki/authorities/local/root.crt "${caFile}" >/dev/null 2>&1; then
      chmod 0644 "${caFile}"
      if curl --cacert "${caFile}" --resolve "api.bridgeyok.localhost:${httpsPort}:127.0.0.1" --connect-timeout 2 --max-time 5 --fail --silent "${apiOrigin}/health/ready" >"${bodyFile}" 2>/dev/null && \
        grep -q '"status":"ready"' "${bodyFile}"; then
        return 0
      fi
    fi
    sleep 1
  done
  printf 'Local stack did not become ready within 120 seconds.\n' >&2
  return 1
}

runWebSocketCheck() {
  local releaseId="$1"
  local mode="$2"
  local origin="$3"

  composeForRelease "${releaseId}" run --rm --no-deps \
    --volume "${caFile}:/tmp/caddy-root.crt:ro" \
    -e WS_CHECK_CA_FILE=/tmp/caddy-root.crt \
    -e "WS_CHECK_MODE=${mode}" \
    -e "WS_CHECK_ORIGIN=${origin}" \
    wscheck
}

assertRelease() {
  local releaseId="$1"
  local serviceName
  local containerId
  local actualRelease
  for serviceName in api web wsprobe; do
    containerId="$(composeForRelease "${releaseId}" ps -q "${serviceName}")"
    if [[ -z "${containerId}" ]]; then
      printf 'Service has no running container: %s\n' "${serviceName}" >&2
      return 1
    fi
    actualRelease="$(docker inspect --format '{{ index .Config.Labels "io.bridgeyok.release" }}' "${containerId}")"
    if [[ "${actualRelease}" != "${releaseId}" ]]; then
      printf 'Service %s runs release %s, expected %s.\n' "${serviceName}" "${actualRelease}" "${releaseId}" >&2
      return 1
    fi
  done
}

smokeRelease() {
  local releaseId="$1"
  local invalidStatus

  waitForStack "${releaseId}"
  assertRelease "${releaseId}"

  curl --cacert "${caFile}" --resolve "api.bridgeyok.localhost:${httpsPort}:127.0.0.1" --fail --silent --show-error "${apiOrigin}/health/live" | grep -q '"status":"ok"'
  curl --cacert "${caFile}" --resolve "api.bridgeyok.localhost:${httpsPort}:127.0.0.1" --fail --silent --show-error --dump-header "${headersFile}" --output "${bodyFile}" --header "Origin: ${webOrigin}" "${apiOrigin}/health/live"
  tr -d '\r' <"${headersFile}" | grep -Fiqx "Access-Control-Allow-Origin: ${webOrigin}"

  invalidStatus="$(curl --cacert "${caFile}" --resolve "api.bridgeyok.localhost:${httpsPort}:127.0.0.1" --silent --show-error --output /dev/null --write-out '%{http_code}' --header 'Origin: https://attacker.example' "${apiOrigin}/health/live")"
  if [[ "${invalidStatus}" != "403" ]]; then
    printf 'Invalid Origin returned HTTP %s, expected 403.\n' "${invalidStatus}" >&2
    return 1
  fi

  curl --cacert "${caFile}" --resolve "bridgeyok.localhost:${httpsPort}:127.0.0.1" --fail --silent --show-error "${webOrigin}" >"${bodyFile}"
  grep -q 'BridgeYok' "${bodyFile}"
  grep -q 'Fondasi siap' "${bodyFile}"

  runWebSocketCheck "${releaseId}" echo "${webOrigin}"
  runWebSocketCheck "${releaseId}" rejected-origin https://attacker.example
  runWebSocketCheck "${releaseId}" oversized "${webOrigin}"
}

deployRelease() {
  local releaseId="$1"
  activeRelease="${releaseId}"
  composeForRelease "${releaseId}" up --detach --force-recreate --remove-orphans api wsprobe web edge
  stackTouched=true
  smokeRelease "${releaseId}"
}

trap finish EXIT

requireLocalTools
if [[ ! -f "${gateEnvFile}" ]]; then
  printf 'Create %s from deploy/local/gate.env.example and set the Supabase session-pooler DATABASE_URL.\n' "${gateEnvFile}" >&2
  exit 1
fi

if [[ "${gateAction}" == "down" ]]; then
  composeForRelease current down --remove-orphans
  printf 'Local gate stack stopped. Supabase data and the local Caddy CA were preserved.\n'
  exit 0
fi
if [[ "${gateAction}" != "run" ]]; then
  printf 'Usage: %s [run|down]\n' "$0" >&2
  exit 1
fi

composeForRelease validation config --quiet

runID="$(date -u +%Y%m%d%H%M%S)-$$"
baselineRelease="baseline-${runID}"
candidateRelease="candidate-${runID}"
activeRelease="${candidateRelease}"

printf 'Building local candidate release %s...\n' "${candidateRelease}"
composeForRelease "${candidateRelease}" build api web
docker image tag "bridgeyok-api:${candidateRelease}" "bridgeyok-api:${baselineRelease}"
docker image tag "bridgeyok-web:${candidateRelease}" "bridgeyok-web:${baselineRelease}"

printf 'Starting known-good baseline %s...\n' "${baselineRelease}"
deployRelease "${baselineRelease}"

printf 'Deploying candidate %s...\n' "${candidateRelease}"
deployRelease "${candidateRelease}"

printf 'Rolling back to %s...\n' "${baselineRelease}"
deployRelease "${baselineRelease}"

printf 'Promoting verified candidate %s...\n' "${candidateRelease}"
deployRelease "${candidateRelease}"

printf 'Local Phase 1 gate passed.\n'
printf 'Web: %s\n' "${webOrigin}"
printf 'API: %s\n' "${apiOrigin}"
