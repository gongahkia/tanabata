# ADR 003: Provenance Review Model

## Status
Accepted

## Decision
Quote provenance is expressed as status, confidence, provider origin, source metadata, and stored evidence.

## Status Values
- `verified`
- `source_attributed`
- `provider_attributed`
- `ambiguous`
- `needs_review`

## Rationale
- A single opaque confidence score is weak product communication.
- The portfolio story is stronger when quote quality and attribution are inspectable.

## Consequences
- `/v1/quotes/{quote_id}/provenance` is a first-class endpoint.
- Legacy imports default to `needs_review`.
