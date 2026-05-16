# ADR 004: Quote Merge Heuristics

## Status
Accepted

## Decision
Tanabata merges duplicate and near-duplicate quotes during import instead of storing every provider variant as a separate quote.

## Context
Provider and curated sources often disagree on punctuation, casing, source richness, and confidence. Keeping each variant would inflate catalog counts and weaken search quality. Blind replacement would lose useful evidence.

## Heuristics
- Normalize quote text before identity and similarity checks.
- Treat same-artist high-similarity records as merge candidates.
- Prefer the candidate with stronger provenance rank.
- Use confidence score as the next tie-breaker.
- Prefer source-backed records over records with no source metadata.
- Preserve the richer text, source metadata, timestamps, tags, and evidence when they improve the merged record.
- Keep merged evidence visible so users can inspect why the final quote exists.

## Rationale
- Search results should show one strong quote, not three punctuation variants.
- Provenance should improve over time as stronger sources arrive.
- The ingestion pipeline needs deterministic behavior for tests and repeatable imports.

## Consequences
- The catalog favors quality and inspectability over raw provider cardinality.
- Tests must cover exact duplicates, near duplicates, conflicting provenance, and multi-source evidence preservation.
- Future admin tooling can expose merge decisions using the ingestion audit trail.
