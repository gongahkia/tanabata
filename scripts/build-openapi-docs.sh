#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

mkdir -p docs/api api/internal/api/docs
npx --yes @redocly/cli@2.38.0 build-docs openapi/openapi.json --output docs/api/index.html --title "Tanabata API"
perl -pi -e 's/[ \t]+$//' docs/api/index.html
cp docs/api/index.html api/internal/api/docs/index.html
