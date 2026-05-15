# ADR 002: Remove Startup Mutation

## Status
Accepted

## Decision
The API server no longer seeds or enriches catalog data during startup.

## Rationale
- Read paths should be deterministic and fast.
- Startup-side mutation made deployment behavior depend on side effects and external providers.
- Explicit ingestion jobs are easier to observe, retry, and explain.

## Consequences
- `api/cmd/ingest` becomes the only catalog mutation path.
- Containers and local environments must ship or mount a prepared catalog.
