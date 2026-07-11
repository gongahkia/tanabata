# Tanabata Backend Runbook

## Health Checks

- Liveness: `GET /livez`
- Readiness: `GET /readyz`
- Metadata health: `GET /health`
- Metrics: `GET /metrics`
- Integrity: `GET /v1/integrity`

Use `/readyz` for container startup checks. Use `/v1/integrity` before publishing a rebuilt catalog.

## Ingestion Recovery

Run a full local rebuild:

```bash
make ingest
```

Run targeted enrichment:

```bash
make ingest-artist ARTIST="Frank Ocean"
```

Inspect job state:

```bash
curl -fsS http://127.0.0.1:8080/v1/jobs
curl -fsS "http://127.0.0.1:8080/v1/jobs/<job_id>?include=audit,snapshots"
curl -fsS http://127.0.0.1:8080/v1/timeline
```

If a job fails:

- Check `error_message` on the job and job item.
- Check `/v1/providers/{provider}/errors`.
- Re-run the same ingestion command; imports are idempotent and merge duplicate quote evidence.
- Run `/v1/integrity` after recovery.

## Ingestion Locking

`cmd/ingest` takes an exclusive POSIX lock at `<catalog>.ingest.lock` and writes the holder PID into that file. A second ingest process exits with `already_running` and code `2`.

If the lock file remains after a confirmed-dead process, inspect the PID first:

```bash
cat api/data/catalog.sqlite.ingest.lock
ps -p "$(cat api/data/catalog.sqlite.ingest.lock)"
```

After confirming the PID is stale, break the lock and continue:

```bash
cd api
go run ./cmd/ingest -catalog data/catalog.sqlite -force-unlock -bootstrap=true -all=true
```

## Provider Cooldowns

Provider failures are classified as:

- `timeout`
- `rate_limit`
- `parse_error`
- `not_found`
- `bad_upstream`
- `network`

Failures are visible at:

```bash
curl -fsS http://127.0.0.1:8080/v1/providers
curl -fsS http://127.0.0.1:8080/v1/providers/wikiquote/errors
```

Active cooldowns appear in provider summaries as `cooldown_until` and `cooldown_reason`.

## Backup And Export

Create a SQLite backup:

```bash
make catalog-backup
```

Export catalog stats and integrity metadata:

```bash
make catalog-export
```

The backup/export command can also be run directly:

```bash
cd api
go run ./cmd/catalog -catalog data/catalog.sqlite -backup data/catalog.backup.sqlite
go run ./cmd/catalog -catalog data/catalog.sqlite -export data/catalog.export.json
```

## Smoke Tests

Run the API container smoke benchmark:

```bash
./scripts/api-container-smoke-benchmark.sh
```

Run the compose smoke test:

```bash
./scripts/compose-smoke.sh
```

Both checks wait for readiness and verify core backend surfaces.

The base Compose file uses a named `catalog-data` volume at `/app/data` to avoid host/catalog write races. For local data hacking, use the dev override to restore the bind mount:

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build api
```

## Contract Checks

The OpenAPI spec is the backend contract source of truth:

```bash
openapi/openapi.json
```

Runtime validation can be enabled with:

```bash
TANABATA_CONTRACT_VALIDATION=true TANABATA_OPENAPI_SPEC=../openapi/openapi.json go run .
```
# Metrics

Prometheus exposes `tanabata_provider_request_duration_seconds`, `tanabata_provider_error_total`, `tanabata_ingest_job_duration_seconds`, `tanabata_claim_status_transition_total`, and `tanabata_catalog_row_count`. Alert on sustained provider errors, rising request latency, failed ingest jobs, unexpected claim-status churn, and material catalog-row drops.
