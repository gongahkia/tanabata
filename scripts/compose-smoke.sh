#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:18080}"

cleanup() {
  docker compose down --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker compose up --build -d api

for _ in $(seq 1 45); do
  if curl -fsS "${BASE_URL}/readyz" >/dev/null; then
    break
  fi
  sleep 1
done

curl -fsS "${BASE_URL}/readyz" >/dev/null
curl -fsS "${BASE_URL}/v1/search?q=frank&limit=2" >/dev/null
curl -fsS "${BASE_URL}/v1/providers" >/dev/null
curl -fsS "${BASE_URL}/v1/timeline?limit=5" >/dev/null
curl -fsS "${BASE_URL}/metrics" >/dev/null

printf 'compose smoke ok: %s\n' "${BASE_URL}"
