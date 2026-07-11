# ADR 005: Unified Claim Model for Lineage Features

## Status
Accepted

## Context
Tanabata V2 expanded beyond quote attribution to cover four additional music-lineage feature families: misquoted quotes (debunking attribution), sampling lineage (recording → recording), songwriting credits and ghostwriting disputes, cover lineage (work → recordings), and performance history (artist played work at venue on date).

A naive implementation would have given each family its own provenance fields and dispute pipeline. That would have meant five copies of: a status enum, a confidence score, a provider origin, an evidence list, a refuting-evidence story, a dispute API, and five separate test surfaces. It would also have made cross-cutting features ("show me every disputed claim regardless of kind") impossible without per-table union queries.

## Decision
Introduce a single `claims` table that records "subject relates to object" assertions, plus a `claim_evidence` table that holds supporting *and* refuting rows for any claim. Every new lineage feature reuses these tables; the kind discriminator (`attribution`, `sample`, `credit`, `cover`, `performance`) tells the API how to interpret subject/object pairs.

Quote attribution becomes a claim of kind `attribution` with `subject=quote`, `object=artist`. The existing `quotes` table is untouched; its `provenance_status` and `confidence_score` are still authoritative for the canonical attribution. Claims add the *trail* — supporting citations, refuting citations, rival attributions, merge history — without disturbing the read path.

Concretely:

| Family       | Subject       | Object    | Relation example          |
| ------------ | ------------- | --------- | ------------------------- |
| attribution  | `quote`       | `artist`  | `attributed_to`           |
| sample       | `recording`   | `recording` | `direct_sample`, `interpolation` |
| credit       | `work_credit` | `work`    | role (`composer`, etc.)   |
| cover        | `recording`   | `work`    | `recording_of`            |
| performance  | `performance` | `work`    | `performed`               |

`claim_evidence.supports` is a boolean — the same row schema captures "this NPR article confirms X sampled Y" and "no documented Hendrix interview contains this phrasing". A `/v1/disputes` feed surfaces every claim whose status is `ambiguous`, `disputed`, `refuted`, or `needs_review` across all five kinds.

A separate `quote_merge_log` table closes the ADR-004 audit gap: every merge decision can now be retrieved via `/v1/quotes/{id}/lineage` rather than only inferred from the ingestion audit trail.

## Rationale
- One status workflow, one evidence model, one dispute pipeline — five features for the engineering cost of two.
- Cross-cutting queries are trivial (`SELECT * FROM claims WHERE status IN ('disputed', 'ambiguous')`), which is what powers `/v1/disputes`.
- The schema doesn't bake in the five current kinds; adding a sixth (e.g. live debut claims, plagiarism allegations) is a kind discriminator, not a new table.
- Quote attribution staying on the `quotes` table keeps backward compatibility — the read paths and the existing OpenAPI surfaces are unchanged.

## Consequences
- Foreign keys on the new entity tables are deliberately loose for optional cross-references (e.g. `recordings.work_id`, `performances.recording_id`). SQLite enforces FKs against non-empty values and our DEFAULT '' for optional fields would clash. Code paths (`ResolveOrCreateWork`, `UpsertRecording`) enforce parentage instead, and `IntegrityReport` carries checks for orphan rows.
- Sample edges are constrained as a DAG at write time: `samples` rejects duplicate `(source_recording_id, derivative_recording_id, kind)` rows, self-loops, and bounded reverse reachability cycles before an edge is recorded.
- Claim status resolution happens in Go (`claimStatusRank` in `store_claims.go`) rather than via SQL UDFs, because `modernc/sqlite` doesn't expose user-defined functions cleanly.
- Every curated seed file emits ingestion audit events with action names per kind (`record_sample`, `upsert_work`, `record_performance`, `record_misquote`), keeping the existing `/v1/jobs/{id}/audit` endpoint as the operational source of truth for ingestion behavior.
- Tests live alongside the new store files (`internal/catalog/lineage_test.go`) and as full HTTP surfaces (`internal/api/lineage_test.go`); the existing runtime contract test now exercises every new endpoint.

## Migration
- Schema migration `004_lineage_claims_and_entities` adds the new tables, indices, and FTS5 virtual tables in a single transaction. It is additive: existing data is untouched.
- Schema migration `005_samples_unique_and_no_self_loops` rebuilds `samples` with the duplicate-edge unique constraint and a self-loop CHECK constraint; recursive cycle rejection remains in application code so ingestion can emit audit events.
- The curated seed bundles (`api/data/curated_samples.json`, `curated_works.json`, `curated_performances.json`, `curated_misquotes.json`) ship in the repository and are loaded by `cmd/ingest -bootstrap` alongside the legacy and curated-quote bundles.
