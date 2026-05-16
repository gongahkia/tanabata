

# `Tanabata`

A read-only REST API for music quotes, provenance, provider health, and ingestion history.

Built with Go, SQLite, FTS5, OpenAPI, Prometheus, and OpenTelemetry.

## Usage

### `GET` `/v1/search?q=frank`

```json
{
  "data": {
    "artists": [
      {
        "artist_id": "tanabata:frank-ocean",
        "name": "Frank Ocean"
      }
    ],
    "quotes": [
      {
        "quote_id": "d37edeaab5a095648aec95beb9944482",
        "text": "Work hard in silence.",
        "artist_name": "Frank Ocean",
        "provenance_status": "verified",
        "confidence_score": 0.99
      }
    ]
  }
}
```

### `GET` `/v1/quotes/{quote_id}/provenance`

```json
{
  "data": {
    "quote_id": "d37edeaab5a095648aec95beb9944482",
    "provenance_status": "verified",
    "confidence_score": 0.99,
    "provider_origin": "tanabata_curated",
    "evidence": [
      "Curated Tanabata editorial note matched to a maintained archive entry."
    ],
    "source": {
      "provider": "editorial_archive",
      "url": "https://archive.tanabata.dev/frank-ocean/studio-notes"
    }
  }
}
```

### `GET` `/v1/providers`

```json
{
  "data": [
    {
      "provider": "wikiquote",
      "category": "enrichment",
      "enabled": true,
      "last_status": "success",
      "recent_error_count": 0
    }
  ]
}
```

### `GET` `/v1/timeline`

```json
{
  "data": [
    {
      "event_id": "golden-job",
      "kind": "job",
      "title": "catalog-refresh",
      "status": "succeeded",
      "at": "2026-05-16T00:00:00Z"
    }
  ]
}
```

> [!NOTE]
> The main API surface lives under `/v1`. Legacy `/quotes`, `/quotes/random`, and `/quotes/{author}` routes are kept for compatibility only.

## API

- `GET /v1/artists`
- `GET /v1/quotes`
- `GET /v1/search`
- `GET /v1/providers`
- `GET /v1/jobs`
- `GET /v1/jobs/{job_id}?include=audit,snapshots`
- `GET /v1/review/queue`
- `GET /v1/review/stale`
- `GET /v1/stats`
- `GET /v1/integrity`
- `GET /livez`, `GET /readyz`, `GET /health`, `GET /metrics`

OpenAPI: [`openapi/openapi.json`](openapi/openapi.json)

## Development

```shell
make test
make ingest
make run
```

## Docker

```shell
docker compose up --build
```

```shell
./scripts/api-container-smoke-benchmark.sh
./scripts/compose-smoke.sh
```

## Operations

- Architecture: [`docs/architecture.md`](docs/architecture.md)
- Runbook: [`docs/runbook.md`](docs/runbook.md)
- Quality model: [`docs/quality-model.md`](docs/quality-model.md)
- Benchmarks: [`docs/benchmarks.md`](docs/benchmarks.md)

## License

No license specified.
