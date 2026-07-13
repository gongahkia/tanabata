[![](https://img.shields.io/badge/tanabata_2.0.0-passing-green)](https://github.com/gongahkia/tanabata/releases/tag/2.0.0)
![](https://github.com/gongahkia/tanabata/actions/workflows/scrape.yml/badge.svg)
![](https://github.com/gongahkia/tanabata/actions/workflows/ci.yml/badge.svg)

> [!IMPORTANT]
> `Tanabata` is now ***live*** at [tanabata.onrender.com/v1/quotes](https://tanabata.onrender.com/v1/quotes).

# `Tanabata` 🎶

Small read-only [REST API](#api) for musician quotes with [built-in provenance](#architecture), [webhooks](#webhook) and [native iframe embedding](#embed).

## Stack

* *Backend*: [Go](https://go.dev/), [OpenAPI](https://www.openapis.org/)
* *DB*: [SQLite](https://www.sqlite.org/), [FTS5](https://www.sqlite.org/fts5.html)
* *Observability*: [Prometheus](https://prometheus.io/), [OpenTelemetry](https://opentelemetry.io/)
* *Package*: [Docker](https://www.docker.com/)

## Usage

If you'd want to try it for yourself, the below instructions are for locally running `Tanabata`.

1. First clone the repo locally on your machine.

```console
$ git clone https://github.com/gongahkia/tanabata
```

2. Then build `Tanabata` with any of the below tools.

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

### Overview

OpenAPI documentation is available at [gabrielongzm.com/tanabata/api](https://gongahkia.github.io/tanabata/api/).

* `GET /v1/artists`
* `GET /v1/artists/{artist_id}/provenance/summary`
* `GET /v1/artists/{artist_id}/recordings`
* `GET /v1/artists/{artist_id}/performances`
* `GET /v1/artists/{artist_id}/performances/stats`
* `GET /v1/quotes`
* `GET /v1/quotes/{quote_id}/similar`
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
* `GET /v1/disputes.atom`
* `GET /v1/graph/{entity_id}`
* `GET /v1/webhooks`
* `GET /v1/entities/search`
* `GET /v1/search`
* `GET /v1/providers`
* `GET /v1/jobs`
* `GET /v1/jobs/{job_id}?include=audit,snapshots`
* `GET /v1/review/queue`
* `GET /v1/review/stale`
* `GET /v1/stats`
* `GET /v1/integrity`
* `GET /livez`, `GET /readyz`, `GET /health`, `GET /metrics`

### In detail

#### `GET` `/v1/search?q=frank`

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

#### `GET` `/v1/entities/search?q=hallelujah`

```json
{
  "data": {
    "hits": [
      {
        "kind": "recording",
        "id": "tanabata:rec:8f45a976f9adc9a9d734715b3c3e7692",
        "label": "Leonard Cohen - Hallelujah",
        "score": 2.4168597598099506,
        "snippet": "Hallelujah"
      },
      {
        "kind": "work",
        "id": "tanabata:work:a0b6305df23a2e8d78d13355de7e0779",
        "label": "Hallelujah",
        "score": 1.5934482758917823,
        "snippet": "Hallelujah"
      }
    ]
  }
}
```

#### `GET` `/v1/quotes/{quote_id}/provenance`

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

#### `GET` `/v1/providers`

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

#### `GET` `/v1/timeline`

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

#### `GET` `/v1/quotes/{quote_id}/lineage`

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

#### `GET` `/v1/recordings/{recording_id}/samples`

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

#### `GET` `/v1/works/{work_id}/recordings`

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

#### `GET` `/v1/disputes`

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

#### `GET` `/v1/artists/{artist_id}/performances/stats`

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

## Webhooks

`Tanabata` also provides [Webhooks](https://www.redhat.com/en/topics/automation/what-is-a-webhook).

1. Set `TANABATA_WEBHOOK_ADMIN_TOKEN` to enable admin-only webhook management.
2. Create subscriptions with `POST /v1/webhooks` and `Authorization: Bearer <token>` for the below.
    1. `claim.state_changed`
    2. `job.completed`
    3. `dispute.raised`
3. Deliveries are signed with `X-Tanabata-Signature: sha256=<hex>` using the per-subscription secret returned only on creation.
4. Failed deliveries retry and disable after **five failures** by default.

## Embed

`Tanabata` additionally allow native HTML embedding as an [iframe](https://www.w3schools.com/tags/tag_iframe.ASP).

1. Use `GET /embed/quote/{quote_id}?theme=light|dark` as an iframe source for a server-rendered quote card.
2. The response is static HTML/CSS that is cacheable for one hour and frameable from third-party pages.

## Architecture

![](./asset/reference/architecture.png)

## Other notes

`Tanabata` is heavily inspired by [`kanye.rest`](https://github.com/ajzbc/kanye.rest).

## Reference

The name `Tanabata` is in reference to [Tanabata](https://sakamoto-days.fandom.com/wiki/Tanabata) (七夕), a new member of the [Order](https://sakamoto-days.fandom.com/wiki/Order) recruited during the [JAA Jail Arc](https://sakamoto-days.fandom.com/wiki/JAA_Jail_Arc). He emerges as an antagonist in the [New JAA Arc](https://sakamoto-days.fandom.com/wiki/New_JAA_Arc) as part of the ongoing manga series [Sakamoto Days](https://sakamoto-days.fandom.com/wiki/Sakamoto_Days_Wiki).

<div align="centre">
    <img src="./asset/logo/tanabata.webp">
</div>
