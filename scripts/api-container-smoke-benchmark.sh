#!/usr/bin/env bash
set -euo pipefail

IMAGE="${IMAGE:-tanabata-api:smoke}"
CONTAINER="${CONTAINER:-tanabata-api-smoke}"
PORT="${PORT:-8080}"
BASE_URL="http://127.0.0.1:${PORT}"
VERSION="${VERSION:-$(git describe --tags --always --dirty)}"
COMMIT="${COMMIT:-$(git rev-parse --short=12 HEAD)}"
REVISION="${REVISION:-${COMMIT}}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%FT%TZ)}"

cleanup() {
  docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

wait_for_container_healthy() {
  for _ in $(seq 1 60); do
    if [ "$(docker inspect --format '{{.State.Health.Status}}' "${CONTAINER}" 2>/dev/null || true)" = "healthy" ]; then
      return 0
    fi
    sleep 1
  done
  return 1
}

docker build \
  --build-arg "VERSION=${VERSION}" \
  --build-arg "COMMIT=${COMMIT}" \
  --build-arg "REVISION=${REVISION}" \
  --build-arg "BUILD_DATE=${BUILD_DATE}" \
  -f api/Dockerfile \
  -t "${IMAGE}" .
docker run --rm "${IMAGE}" -version | grep -F "Tanabata ${VERSION} (${COMMIT}) built ${BUILD_DATE}" >/dev/null
docker inspect --format '{{ json .Config.Labels }}' "${IMAGE}" | grep -F "\"org.opencontainers.image.revision\":\"${REVISION}\"" >/dev/null
cleanup
docker run -d --name "${CONTAINER}" -p "${PORT}:8080" "${IMAGE}" >/dev/null

for _ in $(seq 1 30); do
  if curl -fsS "${BASE_URL}/readyz" >/dev/null; then
    break
  fi
  sleep 1
done

curl -fsS "${BASE_URL}/readyz" >/dev/null
wait_for_container_healthy

benchmark_endpoint() {
  local path="$1"
  local label="$2"
  local seconds
  seconds="$(curl -fsS -o /dev/null -w '%{time_total}' "${BASE_URL}${path}")"
  printf '%s %ss\n' "${label}" "${seconds}"
}

benchmark_endpoint "/livez" "livez"
benchmark_endpoint "/readyz" "readyz"
benchmark_endpoint "/v1/version" "version"
benchmark_endpoint "/v1/providers" "providers"
benchmark_endpoint "/v1/search?q=discipline" "search"

docker stop --time 5 "${CONTAINER}" >/dev/null
