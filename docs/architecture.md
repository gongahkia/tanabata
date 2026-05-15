# Tanabata Architecture

## Serving Path
- `api/main.go` starts a read-only Gin server over a prebuilt SQLite catalog.
- `/livez` and `/readyz` expose runtime health; `/health` returns richer catalog metadata.
- `/v1` uses consistent response envelopes and request middleware for request IDs, structured logs, recovery, CORS, metrics, and tracing.

## Ingestion Path
- `api/cmd/ingest` is the explicit mutation entrypoint.
- `-bootstrap` seeds the catalog from `api/data/quotes.json`.
- `-all` and `-artist` enrich artists through MusicBrainz, Wikidata, Wikiquote, and Last.fm.
- Every ingestion run persists a `jobs` record plus `job_items` so the system page and API can expose pipeline history.

## Storage and Search
- SQLite remains the single runtime store for artists, quotes, sources, releases, provider bookkeeping, cache entries, and ingestion jobs.
- FTS5-backed `artist_search` and `quote_search` tables support `/v1/search` and query filtering.
- Search indices are rebuilt on startup and synced on artist or quote upserts.
- Provider cache entries back `/v1/lyrics` and `/v1/artists/{id}/setlists` to reduce noisy upstream traffic.

## Provider Contracts
- Provider adapters are isolated in `api/internal/providers`.
- Each provider uses a shared HTTP client with deterministic timeout, retry, and rate-limit settings.
- Successful and failed calls are recorded in `provider_runs` and `provider_errors`.
- Quote provenance is persisted as status, confidence, provider origin, source metadata, and human-readable evidence.

## Failure Handling
- Serving never mutates the catalog on startup.
- Upstream provider failures are recorded and downgraded into partial enrichment where possible instead of crashing the pipeline.
- CI validates backend tests, generated client drift, frontend build/test, linting, and a container smoke test.
