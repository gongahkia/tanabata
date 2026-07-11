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
	{
		Version: 4,
		Name:    "lineage_claims_and_entities",
		Statements: []string{
			`CREATE TABLE IF NOT EXISTS works (
				work_id TEXT PRIMARY KEY,
				mbid TEXT NOT NULL DEFAULT '',
				title TEXT NOT NULL,
				normalized_title TEXT NOT NULL,
				iswc TEXT NOT NULL DEFAULT '',
				language TEXT NOT NULL DEFAULT '',
				created_year TEXT NOT NULL DEFAULT '',
				primary_artist_id TEXT NOT NULL DEFAULT '',
				notes TEXT NOT NULL DEFAULT ''
			);`, // primary_artist_id is enforced in code (allows '' for orphan works)
			`CREATE TABLE IF NOT EXISTS recordings (
				recording_id TEXT PRIMARY KEY,
				mbid TEXT NOT NULL DEFAULT '',
				work_id TEXT NOT NULL DEFAULT '',
				artist_id TEXT NOT NULL,
				title TEXT NOT NULL,
				normalized_title TEXT NOT NULL,
				duration_ms INTEGER NOT NULL DEFAULT 0,
				released_year TEXT NOT NULL DEFAULT '',
				release_id TEXT NOT NULL DEFAULT '',
				isrc TEXT NOT NULL DEFAULT '',
				is_original INTEGER NOT NULL DEFAULT 0,
				notes TEXT NOT NULL DEFAULT '',
				FOREIGN KEY (artist_id) REFERENCES artists(artist_id) ON DELETE CASCADE
			);`, // work_id and release_id are optional cross-refs, enforced in code
			`CREATE TABLE IF NOT EXISTS samples (
				sample_id TEXT PRIMARY KEY,
				source_recording_id TEXT NOT NULL,
				derivative_recording_id TEXT NOT NULL,
				kind TEXT NOT NULL DEFAULT 'direct_sample',
				source_offset_ms INTEGER NOT NULL DEFAULT 0,
				derivative_offset_ms INTEGER NOT NULL DEFAULT 0,
				duration_ms INTEGER NOT NULL DEFAULT 0,
				notes TEXT NOT NULL DEFAULT '',
				FOREIGN KEY (source_recording_id) REFERENCES recordings(recording_id) ON DELETE CASCADE,
				FOREIGN KEY (derivative_recording_id) REFERENCES recordings(recording_id) ON DELETE CASCADE
			);`,
			`CREATE TABLE IF NOT EXISTS work_credits (
				credit_id TEXT PRIMARY KEY,
				work_id TEXT NOT NULL,
				credited_artist_id TEXT NOT NULL DEFAULT '',
				credited_name TEXT NOT NULL,
				role TEXT NOT NULL,
				is_disputed INTEGER NOT NULL DEFAULT 0,
				notes TEXT NOT NULL DEFAULT '',
				FOREIGN KEY (work_id) REFERENCES works(work_id) ON DELETE CASCADE
			);`, // credited_artist_id may be empty for credits naming a contributor without an artist row
			`CREATE TABLE IF NOT EXISTS performances (
				performance_id TEXT PRIMARY KEY,
				artist_id TEXT NOT NULL,
				work_id TEXT NOT NULL DEFAULT '',
				recording_id TEXT NOT NULL DEFAULT '',
				event_name TEXT NOT NULL DEFAULT '',
				venue TEXT NOT NULL DEFAULT '',
				city TEXT NOT NULL DEFAULT '',
				country TEXT NOT NULL DEFAULT '',
				performed_at TEXT NOT NULL,
				setlistfm_id TEXT NOT NULL DEFAULT '',
				position_in_set INTEGER NOT NULL DEFAULT 0,
				notes TEXT NOT NULL DEFAULT '',
				FOREIGN KEY (artist_id) REFERENCES artists(artist_id) ON DELETE CASCADE
			);`, // work_id and recording_id are optional cross-refs, enforced in code
			`CREATE TABLE IF NOT EXISTS claims (
				claim_id TEXT PRIMARY KEY,
				kind TEXT NOT NULL,
				subject_type TEXT NOT NULL,
				subject_id TEXT NOT NULL,
				object_type TEXT NOT NULL,
				object_id TEXT NOT NULL,
				relation TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL,
				confidence_score REAL NOT NULL DEFAULT 0,
				provider_origin TEXT NOT NULL DEFAULT '',
				source_id TEXT NOT NULL DEFAULT '',
				asserted_at TEXT NOT NULL,
				last_verified_at TEXT NOT NULL DEFAULT '',
				notes TEXT NOT NULL DEFAULT ''
			);`,
			`CREATE TABLE IF NOT EXISTS claim_evidence (
				evidence_id TEXT PRIMARY KEY,
				claim_id TEXT NOT NULL,
				supports INTEGER NOT NULL DEFAULT 1,
				source_id TEXT NOT NULL DEFAULT '',
				excerpt TEXT NOT NULL,
				source_url TEXT NOT NULL DEFAULT '',
				archived_url TEXT NOT NULL DEFAULT '',
				evidence_kind TEXT NOT NULL DEFAULT 'manual_note',
				weight REAL NOT NULL DEFAULT 1.0,
				recorded_at TEXT NOT NULL,
				FOREIGN KEY (claim_id) REFERENCES claims(claim_id) ON DELETE CASCADE
			);`,
			`CREATE TABLE IF NOT EXISTS quote_merge_log (
				merge_id TEXT PRIMARY KEY,
				winner_quote_id TEXT NOT NULL,
				loser_quote_id TEXT NOT NULL,
				merge_score INTEGER NOT NULL,
				reason TEXT NOT NULL,
				merged_at TEXT NOT NULL,
				job_id TEXT NOT NULL DEFAULT ''
			);`,
			`CREATE INDEX IF NOT EXISTS idx_recordings_artist ON recordings(artist_id);`,
			`CREATE INDEX IF NOT EXISTS idx_recordings_work ON recordings(work_id);`,
			`CREATE INDEX IF NOT EXISTS idx_recordings_mbid ON recordings(mbid);`,
			`CREATE INDEX IF NOT EXISTS idx_works_mbid ON works(mbid);`,
			`CREATE INDEX IF NOT EXISTS idx_works_title ON works(normalized_title);`,
			`CREATE INDEX IF NOT EXISTS idx_samples_source ON samples(source_recording_id);`,
			`CREATE INDEX IF NOT EXISTS idx_samples_derivative ON samples(derivative_recording_id);`,
			`CREATE INDEX IF NOT EXISTS idx_work_credits_work ON work_credits(work_id);`,
			`CREATE INDEX IF NOT EXISTS idx_work_credits_artist ON work_credits(credited_artist_id);`,
			`CREATE INDEX IF NOT EXISTS idx_performances_artist ON performances(artist_id, performed_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_performances_work ON performances(work_id, performed_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_claims_subject ON claims(subject_type, subject_id, kind);`,
			`CREATE INDEX IF NOT EXISTS idx_claims_object ON claims(object_type, object_id, kind);`,
			`CREATE INDEX IF NOT EXISTS idx_claims_status ON claims(status, kind);`,
			`CREATE INDEX IF NOT EXISTS idx_claim_evidence_claim ON claim_evidence(claim_id, supports);`,
			`CREATE INDEX IF NOT EXISTS idx_quote_merge_winner ON quote_merge_log(winner_quote_id);`,
			`CREATE VIRTUAL TABLE IF NOT EXISTS recording_search USING fts5(
				recording_id UNINDEXED,
				artist_id UNINDEXED,
				artist_name,
				title,
				work_title
			);`,
			`CREATE VIRTUAL TABLE IF NOT EXISTS work_search USING fts5(
				work_id UNINDEXED,
				title,
				primary_artist_name
			);`,
		},
	},
	{
		Version: 5,
		Name:    "samples_unique_and_no_self_loops",
		Statements: []string{
			`CREATE TABLE samples_v5 (
				sample_id TEXT PRIMARY KEY,
				source_recording_id TEXT NOT NULL,
				derivative_recording_id TEXT NOT NULL,
				kind TEXT NOT NULL DEFAULT 'direct_sample',
				source_offset_ms INTEGER NOT NULL DEFAULT 0,
				derivative_offset_ms INTEGER NOT NULL DEFAULT 0,
				duration_ms INTEGER NOT NULL DEFAULT 0,
				notes TEXT NOT NULL DEFAULT '',
				CONSTRAINT samples_unique_edge UNIQUE(source_recording_id, derivative_recording_id, kind),
				CONSTRAINT samples_no_self_loop CHECK(source_recording_id != derivative_recording_id),
				FOREIGN KEY (source_recording_id) REFERENCES recordings(recording_id) ON DELETE CASCADE,
				FOREIGN KEY (derivative_recording_id) REFERENCES recordings(recording_id) ON DELETE CASCADE
			);`,
			`INSERT OR IGNORE INTO samples_v5(sample_id, source_recording_id, derivative_recording_id, kind, source_offset_ms, derivative_offset_ms, duration_ms, notes)
				SELECT sample_id, source_recording_id, derivative_recording_id, kind, source_offset_ms, derivative_offset_ms, duration_ms, notes
				FROM samples
				WHERE source_recording_id != derivative_recording_id;`,
			`DROP TABLE samples;`,
			`ALTER TABLE samples_v5 RENAME TO samples;`,
			`CREATE INDEX IF NOT EXISTS idx_samples_source ON samples(source_recording_id);`,
			`CREATE INDEX IF NOT EXISTS idx_samples_derivative ON samples(derivative_recording_id);`,
		},
	},
}
