package catalog

type schemaMigration struct {
	Version    int
	Name       string
	Statements []string
}

var catalogMigrations = []schemaMigration{
	{
		Version: 1,
		Name:    "core_catalog_schema",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS catalog_meta (
				key TEXT PRIMARY KEY,
				value TEXT NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS artists (
				artist_id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				slug TEXT NOT NULL UNIQUE,
				mbid TEXT,
				wikidata_id TEXT,
				wikiquote_title TEXT,
				country TEXT,
				life_span_begin TEXT,
				life_span_end TEXT,
				description TEXT NOT NULL DEFAULT '',
				bio_summary TEXT NOT NULL DEFAULT '',
				provider_status TEXT NOT NULL DEFAULT '{}'
			);`,
			`CREATE TABLE IF NOT EXISTS artist_tags (
				artist_id TEXT NOT NULL,
				tag TEXT NOT NULL,
				PRIMARY KEY (artist_id, tag),
				FOREIGN KEY (artist_id) REFERENCES artists(artist_id) ON DELETE CASCADE
			);`,
			`CREATE TABLE IF NOT EXISTS artist_aliases (
				artist_id TEXT NOT NULL,
				alias TEXT NOT NULL,
				normalized_alias TEXT NOT NULL,
				PRIMARY KEY (artist_id, alias),
				FOREIGN KEY (artist_id) REFERENCES artists(artist_id) ON DELETE CASCADE
			);`,
			`CREATE TABLE IF NOT EXISTS artist_links (
				artist_id TEXT NOT NULL,
				provider TEXT NOT NULL,
				kind TEXT NOT NULL,
				url TEXT NOT NULL,
				external_id TEXT NOT NULL DEFAULT '',
				PRIMARY KEY (artist_id, provider, kind, url),
				FOREIGN KEY (artist_id) REFERENCES artists(artist_id) ON DELETE CASCADE
			);`,
			`CREATE TABLE IF NOT EXISTS quote_sources (
				source_id TEXT PRIMARY KEY,
				provider TEXT NOT NULL,
				url TEXT NOT NULL,
				title TEXT NOT NULL DEFAULT '',
				publisher TEXT NOT NULL DEFAULT '',
				license TEXT NOT NULL DEFAULT '',
				retrieved_at TEXT NOT NULL DEFAULT ''
			);`,
			`CREATE TABLE IF NOT EXISTS quotes (
				quote_id TEXT PRIMARY KEY,
				text TEXT NOT NULL,
				normalized_text TEXT NOT NULL,
				artist_id TEXT NOT NULL,
				source_id TEXT NOT NULL DEFAULT '',
				source_type TEXT NOT NULL DEFAULT '',
				work_title TEXT NOT NULL DEFAULT '',
				year TEXT NOT NULL DEFAULT '',
				provenance_status TEXT NOT NULL,
				confidence_score REAL NOT NULL,
				provider_origin TEXT NOT NULL DEFAULT '',
				license TEXT NOT NULL DEFAULT '',
				first_seen_at TEXT NOT NULL DEFAULT '',
				last_verified_at TEXT NOT NULL DEFAULT '',
				FOREIGN KEY (artist_id) REFERENCES artists(artist_id) ON DELETE CASCADE
			);`,
			`CREATE TABLE IF NOT EXISTS quote_tags (
				quote_id TEXT NOT NULL,
				tag TEXT NOT NULL,
				PRIMARY KEY (quote_id, tag),
				FOREIGN KEY (quote_id) REFERENCES quotes(quote_id) ON DELETE CASCADE
			);`,
			`CREATE TABLE IF NOT EXISTS quote_evidence (
				quote_id TEXT NOT NULL,
				evidence TEXT NOT NULL,
				position INTEGER NOT NULL DEFAULT 0,
				PRIMARY KEY (quote_id, position),
				FOREIGN KEY (quote_id) REFERENCES quotes(quote_id) ON DELETE CASCADE
			);`,
			`CREATE TABLE IF NOT EXISTS artist_relations (
				artist_id TEXT NOT NULL,
				related_artist_id TEXT NOT NULL,
				relation_type TEXT NOT NULL,
				score REAL NOT NULL DEFAULT 0,
				provider TEXT NOT NULL,
				PRIMARY KEY (artist_id, related_artist_id, relation_type, provider),
				FOREIGN KEY (artist_id) REFERENCES artists(artist_id) ON DELETE CASCADE,
				FOREIGN KEY (related_artist_id) REFERENCES artists(artist_id) ON DELETE CASCADE
			);`,
			`CREATE TABLE IF NOT EXISTS releases (
				release_id TEXT PRIMARY KEY,
				artist_id TEXT NOT NULL,
				title TEXT NOT NULL,
				year TEXT NOT NULL DEFAULT '',
				kind TEXT NOT NULL DEFAULT '',
				provider TEXT NOT NULL,
				url TEXT NOT NULL DEFAULT '',
				FOREIGN KEY (artist_id) REFERENCES artists(artist_id) ON DELETE CASCADE
			);`,
			`CREATE TABLE IF NOT EXISTS provider_runs (
				run_id TEXT PRIMARY KEY,
				provider TEXT NOT NULL,
				status TEXT NOT NULL,
				started_at TEXT NOT NULL,
				finished_at TEXT NOT NULL,
				details TEXT NOT NULL DEFAULT ''
			);`,
			`CREATE TABLE IF NOT EXISTS provider_errors (
				error_id TEXT PRIMARY KEY,
				provider TEXT NOT NULL,
				error_kind TEXT NOT NULL DEFAULT '',
				occurred_at TEXT NOT NULL,
				context TEXT NOT NULL DEFAULT '',
				message TEXT NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS provider_cache (
				provider TEXT NOT NULL,
				kind TEXT NOT NULL,
				cache_key TEXT NOT NULL,
				payload TEXT NOT NULL,
				refreshed_at TEXT NOT NULL,
				expires_at TEXT NOT NULL,
				PRIMARY KEY (provider, kind, cache_key)
			);`,
			`CREATE TABLE IF NOT EXISTS provider_cooldowns (
				provider TEXT PRIMARY KEY,
				until TEXT NOT NULL,
				reason TEXT NOT NULL DEFAULT '',
				updated_at TEXT NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS jobs (
				job_id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				scope TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL,
				started_at TEXT NOT NULL,
				finished_at TEXT NOT NULL DEFAULT '',
				details TEXT NOT NULL DEFAULT '',
				error_message TEXT NOT NULL DEFAULT ''
			);`,
			`CREATE TABLE IF NOT EXISTS job_items (
				job_item_id TEXT PRIMARY KEY,
				job_id TEXT NOT NULL,
				provider TEXT NOT NULL,
				target TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL,
				started_at TEXT NOT NULL,
				finished_at TEXT NOT NULL DEFAULT '',
				details TEXT NOT NULL DEFAULT '',
				error_message TEXT NOT NULL DEFAULT '',
				FOREIGN KEY (job_id) REFERENCES jobs(job_id) ON DELETE CASCADE
			);`,
			`CREATE TABLE IF NOT EXISTS ingestion_snapshots (
				snapshot_id TEXT PRIMARY KEY,
				job_id TEXT NOT NULL DEFAULT '',
				phase TEXT NOT NULL,
				captured_at TEXT NOT NULL,
				counts_json TEXT NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS ingestion_audit_events (
				event_id TEXT PRIMARY KEY,
				job_id TEXT NOT NULL DEFAULT '',
				job_item_id TEXT NOT NULL DEFAULT '',
				provider TEXT NOT NULL DEFAULT '',
				target TEXT NOT NULL DEFAULT '',
				action TEXT NOT NULL,
				status TEXT NOT NULL,
				occurred_at TEXT NOT NULL,
				details TEXT NOT NULL DEFAULT ''
			);`,
		},
	},
	{
		Version: 2,
		Name:    "catalog_indexes",
		Statements: []string{
			`CREATE INDEX IF NOT EXISTS idx_artist_aliases_normalized ON artist_aliases(normalized_alias);`,
			`CREATE INDEX IF NOT EXISTS idx_quotes_artist_provenance ON quotes(artist_id, provenance_status, confidence_score DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_quotes_source ON quotes(source_id);`,
			`CREATE INDEX IF NOT EXISTS idx_quote_tags_tag ON quote_tags(tag);`,
			`CREATE INDEX IF NOT EXISTS idx_provider_runs_lookup ON provider_runs(provider, status, started_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_provider_errors_lookup ON provider_errors(provider, occurred_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_provider_cooldowns_until ON provider_cooldowns(until);`,
			`CREATE INDEX IF NOT EXISTS idx_job_items_lookup ON job_items(job_id, provider, started_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_ingestion_snapshots_job ON ingestion_snapshots(job_id, captured_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_ingestion_audit_job ON ingestion_audit_events(job_id, occurred_at DESC);`,
		},
	},
	{
		Version: 3,
		Name:    "fts_search",
		Statements: []string{
			`CREATE VIRTUAL TABLE IF NOT EXISTS artist_search USING fts5(
				artist_id UNINDEXED,
				name,
				aliases
			);`,
			`CREATE VIRTUAL TABLE IF NOT EXISTS quote_search USING fts5(
				quote_id UNINDEXED,
				artist_id UNINDEXED,
				artist_name,
				text,
				tags
			);`,
		},
	},
}
