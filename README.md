# Tanabata

Tanabata is a read-only music knowledge product built around quote discovery, provenance, provider health, and ingestion visibility.

It is deliberately shaped as a backend-focused portfolio project:
- a Go API over a SQLite catalog
- an explicit ingestion pipeline instead of startup mutation
- FTS-backed catalog search
- provider run and error tracking
- OpenAPI contracts for every `/v1` backend surface

## Product Surface

### API
- Legacy endpoints remain for compatibility:
  - `/quotes`
  - `/quotes/random`
  - `/quotes/{author}`
- The main product surface lives under `/v1`:
  - `/v1/search`
  - `/v1/artists`
  - `/v1/quotes`
  - `/v1/quotes/{quote_id}/provenance`
  - `/v1/providers`
  - `/v1/providers/{provider}/runs`
  - `/v1/providers/{provider}/errors`
  - `/v1/jobs`
  - `/v1/jobs/{job_id}/snapshots`
  - `/v1/jobs/{job_id}/audit`
  - `/v1/timeline`
  - `/v1/review/queue`
  - `/v1/review/stale`
  - `/v1/stats`
  - `/v1/integrity`
  - `/v1/lyrics`

### Backend Demo Surfaces
- Quote provenance:
  `/v1/quotes/{quote_id}/provenance`
- Provider and system status:
  `/v1/providers`, `/v1/stats`, `/v1/integrity`, `/health`, `/metrics`
- Ingestion timeline:
  `/v1/timeline`, `/v1/jobs`, `/v1/jobs/{job_id}/snapshots`, `/v1/jobs/{job_id}/audit`

## Architecture

- Serving path:
  The API starts from a prebuilt catalog and does not mutate data on startup.
- Ingestion path:
  `api/cmd/ingest` seeds and enriches the catalog as tracked jobs.
- Search path:
  SQLite FTS5 indices back artist and quote search.
- Provider path:
  Enrichment and runtime providers record runs and failures, and runtime calls can be cached.

More detail lives in [docs/architecture.md](/Users/gongahkia/Desktop/coding/projects/tanabata/docs/architecture.md) and the ADRs under [docs/adr](/Users/gongahkia/Desktop/coding/projects/tanabata/docs/adr).

## Local Development

### Backend
```bash
make test
make ingest
make run
```

### Docker Compose
```bash
docker compose up --build
```

The compose setup expects the catalog to live in `api/data/catalog.sqlite`. Run `make ingest` first if you need to rebuild it locally.

## Contracts

- OpenAPI source of truth:
  [openapi/openapi.json](/Users/gongahkia/Desktop/coding/projects/tanabata/openapi/openapi.json)

## Observability and Ops

- `/livez`, `/readyz`, `/health`, `/metrics`
- request IDs and structured logs
- Prometheus metrics
- OpenTelemetry spans via stdout exporter
- multi-stage API container build
- CI for backend tests, coverage floor, linting, and container smoke test

## Screens

The original architecture reference is available at [asset/reference/architecture.png](/Users/gongahkia/Desktop/coding/projects/tanabata/asset/reference/architecture.png).
