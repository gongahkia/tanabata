[![](https://img.shields.io/badge/tanabata_1.0.0-passing-green)](https://github.com/gongahkia/tanabata/releases/tag/1.0.0) 
![](https://github.com/gongahkia/tanabata/actions/workflows/scrape.yml/badge.svg)
![](https://img.shields.io/badge/tanabata_1.0.0-deployment_down-orange)

> [!WARNING]  
> [`Tanabata`](https://github.com/gongahkia/tanabata)'s Render deployment is inactive as of 16 May 2026.  

# `Tanabata`

Small read-only [REST API](#api) for musician quotes with [built-in provenance & ingestion history](#architecture).

## Stack

* *Backend*: [Go](https://go.dev/), [OpenAPI](https://www.openapis.org/)
* *DB*: [SQLite](https://www.sqlite.org/), [FTS5](https://www.sqlite.org/fts5.html)
* *Observability*: [Prometheus](https://prometheus.io/), [OpenTelemetry](https://opentelemetry.io/)
* *Package*: [Docker](https://www.docker.com/)

## Usage

The below instructions are for locally running `Tanabata`.

1. First clone the repo locally on your machine.

```console
$ git clone https://github.com/gongahkia/tanabata
```

2. Then run `Tanabata` with any of the below.

### make

```console
$ make test
$ make ingest
$ make run
```

### docker

```console
$ docker compose up --build
```

### shell

```console
$ ./scripts/api-container-smoke-benchmark.sh
$ ./scripts/compose-smoke.sh
```

## API

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

## API

See [`openapi/openapi.json`](openapi/openapi.json) for more details.

* `GET /v1/artists`
* `GET /v1/quotes`
* `GET /v1/search`
* `GET /v1/providers`
* `GET /v1/jobs`
* `GET /v1/jobs/{job_id}?include=audit,snapshots`
* `GET /v1/review/queue`
* `GET /v1/review/stale`
* `GET /v1/stats`
* `GET /v1/integrity`
* `GET /livez`, `GET /readyz`, `GET /health`, `GET /metrics`

## Architecture

![](./asset/reference/architecture.png)

## Other notes

`Tanabata` is heavily inspired by [`kanye.rest`](https://github.com/ajzbc/kanye.rest).

## Reference

The name `Tanabata` is in reference to [Tanabata](https://sakamoto-days.fandom.com/wiki/Tanabata) (七夕), a new member of the [Order](https://sakamoto-days.fandom.com/wiki/Order) recruited during the [JAA Jail Arc](https://sakamoto-days.fandom.com/wiki/JAA_Jail_Arc). He emerges as an antagonist in the [New JAA Arc](https://sakamoto-days.fandom.com/wiki/New_JAA_Arc) as part of the ongoing manga series [Sakamoto Days](https://sakamoto-days.fandom.com/wiki/Sakamoto_Days_Wiki).

![](./asset/logo/tanabata.webp)