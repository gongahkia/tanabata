[![](https://img.shields.io/badge/tanabata_1.0.0-passing-green)](https://github.com/gongahkia/tanabata/releases/tag/1.0.0)
![](https://github.com/gongahkia/tanabata/actions/workflows/scrape.yml/badge.svg)
![](https://github.com/gongahkia/tanabata/actions/workflows/ci.yml/badge.svg)
![](https://img.shields.io/badge/tanabata_1.0.0-deployment_down-orange)

# `Tanabata`

Small read-only [REST API](#api) for musician quotes with [built-in provenance, lineage & ingestion history](#architecture).

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

### `GET` `/v1/quotes/{quote_id}/lineage`

```json
{
  "data": {
    "quote_id": "tanabata:claim:hendrix-knowledge",
    "text": "Knowledge speaks, but wisdom listens.",
    "attributed_to_name": "Jimi Hendrix",
    "provenance_status": "ambiguous",
    "confidence_score": 0.35,
    "supporting_evidence": [
      {
        "excerpt": "The quote is reproduced on countless quote-aggregator sites attributed to Jimi Hendrix without a primary citation.",
        "evidence_kind": "aggregator_evidence",
        "weight": 0.3
      }
    ],
    "refuting_evidence": [
      {
        "excerpt": "No published interview, broadcast, or lyric in the Hendrix archive contains this phrasing.",
        "evidence_kind": "archival_negative",
        "weight": 0.9
      }
    ]
  }
}
```

### `GET` `/v1/recordings/{recording_id}/samples`

```json
{
  "data": [
    {
      "sample_id": "tanabata:sample:rappers-delight-good-times",
      "kind": "interpolation",
      "source_recording": {
        "artist_name": "Chic",
        "title": "Good Times",
        "released_year": "1979"
      },
      "derivative_recording": {
        "artist_name": "The Sugarhill Gang",
        "title": "Rapper's Delight",
        "released_year": "1979"
      },
      "claim": {
        "status": "verified",
        "confidence_score": 0.99,
        "supporting_evidence_count": 1
      }
    }
  ]
}
```

### `GET` `/v1/works/{work_id}/recordings`

```json
{
  "data": [
    {
      "recording_id": "tanabata:rec:hallelujah-cohen",
      "artist_name": "Leonard Cohen",
      "title": "Hallelujah",
      "released_year": "1984",
      "is_original": true
    },
    {
      "recording_id": "tanabata:rec:hallelujah-cale",
      "artist_name": "John Cale",
      "title": "Hallelujah",
      "released_year": "1991",
      "is_original": false
    },
    {
      "recording_id": "tanabata:rec:hallelujah-buckley",
      "artist_name": "Jeff Buckley",
      "title": "Hallelujah",
      "released_year": "1994",
      "is_original": false
    }
  ]
}
```

### `GET` `/v1/disputes`

```json
{
  "data": [
    {
      "claim": {
        "kind": "credit",
        "status": "ambiguous",
        "confidence_score": 0.6,
        "supporting_evidence_count": 1
      },
      "human_description": "Credit Richard Ashcroft (composer) on Bittersweet Symphony is ambiguous."
    },
    {
      "claim": {
        "kind": "sample",
        "status": "disputed",
        "confidence_score": 0.7
      },
      "human_description": "Sample claim from Marvin Gaye — Got to Give It Up to Robin Thicke — Blurred Lines is disputed."
    }
  ]
}
```

### `GET` `/v1/artists/{artist_id}/performances/stats`

```json
{
  "data": {
    "artist_id": "tanabata:radiohead",
    "work_id": "tanabata:work:creep",
    "work_title": "Creep",
    "total_performed": 2,
    "first_performed_at": "2016-05-26T00:00:00Z",
    "last_performed_at": "2016-07-08T00:00:00Z",
    "gap_days": 43,
    "average_gap_days": 43,
    "distinct_venues": 2,
    "distinct_countries": 2
  }
}
```

## API

See [`openapi/openapi.json`](openapi/openapi.json) for more details.

* `GET /v1/artists`
* `GET /v1/artists/{artist_id}/recordings`
* `GET /v1/artists/{artist_id}/performances`
* `GET /v1/artists/{artist_id}/performances/stats`
* `GET /v1/quotes`
* `GET /v1/quotes/{quote_id}/lineage`
* `GET /v1/works`
* `GET /v1/works/{work_id}/recordings`
* `GET /v1/works/{work_id}/credits`
* `GET /v1/works/{work_id}/performances`
* `GET /v1/recordings`
* `GET /v1/recordings/{recording_id}/samples`
* `GET /v1/recordings/{recording_id}/sampled_by`
* `GET /v1/samples/{sample_id}`
* `GET /v1/performances/{performance_id}`
* `GET /v1/claims`
* `GET /v1/claims/{claim_id}`
* `GET /v1/disputes`
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

<div align="centre">
    <img src="./asset/logo/tanabata.webp">
</div>
