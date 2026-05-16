#!/usr/bin/env bash
set -euo pipefail

IMAGE="${IMAGE:-tanabata-api:smoke}"
CONTAINER="${CONTAINER:-tanabata-api-smoke}"
PORT="${PORT:-8080}"
BASE_URL="http://127.0.0.1:${PORT}"

cleanup() {
  docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker build -t "${IMAGE}" ./api
cleanup
docker run -d --name "${CONTAINER}" -p "${PORT}:8080" "${IMAGE}" >/dev/null

for _ in $(seq 1 30); do
  if curl -fsS "${BASE_URL}/readyz" >/dev/null; then
    break
  fi
  sleep 1
done

curl -fsS "${BASE_URL}/readyz" >/dev/null

benchmark_endpoint() {
  local path="$1"
  local label="$2"
  local seconds
  seconds="$(curl -fsS -o /dev/null -w '%{time_total}' "${BASE_URL}${path}")"
  printf '%s %ss\n' "${label}" "${seconds}"
}

benchmark_endpoint "/livez" "livez"
benchmark_endpoint "/readyz" "readyz"
benchmark_endpoint "/v1/providers" "providers"
benchmark_endpoint "/v1/search?q=discipline" "search"
