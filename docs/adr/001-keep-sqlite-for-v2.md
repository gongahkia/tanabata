# ADR 001: Keep SQLite For V2

## Status
Accepted

## Decision
Tanabata V2 keeps SQLite as the primary runtime store.

## Rationale
- The project is read-heavy and single-service.
- SQLite is enough to demonstrate schema design, indexing, FTS, and operational discipline without adding infrastructure noise.
- It keeps local development, CI, and Docker smoke tests simple.

## Consequences
- Search depth is shown through FTS5 and query design instead of a separate search tier.
- Multi-writer scaling is not the goal for this phase.
