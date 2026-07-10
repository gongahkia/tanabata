#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:18080}"

cleanup() {
  docker compose down --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker compose up --build -d api

wait_for_ready() {
  for _ in $(seq 1 45); do
    if curl -fsS "${BASE_URL}/readyz" >/dev/null; then
      return 0
    fi
    sleep 1
  done
  return 1
}

wait_for_healthy() {
  local container_id="$1"
  for _ in $(seq 1 45); do
    if [ "$(docker inspect --format '{{.State.Health.Status}}' "${container_id}" 2>/dev/null || true)" = "healthy" ]; then
      return 0
    fi
    sleep 1
  done
  return 1
}

wait_for_restart_ready() {
  local container_id="$1"
  for _ in $(seq 1 45); do
    if [ "$(docker inspect --format '{{.State.Running}}' "${container_id}" 2>/dev/null || true)" = "true" ] && curl -fsS "${BASE_URL}/readyz" >/dev/null; then
      return 0
    fi
    sleep 1
  done
  return 1
}

container_id="$(docker compose ps -q api)"
wait_for_ready
wait_for_healthy "${container_id}"

docker exec "${container_id}" sh -c 'kill -9 1' >/dev/null 2>&1 || true
wait_for_restart_ready "${container_id}"
wait_for_healthy "${container_id}"

curl -fsS "${BASE_URL}/readyz" >/dev/null
curl -fsS "${BASE_URL}/v1/search?q=frank&limit=2" >/dev/null
curl -fsS "${BASE_URL}/v1/providers" >/dev/null
curl -fsS "${BASE_URL}/v1/timeline?limit=5" >/dev/null
curl -fsS "${BASE_URL}/metrics" >/dev/null

printf 'compose smoke ok: %s\n' "${BASE_URL}"
