# Performance Index Audit

This document maps common backend query paths to the SQLite indexes or FTS tables that keep them predictable.

| Query Path | Store/API Surface | Index Strategy |
| --- | --- | --- |
| Artist alias resolution | `ResolveArtistID`, `/v1/quotes?artist=` | `idx_artist_aliases_normalized` |
| Quote provenance filters | `ListQuotes`, `/v1/quotes?provenance_status=` | `idx_quotes_artist_provenance` |
| Quote source filters | `ListQuotes`, `/v1/quotes?source=` | `idx_quotes_source` plus provider lookup in `quote_sources` |
| Tag filters | `ListQuotes`, `/v1/quotes?tag=` | `idx_quote_tags_tag` |
| Provider runs | `/v1/providers/{provider}/runs` | `idx_provider_runs_lookup` |
| Provider errors | `/v1/providers/{provider}/errors` | `idx_provider_errors_lookup` |
| Provider cooldown status | `/v1/providers`, enrichment orchestration | `idx_provider_cooldowns_until` |
| Job item lookup | `/v1/jobs`, `/v1/timeline` | `idx_job_items_lookup` |
| Ingestion snapshots | `/v1/jobs/{job_id}/snapshots` | `idx_ingestion_snapshots_job` |
| Ingestion audit events | `/v1/jobs/{job_id}/audit` | `idx_ingestion_audit_job` |
| Artist search | `/v1/search` | `artist_search` FTS5 table |
| Quote search | `/v1/search`, `/v1/quotes?q=` | `quote_search` FTS5 table |

The test `TestCommonQueryIndexesExist` locks the required index names so future schema edits cannot silently drop a performance-critical path.
