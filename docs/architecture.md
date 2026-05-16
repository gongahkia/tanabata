# Tanabata Architecture

## Serving Path
- `api/main.go` starts a read-only Gin server over a prebuilt SQLite catalog.
- `/livez` and `/readyz` expose runtime health; `/health` returns richer catalog metadata.
- `/v1` uses consistent response envelopes and request middleware for request IDs, structured logs, recovery, CORS, metrics, and tracing.

## Ingestion Path
- `api/cmd/ingest` is the explicit mutation entrypoint.
- `-bootstrap` seeds the catalog from `api/data/quotes.json`.
- `-all` and `-artist` enrich artists through MusicBrainz, Wikidata, Wikiquote, and Last.fm.
- Every ingestion run persists `jobs`, `job_items`, ingestion snapshots, and audit events so the API can expose pipeline history.

## Storage and Search
- SQLite remains the single runtime store for artists, quotes, sources, releases, provider bookkeeping, cache entries, and ingestion jobs.
- FTS5-backed `artist_search` and `quote_search` tables support `/v1/search` and query filtering.
- Search indices are rebuilt during ingestion and synced on artist or quote upserts.
- Provider cache entries back `/v1/lyrics` and `/v1/artists/{id}/setlists` to reduce noisy upstream traffic.

## Provider Contracts
- Provider adapters are isolated in `api/internal/providers`.
- Each provider uses a shared HTTP client with deterministic timeout, retry, and rate-limit settings.
- Successful and failed calls are recorded in `provider_runs` and `provider_errors`.
- Quote provenance is persisted as status, confidence, provider origin, source metadata, and human-readable evidence.
- Operational status is exposed through backend-only endpoints including `/v1/providers`, `/v1/timeline`, `/v1/jobs/{job_id}/snapshots`, and `/v1/jobs/{job_id}/audit`.

## Failure Handling
- Serving never mutates the catalog on startup.
- Upstream provider failures are recorded and downgraded into partial enrichment where possible instead of crashing the pipeline.
- CI validates backend tests, coverage floors, linting, and a container smoke test.
