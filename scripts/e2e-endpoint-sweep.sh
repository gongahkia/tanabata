#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

IMAGE="${IMAGE:-tanabata-api:e2e-sweep}"
CONTAINER="${CONTAINER:-tanabata-api-e2e-sweep}"
PORT="${PORT:-18080}"
BASE_URL="http://127.0.0.1:${PORT}"
SPEC="openapi/openapi.json"
WEBHOOK_TOKEN="e2e-sweep-token"
WEBHOOK_ID=""

cleanup() {
  docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

wait_for_ready() {
  for _ in $(seq 1 60); do
    if curl -fsS "${BASE_URL}/readyz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

first_data_field() {
  local path="$1"
  local filter="$2"
  curl -fsS "${BASE_URL}${path}" | jq -r "${filter}" | head -n 1
}

status_allowed() {
  local status="$1"
  local allowed="$2"
  if [[ "${status}" =~ ^2[0-9][0-9]$ ]]; then
    return 0
  fi
  [[ " ${allowed} " == *" ${status} "* ]]
}

request_operation() {
  local method="$1"
  local path="$2"
  local url_path="${path}"
  local allowed=""
  local payload=""
  local body
  local status
  local -a args

  case "${method} ${path}" in
    "GET /v1/artists/{artist_id}"|\
    "GET /v1/artists/{artist_id}/quotes"|\
    "GET /v1/artists/{artist_id}/provenance/summary"|\
    "GET /v1/artists/{artist_id}/related"|\
    "GET /v1/artists/{artist_id}/releases"|\
    "GET /v1/artists/{artist_id}/recordings"|\
    "GET /v1/artists/{artist_id}/performances"|\
    "GET /v1/artists/{artist_id}/performances/stats")
      url_path="${path//\{artist_id\}/${ARTIST_ID}}"
      ;;
    "GET /v1/artists/{artist_id}/setlists")
      url_path="${path//\{artist_id\}/${ARTIST_ID}}"
      allowed="400 404 503"
      ;;
    "GET /v1/quotes/{quote_id}"|\
    "GET /v1/quotes/{quote_id}/similar"|\
    "GET /v1/quotes/{quote_id}/provenance"|\
    "GET /v1/quotes/{quote_id}/lineage")
      url_path="${path//\{quote_id\}/${QUOTE_ID}}"
      ;;
    "GET /v1/sources/{source_id}")
      url_path="${path//\{source_id\}/${SOURCE_ID}}"
      allowed="404"
      ;;
    "GET /v1/providers/{provider}/runs"|"GET /v1/providers/{provider}/errors")
      url_path="${path//\{provider\}/${PROVIDER}}"
      ;;
    "GET /v1/jobs/{job_id}"|\
    "GET /v1/jobs/{job_id}/snapshots"|\
    "GET /v1/jobs/{job_id}/audit")
      url_path="${path//\{job_id\}/${JOB_ID}}"
      allowed="404"
      ;;
    "GET /v1/works/{work_id}"|\
    "GET /v1/works/{work_id}/recordings"|\
    "GET /v1/works/{work_id}/credits"|\
    "GET /v1/works/{work_id}/performances")
      url_path="${path//\{work_id\}/${WORK_ID}}"
      allowed="404"
      ;;
    "GET /v1/recordings/{recording_id}"|\
    "GET /v1/recordings/{recording_id}/samples"|\
    "GET /v1/recordings/{recording_id}/sampled_by")
      url_path="${path//\{recording_id\}/${RECORDING_ID}}"
      allowed="404"
      ;;
    "GET /v1/samples/{sample_id}")
      url_path="${path//\{sample_id\}/${SAMPLE_ID}}"
      allowed="404"
      ;;
    "GET /v1/performances/{performance_id}")
      url_path="${path//\{performance_id\}/${PERFORMANCE_ID}}"
      allowed="404"
      ;;
    "GET /v1/claims/{claim_id}")
      url_path="${path//\{claim_id\}/${CLAIM_ID}}"
      allowed="404"
      ;;
    "GET /v1/graph/{entity_id}")
      url_path="${path//\{entity_id\}/${ARTIST_ID}}"
      ;;
    "GET /v1/search")
      url_path="${path}?q=frank"
      ;;
    "GET /v1/entities/search")
      url_path="${path}?q=frank"
      ;;
    "GET /v1/lyrics")
      url_path="${path}?artist=Coldplay&track=Yellow&provider=auto"
      allowed="400 404 429 502 503"
      ;;
    "GET /v1/webhooks")
      ;;
    "POST /v1/webhooks")
      payload='{"url":"http://127.0.0.1:65535","event_kinds":["job.completed"]}'
      ;;
    "DELETE /v1/webhooks/{webhook_id}")
      url_path="${path//\{webhook_id\}/${WEBHOOK_ID}}"
      ;;
  esac

  body="$(mktemp)"
  args=(-sS --connect-timeout 5 --max-time 30 -o "${body}" -w '%{http_code}' -X "${method}")
  if [[ "${path}" == /v1/webhooks* ]]; then
    args+=(-H "Authorization: Bearer ${WEBHOOK_TOKEN}")
  fi
  if [[ -n "${payload}" ]]; then
    args+=(-H 'Content-Type: application/json' --data "${payload}")
  fi
  status="$(curl "${args[@]}" "${BASE_URL}${url_path}" || true)"
  status="${status:-000}"
  if ! status_allowed "${status}" "${allowed}"; then
    printf 'endpoint failed: %s %s status=%s\n' "${method}" "${url_path}" "${status}" >&2
    head -c 500 "${body}" >&2 || true
    printf '\n' >&2
    rm -f "${body}"
    exit 1
  fi
  if [[ "${method} ${path}" == "POST /v1/webhooks" ]]; then
    WEBHOOK_ID="$(jq -r '.data.id // empty' "${body}")"
    if [[ -z "${WEBHOOK_ID}" ]]; then
      printf 'endpoint failed: POST /v1/webhooks missing subscription id\n' >&2
      head -c 500 "${body}" >&2 || true
      printf '\n' >&2
      rm -f "${body}"
      exit 1
    fi
  fi
  printf '%s %s -> %s\n' "${method}" "${url_path}" "${status}"
  rm -f "${body}"
}

docker build -f api/Dockerfile -t "${IMAGE}" .
docker run -d --name "${CONTAINER}" -p "${PORT}:8080" -e "TANABATA_WEBHOOK_ADMIN_TOKEN=${WEBHOOK_TOKEN}" -e TANABATA_RATE_LIMIT_RPM=0 "${IMAGE}" >/dev/null
wait_for_ready

ARTIST_ID="$(first_data_field '/v1/artists?limit=1' '.data[0].artist_id // empty')"
QUOTE_ID="$(first_data_field '/v1/quotes?limit=1' '.data[0].quote_id // empty')"
SOURCE_ID="$(first_data_field '/v1/quotes?limit=100' '.data[]? | select(.source_id != null and .source_id != "") | .source_id')"
PROVIDER="$(first_data_field '/v1/providers' '.data[0].provider // empty')"
JOB_ID="$(first_data_field '/v1/jobs?limit=1' '.data[0].job_id // empty')"
WORK_ID="$(first_data_field '/v1/works?limit=1' '.data[0].work_id // empty')"
RECORDING_ID="$(first_data_field '/v1/recordings?limit=1' '.data[0].recording_id // empty')"
PERFORMANCE_ID="$(first_data_field "/v1/artists/${ARTIST_ID}/performances?limit=1" '.data[0].performance_id // empty')"
CLAIM_ID="$(first_data_field '/v1/claims?limit=1' '.data[0].claim_id // empty')"
SAMPLE_ID=""

[[ -n "${ARTIST_ID}" ]] || { printf 'missing seeded artist id\n' >&2; exit 1; }
[[ -n "${QUOTE_ID}" ]] || { printf 'missing seeded quote id\n' >&2; exit 1; }
PROVIDER="${PROVIDER:-wikiquote}"
SOURCE_ID="${SOURCE_ID:-missing-source}"
JOB_ID="${JOB_ID:-missing-job}"
WORK_ID="${WORK_ID:-missing-work}"
RECORDING_ID="${RECORDING_ID:-missing-recording}"
PERFORMANCE_ID="${PERFORMANCE_ID:-missing-performance}"
CLAIM_ID="${CLAIM_ID:-missing-claim}"
SAMPLE_ID="${SAMPLE_ID:-missing-sample}"

while IFS=$'\t' read -r method path; do
  request_operation "${method}" "${path}"
done < <(jq -r '.paths | to_entries[] | .key as $path | .value | to_entries[] | select(.key == "get" or .key == "post" or .key == "delete" or .key == "put" or .key == "patch") | "\(.key | ascii_upcase)\t\($path)"' "${SPEC}")
