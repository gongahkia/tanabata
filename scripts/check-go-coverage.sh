#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
API_DIR="$ROOT_DIR/api"
TMP_OUTPUT="$(mktemp)"
trap 'rm -f "$TMP_OUTPUT"' EXIT

(
  cd "$API_DIR"
  go test ./... -cover
) | tee "$TMP_OUTPUT"

check_floor() {
  local pkg="$1"
  local floor="$2"
  local line
  line="$(grep "$pkg" "$TMP_OUTPUT" || true)"
  if [[ -z "$line" ]]; then
    echo "coverage line missing for $pkg" >&2
    exit 1
  fi
  local actual
  actual="$(echo "$line" | sed -E 's/.*coverage: ([0-9.]+)%.*/\1/')"
  awk -v actual="$actual" -v floor="$floor" 'BEGIN { exit !(actual + 0 >= floor + 0) }' || {
    echo "coverage for $pkg below floor: $actual% < $floor%" >&2
    exit 1
  }
}

check_floor "cmd/ingest" "50"
check_floor "internal/api" "50"
check_floor "internal/catalog" "60"
check_floor "internal/providers" "65"
