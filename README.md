# Tanabata

Tanabata is a read-only music knowledge product built around quote discovery, provenance, provider health, and ingestion visibility.

It is deliberately shaped as a backend-focused portfolio project:
- a Go API over a SQLite catalog
- an explicit ingestion pipeline instead of startup mutation
- FTS-backed catalog search
- provider run and error tracking
- a small TypeScript frontend driven by a generated client from OpenAPI

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
  - `/v1/stats`
  - `/v1/lyrics`

### Web App
- `/`
  Discovery and search
- `/artists/:artistId`
  Artist detail with quotes, releases, and related artists
- `/quotes/:quoteId`
  Quote detail with provenance
- `/system`
  Provider health, freshness, and ingestion history

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

### Frontend
```bash
cd web
npm install
npm run dev
```

### Docker Compose
```bash
docker compose up --build
```

The compose setup expects the catalog to live in `api/data/catalog.sqlite`. Run `make ingest` first if you need to rebuild it locally.

## Contracts

- OpenAPI source of truth:
  [openapi/openapi.json](/Users/gongahkia/Desktop/coding/projects/tanabata/openapi/openapi.json)
- Generated client:
  [web/src/generated/client.ts](/Users/gongahkia/Desktop/coding/projects/tanabata/web/src/generated/client.ts)

Regenerate the client with:

```bash
cd web
npm run generate:client
```

## Observability and Ops

- `/livez`, `/readyz`, `/health`, `/metrics`
- request IDs and structured logs
- Prometheus metrics
- OpenTelemetry spans via stdout exporter
- multi-stage API container build
- CI for backend tests, coverage floor, linting, generated-client drift, frontend test/build, and container smoke test

## Screens

The repo now contains both the backend and the demo frontend. The original architecture reference is still available at [asset/reference/architecture.png](/Users/gongahkia/Desktop/coding/projects/tanabata/asset/reference/architecture.png).
