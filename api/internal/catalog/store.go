package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

const snapshotVersionKey = "snapshot_version"
const activeProvidersKey = "active_providers"
const quoteFreshnessStaleAfter = 180 * 24 * time.Hour
const quoteFreshnessAgingAfter = 90 * 24 * time.Hour
const sqlitePragmaSuffix = "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)"

type Store struct {
	db             *sql.DB
	webhookEmitter WebhookEmitter
}

type WebhookEmitter interface {
	EmitWebhookEvent(context.Context, models.WebhookEvent)
}

func (s *Store) SetWebhookEmitter(emitter WebhookEmitter) {
	s.webhookEmitter = emitter
}

type OrphanRepairResult struct {
	Applied bool
	Counts  map[string]int
	Targets map[string][]string
}

func (r OrphanRepairResult) Total() int {
	total := 0
	for _, count := range r.Counts {
		total += count
	}
	return total
}

type ProviderRun struct {
	RunID      string
	Provider   string
	Status     string
	StartedAt  time.Time
	FinishedAt time.Time
	Details    string
}

type ProviderError struct {
	ErrorID    string
	Provider   string
	Kind       string
	OccurredAt time.Time
	Context    string
	Message    string
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create catalog dir: %w", err)
	}
	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := verifySQLitePragmas(context.Background(), db, path); err != nil {
		_ = db.Close()
		return nil, err
	}

	store := &Store{db: db}
	if err := store.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func sqliteDSN(path string) string {
	if strings.Contains(path, "?") {
		return path + "&" + strings.TrimPrefix(sqlitePragmaSuffix, "?")
	}
	return path + sqlitePragmaSuffix
}

func verifySQLitePragmas(ctx context.Context, db *sql.DB, path string) error {
	pragmas, err := resolvedSQLitePragmas(ctx, db)
	if err != nil {
		return err
	}
	if pragmas.journalMode != "wal" {
		if isInMemorySQLitePath(path) {
			slog.Default().Warn("sqlite_wal_unavailable", "path", path, "journal_mode", pragmas.journalMode)
		} else {
			return fmt.Errorf("sqlite WAL unavailable: journal_mode=%s", pragmas.journalMode)
		}
	}
	slog.Default().Info(
		"sqlite_pragmas_configured",
		"path", path,
		"journal_mode", pragmas.journalMode,
		"busy_timeout", pragmas.busyTimeout,
		"foreign_keys", pragmas.foreignKeys,
		"synchronous", pragmas.synchronous,
		"temp_store", pragmas.tempStore,
		"max_open_conns", 1,
	)
	return nil
}

type sqlitePragmas struct {
	journalMode string
	busyTimeout int
	foreignKeys int
	synchronous int
	tempStore   int
}

func resolvedSQLitePragmas(ctx context.Context, db *sql.DB) (sqlitePragmas, error) {
	var pragmas sqlitePragmas
	if err := db.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&pragmas.journalMode); err != nil {
		return pragmas, fmt.Errorf("read sqlite journal_mode: %w", err)
	}
	pragmas.journalMode = strings.ToLower(strings.TrimSpace(pragmas.journalMode))
	if err := db.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&pragmas.busyTimeout); err != nil {
		return pragmas, fmt.Errorf("read sqlite busy_timeout: %w", err)
	}
	if err := db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&pragmas.foreignKeys); err != nil {
		return pragmas, fmt.Errorf("read sqlite foreign_keys: %w", err)
	}
	if err := db.QueryRowContext(ctx, `PRAGMA synchronous`).Scan(&pragmas.synchronous); err != nil {
		return pragmas, fmt.Errorf("read sqlite synchronous: %w", err)
	}
	if err := db.QueryRowContext(ctx, `PRAGMA temp_store`).Scan(&pragmas.tempStore); err != nil {
		return pragmas, fmt.Errorf("read sqlite temp_store: %w", err)
	}
	return pragmas, nil
}

func isInMemorySQLitePath(path string) bool {
	return path == ":memory:" || strings.Contains(path, "mode=memory")
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.QueryRowContext(ctx, `SELECT 1`).Scan(new(int))
}

func (s *Store) init(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA journal_mode = WAL;`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		);`,
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	if err := s.applySchemaMigrations(ctx); err != nil {
		return err
	}
	if err := s.migrate(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Store) applySchemaMigrations(ctx context.Context) error {
	applied, err := s.appliedMigrationVersions(ctx)
	if err != nil {
		return err
	}
	for _, migration := range catalogMigrations {
		if applied[migration.Version] {
			continue
		}
		if migration.ForeignKeysOff {
			if err := s.applySchemaMigrationWithForeignKeysOff(ctx, migration); err != nil {
				return err
			}
			continue
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, stmt := range migration.Statements {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply migration %03d %s: %w", migration.Version, migration.Name, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO schema_migrations(version, name, applied_at)
			VALUES(?, ?, ?)
		`, migration.Version, migration.Name, time.Now().UTC().Format(time.RFC3339)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %03d %s: %w", migration.Version, migration.Name, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) applySchemaMigrationWithForeignKeysOff(ctx context.Context, migration schemaMigration) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return fmt.Errorf("disable foreign keys for migration %03d %s: %w", migration.Version, migration.Name, err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA legacy_alter_table = ON`); err != nil {
		_, _ = s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON`)
		return fmt.Errorf("enable legacy alter table for migration %03d %s: %w", migration.Version, migration.Name, err)
	}
	defer func() {
		_, _ = s.db.ExecContext(context.Background(), `PRAGMA legacy_alter_table = OFF`)
		_, _ = s.db.ExecContext(context.Background(), `PRAGMA foreign_keys = ON`)
	}()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, stmt := range migration.Statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %03d %s: %w", migration.Version, migration.Name, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO schema_migrations(version, name, applied_at)
		VALUES(?, ?, ?)
	`, migration.Version, migration.Name, time.Now().UTC().Format(time.RFC3339)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %03d %s: %w", migration.Version, migration.Name, err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) appliedMigrationVersions(ctx context.Context) (map[int]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	applied := map[int]bool{}
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

func (s *Store) AppliedMigrations(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT printf('%03d:%s', version, name)
		FROM schema_migrations
		ORDER BY version ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var migrations []string
	for rows.Next() {
		var migration string
		if err := rows.Scan(&migration); err != nil {
			return nil, err
		}
		migrations = append(migrations, migration)
	}
	return migrations, rows.Err()
}

func (s *Store) indexNames(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT name
		FROM sqlite_master
		WHERE type = 'index'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	indexes := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		indexes[name] = true
	}
	return indexes, rows.Err()
}

func (s *Store) migrate(ctx context.Context) error {
	if err := s.ensureColumn(ctx, "provider_errors", "error_kind", `ALTER TABLE provider_errors ADD COLUMN error_kind TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "quotes", "provider_origin", `ALTER TABLE quotes ADD COLUMN provider_origin TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE quotes
		SET provenance_status = 'needs_review'
		WHERE provenance_status = 'legacy_unverified' OR provenance_status = ''
	`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE quotes
		SET provider_origin = COALESCE(
			NULLIF(provider_origin, ''),
			(SELECT provider FROM quote_sources WHERE quote_sources.source_id = quotes.source_id),
			'legacy'
		)
		WHERE provider_origin = ''
	`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO quote_evidence(quote_id, evidence, position)
		SELECT quote_id,
			CASE
				WHEN provenance_status = 'source_attributed' THEN 'Source metadata exists, but evidence predates Tanabata V2.'
				ELSE 'Imported from the legacy catalog and requires manual verification.'
			END,
			0
		FROM quotes
		WHERE quote_id NOT IN (SELECT quote_id FROM quote_evidence)
	`); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table, column, ddl string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var (
			cid        int
			name       string
			colType    string
			notNull    int
			defaultV   sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultV, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		if name == column {
			found = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = s.db.ExecContext(ctx, ddl)
	return err
}

func (s *Store) SeedFromLegacyJSON(ctx context.Context, legacyPath string) error {
	count, err := s.quoteCount(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	type legacyQuote struct {
		Author string `json:"author"`
		Text   string `json:"text"`
	}

	content, err := os.ReadFile(legacyPath) // #nosec G304 -- caller-provided import path
	if err != nil {
		return fmt.Errorf("read legacy json: %w", err)
	}
	var quotes []legacyQuote
	if err := json.Unmarshal(content, &quotes); err != nil {
		return fmt.Errorf("decode legacy json: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	sourceURL := "https://quotefancy.com"
	sourceID := search.SourceID("quotefancy", sourceURL)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO quote_sources(source_id, provider, url, title, publisher, license, retrieved_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)
	`, sourceID, "quotefancy", sourceURL, "QuoteFancy legacy import", "QuoteFancy", "unknown", now); err != nil {
		return fmt.Errorf("insert legacy source: %w", err)
	}

	artists := make(map[string]struct{})
	for _, quote := range quotes {
		name := strings.TrimSpace(quote.Author)
		if name == "" {
			continue
		}
		artistID := search.ArtistID(name, "")
		if _, seen := artists[artistID]; !seen {
			artists[artistID] = struct{}{}
			status, _ := json.Marshal(map[string]string{"legacy": "seeded"})
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO artists(artist_id, name, slug, provider_status)
				VALUES(?, ?, ?, ?)
			`, artistID, name, search.Slug(name), string(status)); err != nil {
				return fmt.Errorf("insert artist %s: %w", name, err)
			}
			for _, alias := range dedupeStrings([]string{name, strings.ReplaceAll(name, " ", "-")}) {
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO artist_aliases(artist_id, alias, normalized_alias)
					VALUES(?, ?, ?)
				`, artistID, alias, search.NormalizeText(alias)); err != nil {
					return fmt.Errorf("insert alias %s: %w", alias, err)
				}
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO artist_links(artist_id, provider, kind, url, external_id)
				VALUES(?, ?, ?, ?, ?)
			`, artistID, "quotefancy", "source_home", sourceURL, ""); err != nil {
				return fmt.Errorf("insert legacy link: %w", err)
			}
		}

		normalizedText := search.NormalizeText(quote.Text)
		quoteID := search.QuoteID(artistID, normalizedText, sourceURL)
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO quotes(
				quote_id, text, normalized_text, artist_id, source_id, source_type, work_title, year,
				provenance_status, confidence_score, provider_origin, license, first_seen_at, last_verified_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, quoteID, strings.TrimSpace(quote.Text), normalizedText, artistID, sourceID, "legacy_scrape", "", "", "needs_review", 0.25, "quotefancy", "unknown", now, now); err != nil {
			return fmt.Errorf("insert quote: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO quote_evidence(quote_id, evidence, position)
			VALUES(?, ?, ?)
		`, quoteID, "Imported from QuoteFancy during legacy bootstrap; requires manual verification.", 0); err != nil {
			return fmt.Errorf("insert quote evidence: %w", err)
		}
	}

	if err := s.setMetaTx(ctx, tx, snapshotVersionKey, now); err != nil {
		return err
	}
	if err := s.setMetaTx(ctx, tx, activeProvidersKey, "quotefancy"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO provider_runs(run_id, provider, status, started_at, finished_at, details)
		VALUES(?, ?, ?, ?, ?, ?)
	`, search.StableHash("legacy-import", now), "quotefancy", "success", now, now, "seeded from quotes.json"); err != nil {
		return fmt.Errorf("insert provider run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.rebuildSearchIndices(ctx)
}

func (s *Store) ImportCuratedQuotes(ctx context.Context, bundlePath string) (int, error) {
	content, err := os.ReadFile(bundlePath) // #nosec G304 -- caller-provided curated bundle path
	if err != nil {
		return 0, fmt.Errorf("read curated quotes: %w", err)
	}
	var records []models.CuratedQuoteRecord
	if err := json.Unmarshal(content, &records); err != nil {
		return 0, fmt.Errorf("decode curated quotes: %w", err)
	}

	imported := 0
	for _, record := range records {
		artistName := strings.TrimSpace(record.ArtistName)
		if artistName == "" || strings.TrimSpace(record.Text) == "" {
			continue
		}

		artistID, err := s.ResolveArtistID(ctx, artistName)
		if err != nil {
			return imported, err
		}
		artist := &models.Artist{
			ArtistID: artistID,
			Name:     artistName,
			Aliases:  dedupeStrings(append(record.Aliases, artistName)),
			Genres:   []string{},
			Links:    []models.ArtistLink{},
			ProviderStatus: map[string]string{
				"tanabata_curated": "imported",
			},
		}
		if artistID == "" {
			artist.ArtistID = search.ArtistID(artistName, "")
		} else if existing, err := s.ArtistByID(ctx, artistID); err != nil {
			return imported, err
		} else if existing != nil {
			artist = existing
			artist.Aliases = dedupeStrings(append(existing.Aliases, record.Aliases...))
			if artist.ProviderStatus == nil {
				artist.ProviderStatus = map[string]string{}
			}
			artist.ProviderStatus["tanabata_curated"] = "imported"
		}
		if err := s.UpsertArtist(ctx, *artist); err != nil {
			return imported, err
		}

		source := record.Source
		if source != nil {
			if source.SourceID == "" {
				source.SourceID = search.SourceID(source.Provider, source.URL)
			}
			if source.Provider == "" {
				source.Provider = "tanabata_curated"
			}
			if source.License == "" {
				source.License = record.License
			}
			if err := s.UpsertSource(ctx, *source); err != nil {
				return imported, err
			}
		}

		providerOrigin := strings.TrimSpace(record.ProviderOrigin)
		if providerOrigin == "" {
			if source != nil && source.Provider != "" {
				providerOrigin = source.Provider
			} else {
				providerOrigin = "tanabata_curated"
			}
		}
		license := strings.TrimSpace(record.License)
		if license == "" && source != nil {
			license = source.License
		}
		firstSeenAt := strings.TrimSpace(record.FirstSeenAt)
		if firstSeenAt == "" && source != nil {
			firstSeenAt = source.RetrievedAt
		}
		lastVerifiedAt := strings.TrimSpace(record.LastVerifiedAt)
		if lastVerifiedAt == "" {
			lastVerifiedAt = firstSeenAt
		}
		sourceID := ""
		if source != nil {
			sourceID = source.SourceID
		}

		if err := s.UpsertQuote(ctx, models.Quote{
			Text:             strings.TrimSpace(record.Text),
			ArtistID:         artist.ArtistID,
			ArtistName:       artistName,
			SourceID:         sourceID,
			SourceType:       record.SourceType,
			WorkTitle:        record.WorkTitle,
			Tags:             record.Tags,
			ProvenanceStatus: record.ProvenanceStatus,
			ConfidenceScore:  record.ConfidenceScore,
			ProviderOrigin:   providerOrigin,
			Evidence:         record.Evidence,
			License:          license,
			FirstSeenAt:      firstSeenAt,
			LastVerifiedAt:   lastVerifiedAt,
			Source:           source,
		}); err != nil {
			return imported, err
		}
		imported++
	}
	return imported, nil
}

func (s *Store) RefreshSearchIndices(ctx context.Context) error {
	return s.rebuildSearchIndices(ctx)
}

func (s *Store) quoteCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM quotes`).Scan(&count)
	return count, err
}

func (s *Store) Meta(ctx context.Context) (models.ListMeta, error) {
	meta := models.ListMeta{}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(value, '') FROM catalog_meta WHERE key = ?`, snapshotVersionKey).Scan(&meta.SnapshotVersion); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return meta, err
	}
	var providers string
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(value, '') FROM catalog_meta WHERE key = ?`, activeProvidersKey).Scan(&providers); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return meta, err
	}
	if providers != "" {
		meta.ActiveProviders = strings.Split(providers, ",")
	}
	return meta, nil
}

func (s *Store) setMetaTx(ctx context.Context, tx *sql.Tx, key, value string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO catalog_meta(key, value)
		VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO catalog_meta(key, value)
		VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

func (s *Store) UpdateActiveProviders(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT provider FROM provider_runs WHERE status = 'success'`)
	if err != nil {
		return err
	}
	var providers []string
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			_ = rows.Close()
			return err
		}
		providers = append(providers, provider)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	sort.Strings(providers)
	return s.SetMeta(ctx, activeProvidersKey, strings.Join(providers, ","))
}

func (s *Store) RecordProviderRun(ctx context.Context, run ProviderRun) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO provider_runs(run_id, provider, status, started_at, finished_at, details)
		VALUES(?, ?, ?, ?, ?, ?)
	`, run.RunID, run.Provider, run.Status, run.StartedAt.UTC().Format(time.RFC3339), run.FinishedAt.UTC().Format(time.RFC3339), run.Details)
	return err
}

func (s *Store) RecordProviderError(ctx context.Context, failure ProviderError) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO provider_errors(error_id, provider, error_kind, occurred_at, context, message)
		VALUES(?, ?, ?, ?, ?, ?)
	`, failure.ErrorID, failure.Provider, failure.Kind, failure.OccurredAt.UTC().Format(time.RFC3339), failure.Context, failure.Message)
	return err
}

func (s *Store) UpsertArtist(ctx context.Context, artist models.Artist) error {
	status := artist.ProviderStatus
	if status == nil {
		status = map[string]string{}
	}
	statusJSON, err := json.Marshal(status)
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO artists(artist_id, name, slug, mbid, wikidata_id, wikiquote_title, country, life_span_begin, life_span_end, description, bio_summary, provider_status)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(artist_id) DO UPDATE SET
			name = excluded.name,
			slug = excluded.slug,
			mbid = excluded.mbid,
			wikidata_id = excluded.wikidata_id,
			wikiquote_title = excluded.wikiquote_title,
			country = excluded.country,
			life_span_begin = excluded.life_span_begin,
			life_span_end = excluded.life_span_end,
			description = excluded.description,
			bio_summary = excluded.bio_summary,
			provider_status = excluded.provider_status
	`, artist.ArtistID, artist.Name, search.Slug(artist.Name), artist.MBID, artist.WikidataID, artist.WikiquoteTitle, artist.Country, artist.LifeSpan.Begin, artist.LifeSpan.End, artist.Description, artist.BioSummary, string(statusJSON)); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM artist_aliases WHERE artist_id = ?`, artist.ArtistID); err != nil {
		return err
	}
	for _, alias := range dedupeStrings(append(artist.Aliases, artist.Name)) {
		if _, err := s.db.ExecContext(ctx, `
			INSERT OR IGNORE INTO artist_aliases(artist_id, alias, normalized_alias)
			VALUES(?, ?, ?)
		`, artist.ArtistID, alias, search.NormalizeText(alias)); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM artist_links WHERE artist_id = ?`, artist.ArtistID); err != nil {
		return err
	}
	for _, link := range dedupeArtistLinks(artist.Links) {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO artist_links(artist_id, provider, kind, url, external_id)
			VALUES(?, ?, ?, ?, ?)
			ON CONFLICT(artist_id, provider, kind, url) DO UPDATE SET
				external_id = excluded.external_id
		`, artist.ArtistID, link.Provider, link.Kind, link.URL, link.ExternalID); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM artist_tags WHERE artist_id = ?`, artist.ArtistID); err != nil {
		return err
	}
	for _, genre := range dedupeStrings(artist.Genres) {
		if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO artist_tags(artist_id, tag) VALUES(?, ?)`, artist.ArtistID, genre); err != nil {
			return err
		}
	}
	return s.syncArtistSearch(ctx, artist)
}

func (s *Store) RekeyArtist(ctx context.Context, oldID, newID string, canonicalName string) error {
	oldID = strings.TrimSpace(oldID)
	newID = strings.TrimSpace(newID)
	if oldID == "" || newID == "" || oldID == newID {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM artists WHERE artist_id = ?`, oldID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO artists(artist_id, name, slug)
		SELECT ?, COALESCE(NULLIF(name, ''), ?), ?
		FROM artists
		WHERE artist_id = ?
	`, newID, canonicalName, search.Slug(canonicalName)+"-"+search.StableHash(newID)[:8], oldID); err != nil {
		return err
	}
	for _, table := range rekeyArtistTables {
		stmt, ok := rekeyArtistTableUpdateSQL(table)
		if !ok {
			return errors.New("unsupported artist rekey table")
		}
		if _, err := tx.ExecContext(ctx, stmt, newID, oldID); err != nil {
			return err
		}
	}
	for _, column := range rekeyArtistRelationColumns {
		stmt, ok := rekeyArtistRelationUpdateSQL(column)
		if !ok {
			return errors.New("unsupported artist rekey relation column")
		}
		if _, err := tx.ExecContext(ctx, stmt, newID, oldID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM artists WHERE artist_id = ?`, oldID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.rebuildSearchIndices(ctx)
}

var rekeyArtistTables = []string{"quotes", "artist_aliases", "artist_links", "artist_tags", "releases"}
var rekeyArtistRelationColumns = []string{"artist_id", "related_artist_id"}

func rekeyArtistTableUpdateSQL(table string) (string, bool) {
	switch table {
	case "quotes":
		return `UPDATE quotes SET artist_id = ? WHERE artist_id = ?`, true
	case "artist_aliases":
		return `UPDATE artist_aliases SET artist_id = ? WHERE artist_id = ?`, true
	case "artist_links":
		return `UPDATE artist_links SET artist_id = ? WHERE artist_id = ?`, true
	case "artist_tags":
		return `UPDATE artist_tags SET artist_id = ? WHERE artist_id = ?`, true
	case "releases":
		return `UPDATE releases SET artist_id = ? WHERE artist_id = ?`, true
	default:
		return "", false
	}
}

func rekeyArtistRelationUpdateSQL(column string) (string, bool) {
	switch column {
	case "artist_id":
		return `UPDATE artist_relations SET artist_id = ? WHERE artist_id = ?`, true
	case "related_artist_id":
		return `UPDATE artist_relations SET related_artist_id = ? WHERE related_artist_id = ?`, true
	default:
		return "", false
	}
}

func (s *Store) UpsertSource(ctx context.Context, source models.Source) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO quote_sources(source_id, provider, url, title, publisher, license, retrieved_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_id) DO UPDATE SET
			provider = excluded.provider,
			url = excluded.url,
			title = excluded.title,
			publisher = excluded.publisher,
			license = excluded.license,
			retrieved_at = excluded.retrieved_at
	`, source.SourceID, source.Provider, source.URL, source.Title, source.Publisher, source.License, source.RetrievedAt)
	return err
}

func (s *Store) UpsertQuote(ctx context.Context, quote models.Quote) error {
	existingID, existingStatus, mergeScore, err := s.existingQuote(ctx, quote.ArtistID, quote.Text)
	if err != nil {
		return err
	}
	var mergeLog *models.QuoteMergeLog
	if existingID != "" {
		incomingQuoteID := candidateQuoteID(quote)
		existingQuote, err := s.QuoteByID(ctx, existingID)
		if err != nil {
			return err
		}
		if existingQuote != nil {
			quote = mergeQuotes(*existingQuote, quote)
		}
		if existingStatus == "needs_review" || existingStatus == "legacy_unverified" || quote.QuoteID == "" {
			quote.QuoteID = existingID
		}
		if incomingQuoteID != "" && incomingQuoteID != quote.QuoteID {
			mergeLog = &models.QuoteMergeLog{
				WinnerQuoteID: quote.QuoteID,
				LoserQuoteID:  incomingQuoteID,
				MergeScore:    mergeScore,
				Reason:        "near_duplicate_quote",
				MergedAt:      time.Now().UTC().Format(time.RFC3339),
			}
		}
	}
	if quote.QuoteID == "" {
		quote.QuoteID = candidateQuoteID(quote)
	}
	year := ""
	if quote.Year != nil {
		year = strconv.Itoa(*quote.Year)
	}
	providerOrigin := strings.TrimSpace(quote.ProviderOrigin)
	if providerOrigin == "" && quote.Source != nil {
		providerOrigin = quote.Source.Provider
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO quotes(
			quote_id, text, normalized_text, artist_id, source_id, source_type, work_title, year,
			provenance_status, confidence_score, provider_origin, license, first_seen_at, last_verified_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(quote_id) DO UPDATE SET
			text = excluded.text,
			normalized_text = excluded.normalized_text,
			artist_id = excluded.artist_id,
			source_id = excluded.source_id,
			source_type = excluded.source_type,
			work_title = excluded.work_title,
			year = excluded.year,
			provenance_status = CASE
				WHEN (CASE excluded.provenance_status WHEN 'verified' THEN 5 WHEN 'source_attributed' THEN 4 WHEN 'provider_attributed' THEN 3 WHEN 'ambiguous' THEN 2 WHEN 'needs_review' THEN 1 ELSE 0 END) > (CASE quotes.provenance_status WHEN 'verified' THEN 5 WHEN 'source_attributed' THEN 4 WHEN 'provider_attributed' THEN 3 WHEN 'ambiguous' THEN 2 WHEN 'needs_review' THEN 1 ELSE 0 END)
					OR ((CASE excluded.provenance_status WHEN 'verified' THEN 5 WHEN 'source_attributed' THEN 4 WHEN 'provider_attributed' THEN 3 WHEN 'ambiguous' THEN 2 WHEN 'needs_review' THEN 1 ELSE 0 END) = (CASE quotes.provenance_status WHEN 'verified' THEN 5 WHEN 'source_attributed' THEN 4 WHEN 'provider_attributed' THEN 3 WHEN 'ambiguous' THEN 2 WHEN 'needs_review' THEN 1 ELSE 0 END) AND excluded.confidence_score >= quotes.confidence_score)
				THEN excluded.provenance_status ELSE quotes.provenance_status END,
			confidence_score = MAX(quotes.confidence_score, excluded.confidence_score),
			provider_origin = CASE
				WHEN (CASE excluded.provenance_status WHEN 'verified' THEN 5 WHEN 'source_attributed' THEN 4 WHEN 'provider_attributed' THEN 3 WHEN 'ambiguous' THEN 2 WHEN 'needs_review' THEN 1 ELSE 0 END) > (CASE quotes.provenance_status WHEN 'verified' THEN 5 WHEN 'source_attributed' THEN 4 WHEN 'provider_attributed' THEN 3 WHEN 'ambiguous' THEN 2 WHEN 'needs_review' THEN 1 ELSE 0 END)
					OR ((CASE excluded.provenance_status WHEN 'verified' THEN 5 WHEN 'source_attributed' THEN 4 WHEN 'provider_attributed' THEN 3 WHEN 'ambiguous' THEN 2 WHEN 'needs_review' THEN 1 ELSE 0 END) = (CASE quotes.provenance_status WHEN 'verified' THEN 5 WHEN 'source_attributed' THEN 4 WHEN 'provider_attributed' THEN 3 WHEN 'ambiguous' THEN 2 WHEN 'needs_review' THEN 1 ELSE 0 END) AND excluded.confidence_score >= quotes.confidence_score)
				THEN COALESCE(NULLIF(excluded.provider_origin, ''), quotes.provider_origin) ELSE quotes.provider_origin END,
			license = excluded.license,
			first_seen_at = excluded.first_seen_at,
			last_verified_at = excluded.last_verified_at
	`, quote.QuoteID, quote.Text, search.NormalizeText(quote.Text), quote.ArtistID, nullToEmpty(quote.SourceID), quote.SourceType, quote.WorkTitle, year, quote.ProvenanceStatus, quote.ConfidenceScore, providerOrigin, quote.License, quote.FirstSeenAt, quote.LastVerifiedAt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM quote_tags WHERE quote_id = ?`, quote.QuoteID); err != nil {
		return err
	}
	for _, tag := range dedupeStrings(quote.Tags) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO quote_tags(quote_id, tag) VALUES(?, ?)`, quote.QuoteID, tag); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM quote_evidence WHERE quote_id = ?`, quote.QuoteID); err != nil {
		return err
	}
	for idx, evidence := range dedupeStrings(quote.Evidence) {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO quote_evidence(quote_id, evidence, position)
			VALUES(?, ?, ?)
		`, quote.QuoteID, evidence, idx); err != nil {
			return err
		}
	}
	if len(quote.Evidence) == 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO quote_evidence(quote_id, evidence, position)
			VALUES(?, ?, 0)
		`, quote.QuoteID, "Imported without explicit evidence; inspect source metadata."); err != nil {
			return err
		}
	}
	if mergeLog != nil {
		if err := recordQuoteMerge(ctx, tx, *mergeLog); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	if quote.ArtistName == "" {
		artist, _ := s.ArtistByID(ctx, quote.ArtistID)
		if artist != nil {
			quote.ArtistName = artist.Name
		}
	}
	return s.syncQuoteSearch(ctx, quote)
}

func candidateQuoteID(quote models.Quote) string {
	if strings.TrimSpace(quote.QuoteID) != "" {
		return quote.QuoteID
	}
	sourceURL := ""
	if quote.Source != nil {
		sourceURL = quote.Source.URL
	}
	return search.QuoteID(quote.ArtistID, search.NormalizeText(quote.Text), sourceURL)
}

func (s *Store) existingQuote(ctx context.Context, artistID, text string) (string, string, int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT quote_id, provenance_status, text
		FROM quotes
		WHERE artist_id = ?
	`, artistID)
	if err != nil {
		return "", "", 0, err
	}
	defer rows.Close()

	bestID := ""
	bestProvenance := ""
	bestScore := 0
	for rows.Next() {
		var quoteID, provenance, candidateText string
		if err := rows.Scan(&quoteID, &provenance, &candidateText); err != nil {
			return "", "", 0, err
		}
		score := search.QuoteMergeScore(text, candidateText)
		if score > bestScore {
			bestID = quoteID
			bestProvenance = provenance
			bestScore = score
		}
	}
	if err := rows.Err(); err != nil {
		return "", "", 0, err
	}
	if bestScore < 90 {
		return "", "", 0, nil
	}
	return bestID, bestProvenance, bestScore, nil
}

func (s *Store) ReplaceArtistRelations(ctx context.Context, artistID string, relations []models.RelatedArtist) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM artist_relations WHERE artist_id = ?`, artistID); err != nil {
		return err
	}
	for _, relation := range relations {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO artist_relations(artist_id, related_artist_id, relation_type, score, provider)
			VALUES(?, ?, ?, ?, ?)
		`, artistID, relation.ArtistID, relation.Relation, relation.Score, relation.Provider); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ReplaceReleases(ctx context.Context, artistID string, releases []models.Release) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM releases WHERE artist_id = ?`, artistID); err != nil {
		return err
	}
	for _, release := range releases {
		year := ""
		if release.Year != nil {
			year = strconv.Itoa(*release.Year)
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO releases(release_id, artist_id, title, year, kind, provider, url)
			VALUES(?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(release_id) DO UPDATE SET
				title = excluded.title,
				year = excluded.year,
				kind = excluded.kind,
				provider = excluded.provider,
				url = excluded.url
			WHERE releases.artist_id = excluded.artist_id
		`, release.ReleaseID, artistID, release.Title, year, release.Kind, release.Provider, release.URL); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) LegacyQuotes(ctx context.Context, author string) ([]models.LegacyQuote, error) {
	base := `
		SELECT artists.name, quotes.text
		FROM quotes
		JOIN artists ON artists.artist_id = quotes.artist_id
	`
	args := []any{}
	if author != "" {
		base += ` WHERE lower(artists.name) = lower(?) ORDER BY quotes.text`
		args = append(args, author)
	} else {
		base += ` ORDER BY artists.name, quotes.text`
	}
	rows, err := s.db.QueryContext(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []models.LegacyQuote
	for rows.Next() {
		var quote models.LegacyQuote
		if err := rows.Scan(&quote.Author, &quote.Text); err != nil {
			return nil, err
		}
		results = append(results, quote)
	}
	return results, rows.Err()
}

func (s *Store) RandomLegacyQuote(ctx context.Context) (*models.LegacyQuote, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT artists.name, quotes.text
		FROM quotes
		JOIN artists ON artists.artist_id = quotes.artist_id
		ORDER BY RANDOM()
		LIMIT 1
	`)
	var quote models.LegacyQuote
	if err := row.Scan(&quote.Author, &quote.Text); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &quote, nil
}

func (s *Store) ListQuotes(ctx context.Context, filters models.QuoteFilters) (models.ListResponse[models.Quote], error) {
	response := models.ListResponse[models.Quote]{}
	queryParts := []string{
		`SELECT
			quotes.quote_id,
			quotes.text,
			artists.artist_id,
			artists.name,
			quotes.source_id,
			quotes.source_type,
			quotes.work_title,
			quotes.year,
			quotes.provenance_status,
			quotes.confidence_score,
			quotes.provider_origin,
			quotes.license,
			quotes.first_seen_at,
			quotes.last_verified_at
		FROM quotes
		JOIN artists ON artists.artist_id = quotes.artist_id`,
	}
	countParts := []string{
		`SELECT COUNT(*) FROM quotes JOIN artists ON artists.artist_id = quotes.artist_id`,
	}
	var where []string
	var args []any

	if filters.ArtistID != "" {
		where = append(where, `artists.artist_id = ?`)
		args = append(args, filters.ArtistID)
	}
	if filters.Artist != "" {
		where = append(where, `artists.artist_id IN (
			SELECT artist_id FROM artist_aliases WHERE normalized_alias = ?
		)`)
		args = append(args, search.NormalizeText(filters.Artist))
	}
	if filters.Query != "" {
		fts := ftsQuery(filters.Query)
		pattern := "%" + search.NormalizeText(filters.Query) + "%"
		if fts != "" {
			where = append(where, `quotes.quote_id IN (
				SELECT quote_id FROM quote_search WHERE quote_search MATCH ?
				UNION
				SELECT quote_id FROM quotes WHERE normalized_text LIKE ?
			)`)
			args = append(args, fts, pattern)
		} else {
			where = append(where, `quotes.normalized_text LIKE ?`)
			args = append(args, pattern)
		}
	}
	if filters.Tag != "" {
		where = append(where, `quotes.quote_id IN (SELECT quote_id FROM quote_tags WHERE tag = ?)`)
		args = append(args, filters.Tag)
	}
	if filters.Source != "" {
		where = append(where, `quotes.source_id IN (
			SELECT source_id FROM quote_sources WHERE provider = ?
		)`)
		args = append(args, filters.Source)
	}
	if filters.ProvenanceStatus != "" {
		where = append(where, `quotes.provenance_status = ?`)
		args = append(args, filters.ProvenanceStatus)
	}
	if filters.FreshnessStatus != "" {
		switch filters.FreshnessStatus {
		case "unknown":
			where = append(where, `(COALESCE(quotes.last_verified_at, '') = '' OR datetime(quotes.last_verified_at) IS NULL)`)
		case "stale":
			where = append(where, `datetime(quotes.last_verified_at) <= datetime('now', '-180 days')`)
		case "aging":
			where = append(where, `datetime(quotes.last_verified_at) > datetime('now', '-180 days') AND datetime(quotes.last_verified_at) <= datetime('now', '-90 days')`)
		case "fresh":
			where = append(where, `datetime(quotes.last_verified_at) > datetime('now', '-90 days')`)
		}
	}
	if len(where) > 0 {
		whereClause := " WHERE " + strings.Join(where, " AND ")
		queryParts = append(queryParts, whereClause)
		countParts = append(countParts, whereClause)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, strings.Join(countParts, ""), args...).Scan(&total); err != nil {
		return response, err
	}
	response.Pagination = models.Pagination{
		Limit:  normalizeLimit(filters.Limit),
		Offset: normalizeOffset(filters.Offset),
		Total:  total,
	}
	meta, err := s.Meta(ctx)
	if err != nil {
		return response, err
	}
	response.Meta = meta

	order := ` ORDER BY quotes.confidence_score DESC, quotes.last_verified_at DESC, artists.name ASC, quotes.quote_id ASC`
	if filters.Sort == "random" {
		order = ` ORDER BY RANDOM()`
	}
	queryParts = append(queryParts, order, ` LIMIT ? OFFSET ?`)
	args = append(args, response.Pagination.Limit, response.Pagination.Offset)
	rows, err := s.db.QueryContext(ctx, strings.Join(queryParts, ""), args...)
	if err != nil {
		return response, err
	}
	quotes := []models.Quote{}
	for rows.Next() {
		quote, err := scanQuoteRow(rows)
		if err != nil {
			_ = rows.Close()
			return response, err
		}
		quotes = append(quotes, quote)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return response, err
	}
	if err := rows.Close(); err != nil {
		return response, err
	}
	for idx := range quotes {
		if err := s.hydrateQuote(ctx, &quotes[idx]); err != nil {
			return response, err
		}
	}
	response.Data = quotes
	return response, nil
}

func (s *Store) RandomQuote(ctx context.Context, filters models.QuoteFilters) (*models.Quote, error) {
	filters.Sort = "random"
	filters.Limit = 1
	filters.Offset = 0
	response, err := s.ListQuotes(ctx, filters)
	if err != nil {
		return nil, err
	}
	if len(response.Data) == 0 {
		return nil, nil
	}
	return &response.Data[0], nil
}

func (s *Store) QuoteByID(ctx context.Context, quoteID string) (*models.Quote, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			quotes.quote_id,
			quotes.text,
			artists.artist_id,
			artists.name,
			quotes.source_id,
			quotes.source_type,
			quotes.work_title,
			quotes.year,
			quotes.provenance_status,
			quotes.confidence_score,
			quotes.provider_origin,
			quotes.license,
			quotes.first_seen_at,
			quotes.last_verified_at
		FROM quotes
		JOIN artists ON artists.artist_id = quotes.artist_id
		WHERE quotes.quote_id = ?
	`, quoteID)
	quote, err := scanQuoteRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if err := s.hydrateQuote(ctx, &quote); err != nil {
		return nil, err
	}
	return &quote, nil
}

func (s *Store) QuoteProvenance(ctx context.Context, quoteID string) (*models.QuoteProvenance, error) {
	quote, err := s.QuoteByID(ctx, quoteID)
	if err != nil {
		return nil, err
	}
	if quote == nil {
		return nil, nil
	}
	return &models.QuoteProvenance{
		QuoteID:          quote.QuoteID,
		ProvenanceStatus: quote.ProvenanceStatus,
		ConfidenceScore:  quote.ConfidenceScore,
		ProviderOrigin:   quote.ProviderOrigin,
		FirstSeenAt:      quote.FirstSeenAt,
		LastVerifiedAt:   quote.LastVerifiedAt,
		Evidence:         quote.Evidence,
		Source:           quote.Source,
	}, nil
}

func (s *Store) SimilarQuotes(ctx context.Context, quoteID string, threshold float64, limit int) (*models.ListResponse[models.SimilarQuote], error) {
	quote, err := s.QuoteByID(ctx, quoteID)
	if err != nil {
		return nil, err
	}
	if quote == nil {
		return nil, nil
	}
	limit = normalizeSimilarLimit(limit)

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			quotes.quote_id,
			quotes.text,
			artists.artist_id,
			artists.name,
			quotes.source_id,
			quotes.source_type,
			quotes.work_title,
			quotes.year,
			quotes.provenance_status,
			quotes.confidence_score,
			quotes.provider_origin,
			quotes.license,
			quotes.first_seen_at,
			quotes.last_verified_at
		FROM quotes
		JOIN artists ON artists.artist_id = quotes.artist_id
		WHERE quotes.quote_id <> ?
			AND NOT EXISTS (
				SELECT 1
				FROM quote_merge_log
				WHERE (winner_quote_id = ? AND loser_quote_id = quotes.quote_id)
					OR (winner_quote_id = quotes.quote_id AND loser_quote_id = ?)
			)
	`, quoteID, quoteID, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	matches := []models.SimilarQuote{}
	for rows.Next() {
		candidate, err := scanQuoteRow(rows)
		if err != nil {
			return nil, err
		}
		score := float64(search.QuoteMergeScore(quote.Text, candidate.Text)) / 100
		if score >= threshold {
			matches = append(matches, models.SimilarQuote{Quote: candidate, MergeScore: score})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].MergeScore == matches[j].MergeScore {
			return matches[i].Quote.QuoteID < matches[j].Quote.QuoteID
		}
		return matches[i].MergeScore > matches[j].MergeScore
	})

	total := len(matches)
	if len(matches) > limit {
		matches = matches[:limit]
	}
	for idx := range matches {
		if err := s.hydrateQuote(ctx, &matches[idx].Quote); err != nil {
			return nil, err
		}
	}
	meta, err := s.Meta(ctx)
	if err != nil {
		return nil, err
	}
	return &models.ListResponse[models.SimilarQuote]{
		Data: matches,
		Meta: meta,
		Pagination: models.Pagination{
			Limit:  limit,
			Offset: 0,
			Total:  total,
		},
	}, nil
}

func (s *Store) ArtistProvenanceSummary(ctx context.Context, artistID string) (*models.ArtistProvenanceSummary, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM artists WHERE artist_id = ?`, artistID).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT provenance_status, confidence_score, last_verified_at
		FROM quotes
		WHERE artist_id = ?
	`, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := &models.ArtistProvenanceSummary{
		ArtistID:            artistID,
		StatusCounts:        provenanceStatusCounts(),
		ConfidenceHistogram: make([]int, 10),
		RefreshHint:         "unknown",
	}
	now := time.Now().UTC()
	refreshRank := 0
	total := 0
	confidenceSum := 0.0
	for rows.Next() {
		var status, lastVerifiedAt string
		var confidence float64
		if err := rows.Scan(&status, &confidence, &lastVerifiedAt); err != nil {
			return nil, err
		}
		summary.StatusCounts[status]++
		summary.ConfidenceHistogram[confidenceBucket(confidence)]++
		confidenceSum += confidence
		total++
		refreshRank = maxInt(refreshRank, refreshHintRank(refreshHint(lastVerifiedAt, now)))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if total > 0 {
		summary.MeanConfidence = confidenceSum / float64(total)
		summary.RefreshHint = refreshHintFromRank(refreshRank)
	}
	return summary, nil
}

func (s *Store) ReviewQueue(ctx context.Context, filters models.ReviewQueueFilters) (models.ListResponse[models.ReviewQueueItem], error) {
	response := models.ListResponse[models.ReviewQueueItem]{}
	statuses := []string{"needs_review", "ambiguous", "provider_attributed"}
	var where []string
	var args []any
	if filters.ProvenanceStatus != "" {
		where = append(where, "quotes.provenance_status = ?")
		args = append(args, filters.ProvenanceStatus)
	} else {
		where = append(where, "quotes.provenance_status IN (?, ?, ?)")
		for _, status := range statuses {
			args = append(args, status)
		}
	}
	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}
	countQuery := `SELECT COUNT(*) FROM quotes` + whereClause
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&response.Pagination.Total); err != nil {
		return response, err
	}
	response.Pagination.Limit = normalizeLimit(filters.Limit)
	response.Pagination.Offset = normalizeOffset(filters.Offset)
	meta, err := s.Meta(ctx)
	if err != nil {
		return response, err
	}
	response.Meta = meta

	query := `
		SELECT quote_id, provenance_status, confidence_score
		FROM quotes` + whereClause + `
		ORDER BY
			CASE provenance_status
				WHEN 'needs_review' THEN 0
				WHEN 'ambiguous' THEN 1
				WHEN 'provider_attributed' THEN 2
				ELSE 3
			END ASC,
			confidence_score ASC,
			COALESCE(last_verified_at, '') ASC,
			quote_id ASC
		LIMIT ? OFFSET ?`
	queryArgs := append(append([]any{}, args...), response.Pagination.Limit, response.Pagination.Offset)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return response, err
	}
	type reviewCandidate struct {
		quoteID    string
		status     string
		confidence float64
	}
	candidates := []reviewCandidate{}
	for rows.Next() {
		var candidate reviewCandidate
		if err := rows.Scan(&candidate.quoteID, &candidate.status, &candidate.confidence); err != nil {
			_ = rows.Close()
			return response, err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return response, err
	}
	if err := rows.Close(); err != nil {
		return response, err
	}
	quoteIDs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		quoteIDs = append(quoteIDs, candidate.quoteID)
	}
	quotes, err := s.quotesByIDs(ctx, quoteIDs)
	if err != nil {
		return response, err
	}
	quotesByID := make(map[string]models.Quote, len(quotes))
	for _, quote := range quotes {
		quotesByID[quote.QuoteID] = quote
	}
	for _, candidate := range candidates {
		quote, ok := quotesByID[candidate.quoteID]
		if !ok {
			continue
		}
		response.Data = append(response.Data, models.ReviewQueueItem{
			Quote:     quote,
			Reason:    reviewReason(candidate.status, candidate.confidence),
			RiskScore: reviewRiskScore(candidate.status, candidate.confidence),
		})
	}
	return response, nil
}

func (s *Store) StaleQuotes(ctx context.Context, filters models.ReviewQueueFilters) (models.ListResponse[models.Quote], error) {
	response := models.ListResponse[models.Quote]{}
	where := `WHERE COALESCE(last_verified_at, '') = ''
		OR datetime(last_verified_at) IS NULL
		OR datetime(last_verified_at) <= datetime('now', '-180 days')`
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM quotes `+where).Scan(&response.Pagination.Total); err != nil {
		return response, err
	}
	response.Pagination.Limit = normalizeLimit(filters.Limit)
	response.Pagination.Offset = normalizeOffset(filters.Offset)
	meta, err := s.Meta(ctx)
	if err != nil {
		return response, err
	}
	response.Meta = meta
	rows, err := s.db.QueryContext(ctx, `
		SELECT quote_id
		FROM quotes
		`+where+`
		ORDER BY
			CASE WHEN COALESCE(last_verified_at, '') = '' OR datetime(last_verified_at) IS NULL THEN 0 ELSE 1 END ASC,
			COALESCE(last_verified_at, '') ASC,
			confidence_score ASC,
			quote_id ASC
		LIMIT ? OFFSET ?`, response.Pagination.Limit, response.Pagination.Offset)
	if err != nil {
		return response, err
	}
	quoteIDs := []string{}
	for rows.Next() {
		var quoteID string
		if err := rows.Scan(&quoteID); err != nil {
			_ = rows.Close()
			return response, err
		}
		quoteIDs = append(quoteIDs, quoteID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return response, err
	}
	if err := rows.Close(); err != nil {
		return response, err
	}
	quotes, err := s.quotesByIDs(ctx, quoteIDs)
	if err != nil {
		return response, err
	}
	response.Data = quotes
	return response, nil
}

func reviewReason(status string, confidence float64) string {
	switch {
	case status == "needs_review":
		return "explicitly flagged for editorial review"
	case status == "ambiguous":
		return "attribution is ambiguous across available evidence"
	case status == "provider_attributed":
		return "only provider-level attribution is available"
	case confidence < 0.5:
		return "low confidence score"
	default:
		return "manual review recommended"
	}
}

func reviewRiskScore(status string, confidence float64) float64 {
	base := map[string]float64{
		"needs_review":        1.0,
		"ambiguous":           0.85,
		"provider_attributed": 0.65,
	}[status]
	if base == 0 {
		base = 0.5
	}
	score := base + (1-confidence)*0.25
	if score > 1 {
		return 1
	}
	if score < 0 {
		return 0
	}
	return score
}

func (s *Store) ListArtists(ctx context.Context, filters models.ArtistFilters) (models.ListResponse[models.Artist], error) {
	response := models.ListResponse[models.Artist]{}
	base := `
		SELECT DISTINCT artists.artist_id, artists.name, COALESCE(artists.mbid, ''), COALESCE(artists.wikidata_id, ''),
			COALESCE(artists.wikiquote_title, ''), COALESCE(artists.country, ''), COALESCE(artists.life_span_begin, ''),
			COALESCE(artists.life_span_end, ''), COALESCE(artists.description, ''), COALESCE(artists.bio_summary, ''), COALESCE(artists.provider_status, '{}')
		FROM artists
	`
	var joins []string
	var where []string
	var args []any

	if filters.Tag != "" {
		joins = append(joins, `JOIN artist_tags ON artist_tags.artist_id = artists.artist_id`)
		where = append(where, `artist_tags.tag = ?`)
		args = append(args, filters.Tag)
	}
	if filters.MBID != "" {
		where = append(where, `artists.mbid = ?`)
		args = append(args, filters.MBID)
	}
	if filters.WikiquoteTitle != "" {
		where = append(where, `artists.wikiquote_title = ?`)
		args = append(args, filters.WikiquoteTitle)
	}
	if filters.Query != "" {
		pattern := "%" + search.NormalizeText(filters.Query) + "%"
		fts := ftsQuery(filters.Query)
		if fts != "" {
			where = append(where, `(artists.artist_id IN (
				SELECT artist_id FROM artist_search WHERE artist_search MATCH ?
			) OR artists.artist_id IN (
				SELECT artist_id FROM artist_aliases WHERE normalized_alias LIKE ?
			))`)
			args = append(args, fts, pattern)
		} else {
			where = append(where, `artists.artist_id IN (
				SELECT artist_id FROM artist_aliases WHERE normalized_alias LIKE ?
			)`)
			args = append(args, pattern)
		}
	}

	query := base
	if len(joins) > 0 {
		query += " " + strings.Join(joins, " ")
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	countQuery := `SELECT COUNT(*) FROM (` + query + `) AS artist_list`
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return response, err
	}

	response.Pagination = models.Pagination{
		Limit:  normalizeLimit(filters.Limit),
		Offset: normalizeOffset(filters.Offset),
		Total:  total,
	}
	meta, err := s.Meta(ctx)
	if err != nil {
		return response, err
	}
	response.Meta = meta

	query += ` ORDER BY artists.name ASC LIMIT ? OFFSET ?`
	args = append(args, response.Pagination.Limit, response.Pagination.Offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return response, err
	}
	artists := []models.Artist{}
	for rows.Next() {
		artist, err := scanArtistRow(rows)
		if err != nil {
			_ = rows.Close()
			return response, err
		}
		artists = append(artists, artist)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return response, err
	}
	if err := rows.Close(); err != nil {
		return response, err
	}
	for idx := range artists {
		if err := s.hydrateArtist(ctx, &artists[idx]); err != nil {
			return response, err
		}
	}
	response.Data = artists
	return response, nil
}

func (s *Store) ArtistByID(ctx context.Context, artistID string) (*models.Artist, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT artist_id, name, COALESCE(mbid, ''), COALESCE(wikidata_id, ''), COALESCE(wikiquote_title, ''), COALESCE(country, ''), COALESCE(life_span_begin, ''), COALESCE(life_span_end, ''), COALESCE(description, ''), COALESCE(bio_summary, ''), COALESCE(provider_status, '{}')
		FROM artists WHERE artist_id = ?
	`, artistID)
	artist, err := scanArtistRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if err := s.hydrateArtist(ctx, &artist); err != nil {
		return nil, err
	}
	return &artist, nil
}

func (s *Store) ResolveArtistID(ctx context.Context, input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", nil
	}
	normalized := search.NormalizeText(input)
	var artistID string
	err := s.db.QueryRowContext(ctx, `
		SELECT artist_id
		FROM artist_aliases
		WHERE normalized_alias = ?
		LIMIT 1
	`, normalized).Scan(&artistID)
	if err == nil {
		return artistID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	rows, err := s.db.QueryContext(ctx, `SELECT artist_id, alias FROM artist_aliases`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	bestArtistID := ""
	bestScore := 0
	for rows.Next() {
		var candidateID, alias string
		if err := rows.Scan(&candidateID, &alias); err != nil {
			return "", err
		}
		score := search.SimilarityScore(input, alias)
		if score > bestScore {
			bestScore = score
			bestArtistID = candidateID
		}
	}
	if bestScore >= 60 {
		return bestArtistID, nil
	}
	return "", nil
}

func (s *Store) ArtistQuotes(ctx context.Context, artistID string, filters models.QuoteFilters) (models.ListResponse[models.Quote], error) {
	filters.ArtistID = artistID
	return s.ListQuotes(ctx, filters)
}

func (s *Store) RelatedArtists(ctx context.Context, artistID string) ([]models.RelatedArtist, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT artists.artist_id, artists.name, artist_relations.relation_type, artist_relations.score, artist_relations.provider
		FROM artist_relations
		JOIN artists ON artists.artist_id = artist_relations.related_artist_id
		WHERE artist_relations.artist_id = ?
		ORDER BY artist_relations.score DESC, artists.name ASC
	`, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var related []models.RelatedArtist
	for rows.Next() {
		var relation models.RelatedArtist
		if err := rows.Scan(&relation.ArtistID, &relation.Name, &relation.Relation, &relation.Score, &relation.Provider); err != nil {
			return nil, err
		}
		related = append(related, relation)
	}
	return related, rows.Err()
}

func (s *Store) Releases(ctx context.Context, artistID string) ([]models.Release, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT release_id, title, year, kind, provider, url
		FROM releases
		WHERE artist_id = ?
		ORDER BY year DESC, title ASC
	`, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var releases []models.Release
	for rows.Next() {
		var release models.Release
		var year string
		if err := rows.Scan(&release.ReleaseID, &release.Title, &year, &release.Kind, &release.Provider, &release.URL); err != nil {
			return nil, err
		}
		release.Year = parseOptionalYear(year)
		releases = append(releases, release)
	}
	return releases, rows.Err()
}

func (s *Store) SourceByID(ctx context.Context, sourceID string) (*models.Source, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT source_id, provider, url, title, publisher, license, retrieved_at
		FROM quote_sources WHERE source_id = ?
	`, sourceID)
	var source models.Source
	if err := row.Scan(&source.SourceID, &source.Provider, &source.URL, &source.Title, &source.Publisher, &source.License, &source.RetrievedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &source, nil
}

func (s *Store) Search(ctx context.Context, query string) (models.SearchResponse, error) {
	return s.SearchWithLimit(ctx, query, 10)
}

func (s *Store) SearchWithLimit(ctx context.Context, query string, limit int) (models.SearchResponse, error) {
	response := models.SearchResponse{}
	unified, err := s.EntitySearch(ctx, query, []string{"artist", "quote"}, legacySearchCandidateLimit(limit))
	if err != nil {
		return response, err
	}
	response.Meta = unified.Meta
	artists, _, err := s.searchResultsFromHits(ctx, unified.Data.Hits)
	if err != nil {
		return response, err
	}
	limit = normalizeLimit(limit)
	quotes, err := s.searchQuotes(ctx, query, limit)
	if err != nil {
		return response, err
	}
	fallbackQuery := search.NormalizeText(query)
	if len(artists) == 0 && fallbackQuery != "" {
		fallback, err := s.ListArtists(ctx, models.ArtistFilters{Query: query, Limit: limit, Offset: 0})
		if err != nil {
			return response, err
		}
		artists = fallback.Data
	}
	if len(quotes) == 0 && fallbackQuery != "" {
		fallback, err := s.ListQuotes(ctx, models.QuoteFilters{Query: query, Limit: limit, Offset: 0})
		if err != nil {
			return response, err
		}
		quotes = fallback.Data
	}
	response.Data = models.SearchResults{
		Artists: artists,
		Quotes:  quotes,
	}
	return response, nil
}

func legacySearchCandidateLimit(limit int) int {
	if limit < 20 {
		return 20
	}
	return limit
}

func (s *Store) EntitySearch(ctx context.Context, query string, kinds []string, limit int) (models.EntitySearchResponse, error) {
	response := models.EntitySearchResponse{Data: models.EntitySearchResults{Hits: []models.EntitySearchHit{}}}
	meta, err := s.Meta(ctx)
	if err != nil {
		return response, err
	}
	response.Meta = meta
	fts := ftsQuery(query)
	if fts == "" {
		return response, nil
	}
	searchLimit := normalizeEntitySearchLimit(limit)
	enabled := entitySearchKindSet(kinds)
	weights := entitySearchWeights()
	hits := []models.EntitySearchHit{}
	if enabled["artist"] {
		artistHits, err := s.entitySearchArtists(ctx, fts, weights["artist"], searchLimit)
		if err != nil {
			return response, err
		}
		hits = append(hits, artistHits...)
	}
	if enabled["quote"] {
		quoteHits, err := s.entitySearchQuotes(ctx, fts, weights["quote"], searchLimit)
		if err != nil {
			return response, err
		}
		hits = append(hits, quoteHits...)
	}
	if enabled["work"] {
		workHits, err := s.entitySearchWorks(ctx, fts, weights["work"], searchLimit)
		if err != nil {
			return response, err
		}
		hits = append(hits, workHits...)
	}
	if enabled["recording"] {
		recordingHits, err := s.entitySearchRecordings(ctx, fts, weights["recording"], searchLimit)
		if err != nil {
			return response, err
		}
		hits = append(hits, recordingHits...)
	}
	if enabled["performance"] {
		performanceHits, err := s.entitySearchPerformances(ctx, fts, weights["performance"], searchLimit)
		if err != nil {
			return response, err
		}
		hits = append(hits, performanceHits...)
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			if hits[i].Kind == hits[j].Kind {
				return hits[i].ID < hits[j].ID
			}
			return hits[i].Kind < hits[j].Kind
		}
		return hits[i].Score > hits[j].Score
	})
	if len(hits) > searchLimit {
		hits = hits[:searchLimit]
	}
	response.Data.Hits = hits
	return response, nil
}

func (s *Store) searchResultsFromHits(ctx context.Context, hits []models.EntitySearchHit) ([]models.Artist, []models.Quote, error) {
	artists := []models.Artist{}
	quotes := []models.Quote{}
	seenArtists := map[string]bool{}
	seenQuotes := map[string]bool{}
	for _, hit := range hits {
		switch hit.Kind {
		case "artist":
			if seenArtists[hit.ID] {
				continue
			}
			artist, err := s.ArtistByID(ctx, hit.ID)
			if err != nil {
				return nil, nil, err
			}
			if artist != nil {
				artists = append(artists, *artist)
				seenArtists[hit.ID] = true
			}
		case "quote":
			if seenQuotes[hit.ID] {
				continue
			}
			quote, err := s.QuoteByID(ctx, hit.ID)
			if err != nil {
				return nil, nil, err
			}
			if quote != nil {
				quotes = append(quotes, *quote)
				seenQuotes[hit.ID] = true
			}
		}
	}
	return artists, quotes, nil
}

func (s *Store) Stats(ctx context.Context) (map[string]any, error) {
	meta, err := s.Meta(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := s.catalogCounts(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"artists":          counts["artists"],
		"quotes":           counts["quotes"],
		"sources":          counts["sources"],
		"releases":         counts["releases"],
		"related_artists":  counts["related_artists"],
		"jobs":             counts["jobs"],
		"works":            counts["works"],
		"recordings":       counts["recordings"],
		"samples":          counts["samples"],
		"work_credits":     counts["work_credits"],
		"performances":     counts["performances"],
		"claims":           counts["claims"],
		"claim_evidence":   counts["claim_evidence"],
		"quote_merges":     counts["quote_merges"],
		"snapshot_version": meta.SnapshotVersion,
		"active_providers": meta.ActiveProviders,
	}, nil
}

func (s *Store) IntegrityReport(ctx context.Context) (models.IntegrityReport, error) {
	report := models.IntegrityReport{
		OK:        true,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Counts:    map[string]int{},
		Issues:    []string{},
	}
	if err := s.db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&report.SQLite); err != nil {
		return report, err
	}
	if report.SQLite != "ok" {
		report.OK = false
		report.Issues = append(report.Issues, "sqlite_integrity_check_failed")
	}
	for _, check := range catalogIntegrityChecks {
		var count int
		if err := s.db.QueryRowContext(ctx, check.CountSQL).Scan(&count); err != nil {
			return report, err
		}
		report.Counts[check.Name] = count
		if count > 0 {
			report.OK = false
			report.Issues = append(report.Issues, check.Name)
		}
	}
	sort.Strings(report.Issues)
	return report, nil
}

func (s *Store) RepairOrphans(ctx context.Context, apply bool) (OrphanRepairResult, error) {
	result := OrphanRepairResult{
		Applied: apply,
		Counts:  map[string]int{},
		Targets: map[string][]string{},
	}
	if !apply {
		for _, rule := range orphanRepairRules {
			targets, err := orphanTargets(ctx, s.db, rule.SelectSQL)
			if err != nil {
				return result, err
			}
			result.Counts[rule.Kind] = len(targets)
			result.Targets[rule.Kind] = targets
		}
		return result, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, rule := range orphanRepairRules {
		targets, err := orphanTargets(ctx, tx, rule.SelectSQL)
		if err != nil {
			_ = tx.Rollback()
			return result, err
		}
		result.Counts[rule.Kind] = len(targets)
		result.Targets[rule.Kind] = targets
		if len(targets) == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, rule.DeleteSQL); err != nil {
			_ = tx.Rollback()
			return result, err
		}
		for idx, target := range targets {
			eventID := search.StableHash("repair-orphan", rule.Kind, target, now, strconv.Itoa(idx))
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO ingestion_audit_events(event_id, provider, target, action, status, occurred_at, details)
				VALUES(?, 'catalog_repair', ?, ?, 'succeeded', ?, ?)
			`, eventID, target, "repair_orphan_"+rule.Kind, now, "deleted orphan row"); err != nil {
				_ = tx.Rollback()
				return result, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

type catalogIntegrityCheck struct {
	Name     string
	CountSQL string
}

type orphanRepairRule struct {
	Kind      string
	SelectSQL string
	DeleteSQL string
}

type orphanTargetQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

const claimSubjectMissingPredicate = `(claims.subject_type = 'artist' AND NOT EXISTS (SELECT 1 FROM artists WHERE artists.artist_id = claims.subject_id))
	OR (claims.subject_type = 'quote' AND NOT EXISTS (SELECT 1 FROM quotes WHERE quotes.quote_id = claims.subject_id))
	OR (claims.subject_type = 'recording' AND NOT EXISTS (SELECT 1 FROM recordings WHERE recordings.recording_id = claims.subject_id))
	OR (claims.subject_type = 'work' AND NOT EXISTS (SELECT 1 FROM works WHERE works.work_id = claims.subject_id))
	OR (claims.subject_type = 'work_credit' AND NOT EXISTS (SELECT 1 FROM work_credits WHERE work_credits.credit_id = claims.subject_id))
	OR (claims.subject_type = 'performance' AND NOT EXISTS (SELECT 1 FROM performances WHERE performances.performance_id = claims.subject_id))
	OR (claims.subject_type = 'sample' AND NOT EXISTS (SELECT 1 FROM samples WHERE samples.sample_id = claims.subject_id))
	OR claims.subject_type NOT IN ('artist', 'quote', 'recording', 'work', 'work_credit', 'performance', 'sample')`

const claimObjectMissingPredicate = `(claims.object_type = 'artist' AND NOT EXISTS (SELECT 1 FROM artists WHERE artists.artist_id = claims.object_id))
	OR (claims.object_type = 'quote' AND NOT EXISTS (SELECT 1 FROM quotes WHERE quotes.quote_id = claims.object_id))
	OR (claims.object_type = 'recording' AND NOT EXISTS (SELECT 1 FROM recordings WHERE recordings.recording_id = claims.object_id))
	OR (claims.object_type = 'work' AND NOT EXISTS (SELECT 1 FROM works WHERE works.work_id = claims.object_id))
	OR (claims.object_type = 'work_credit' AND NOT EXISTS (SELECT 1 FROM work_credits WHERE work_credits.credit_id = claims.object_id))
	OR (claims.object_type = 'performance' AND NOT EXISTS (SELECT 1 FROM performances WHERE performances.performance_id = claims.object_id))
	OR (claims.object_type = 'sample' AND NOT EXISTS (SELECT 1 FROM samples WHERE samples.sample_id = claims.object_id))
	OR claims.object_type NOT IN ('artist', 'quote', 'recording', 'work', 'work_credit', 'performance', 'sample')`

var catalogIntegrityChecks = []catalogIntegrityCheck{ // #nosec G101 -- names are integrity check identifiers
	{Name: "quotes_missing_artist", CountSQL: `SELECT COUNT(*) FROM quotes LEFT JOIN artists ON artists.artist_id = quotes.artist_id WHERE artists.artist_id IS NULL`},
	{Name: "quotes_missing_source", CountSQL: `SELECT COUNT(*) FROM quotes WHERE source_id <> '' AND source_id NOT IN (SELECT source_id FROM quote_sources)`},
	{Name: "tags_missing_quote", CountSQL: `SELECT COUNT(*) FROM quote_tags LEFT JOIN quotes ON quotes.quote_id = quote_tags.quote_id WHERE quotes.quote_id IS NULL`},
	{Name: "evidence_missing_quote", CountSQL: `SELECT COUNT(*) FROM quote_evidence
		LEFT JOIN quotes ON quotes.quote_id = quote_evidence.quote_id
		WHERE quotes.quote_id IS NULL`},
	{Name: "job_items_missing_job", CountSQL: `SELECT COUNT(*) FROM job_items LEFT JOIN jobs ON jobs.job_id = job_items.job_id WHERE jobs.job_id IS NULL`},
	{Name: "recordings_missing_artist", CountSQL: `SELECT COUNT(*) FROM recordings LEFT JOIN artists ON artists.artist_id = recordings.artist_id WHERE artists.artist_id IS NULL`},
	{Name: "recordings_missing_work", CountSQL: `SELECT COUNT(*) FROM recordings WHERE work_id <> '' AND work_id NOT IN (SELECT work_id FROM works)`},
	{Name: "samples_missing_source", CountSQL: `SELECT COUNT(*) FROM samples LEFT JOIN recordings ON recordings.recording_id = samples.source_recording_id WHERE recordings.recording_id IS NULL`},
	{Name: "samples_missing_derivative", CountSQL: `SELECT COUNT(*) FROM samples LEFT JOIN recordings ON recordings.recording_id = samples.derivative_recording_id WHERE recordings.recording_id IS NULL`},
	{Name: "credits_missing_work", CountSQL: `SELECT COUNT(*) FROM work_credits LEFT JOIN works ON works.work_id = work_credits.work_id WHERE works.work_id IS NULL`},
	{Name: "performances_missing_artist", CountSQL: `SELECT COUNT(*) FROM performances LEFT JOIN artists ON artists.artist_id = performances.artist_id WHERE artists.artist_id IS NULL`},
	{Name: "performances_missing_work", CountSQL: `SELECT COUNT(*) FROM performances WHERE work_id <> '' AND work_id NOT IN (SELECT work_id FROM works)`},
	{Name: "performances_missing_recording", CountSQL: `SELECT COUNT(*) FROM performances WHERE recording_id <> '' AND recording_id NOT IN (SELECT recording_id FROM recordings)`},
	{Name: "claims_missing_subject", CountSQL: `SELECT COUNT(*) FROM claims WHERE ` + claimSubjectMissingPredicate},
	{Name: "claims_missing_object", CountSQL: `SELECT COUNT(*) FROM claims WHERE ` + claimObjectMissingPredicate},
	{Name: "claim_evidence_missing_claim", CountSQL: `SELECT COUNT(*) FROM claim_evidence LEFT JOIN claims ON claims.claim_id = claim_evidence.claim_id WHERE claims.claim_id IS NULL`},
}

var orphanRepairRules = []orphanRepairRule{ // #nosec G101 -- names are repair identifiers
	{
		Kind:      "tags_missing_quote",
		SelectSQL: `SELECT quote_id || ':' || tag FROM quote_tags WHERE quote_id NOT IN (SELECT quote_id FROM quotes) ORDER BY quote_id, tag`,
		DeleteSQL: `DELETE FROM quote_tags WHERE quote_id NOT IN (SELECT quote_id FROM quotes)`,
	},
	{
		Kind:      "evidence_missing_quote",
		SelectSQL: `SELECT quote_id || ':' || position FROM quote_evidence WHERE quote_id NOT IN (SELECT quote_id FROM quotes) ORDER BY quote_id, position`,
		DeleteSQL: `DELETE FROM quote_evidence WHERE quote_id NOT IN (SELECT quote_id FROM quotes)`,
	},
	{
		Kind:      "job_items_missing_job",
		SelectSQL: `SELECT job_item_id FROM job_items WHERE job_id NOT IN (SELECT job_id FROM jobs) ORDER BY job_item_id`,
		DeleteSQL: `DELETE FROM job_items WHERE job_id NOT IN (SELECT job_id FROM jobs)`,
	},
	{
		Kind:      "claim_evidence_missing_claim",
		SelectSQL: `SELECT evidence_id FROM claim_evidence WHERE claim_id NOT IN (SELECT claim_id FROM claims) ORDER BY evidence_id`,
		DeleteSQL: `DELETE FROM claim_evidence WHERE claim_id NOT IN (SELECT claim_id FROM claims)`,
	},
	{
		Kind:      "samples_missing_source",
		SelectSQL: `SELECT sample_id FROM samples WHERE source_recording_id NOT IN (SELECT recording_id FROM recordings) ORDER BY sample_id`,
		DeleteSQL: `DELETE FROM samples WHERE source_recording_id NOT IN (SELECT recording_id FROM recordings)`,
	},
	{
		Kind:      "samples_missing_derivative",
		SelectSQL: `SELECT sample_id FROM samples WHERE derivative_recording_id NOT IN (SELECT recording_id FROM recordings) ORDER BY sample_id`,
		DeleteSQL: `DELETE FROM samples WHERE derivative_recording_id NOT IN (SELECT recording_id FROM recordings)`,
	},
	{
		Kind:      "quotes_missing_artist",
		SelectSQL: `SELECT quote_id FROM quotes WHERE artist_id NOT IN (SELECT artist_id FROM artists) ORDER BY quote_id`,
		DeleteSQL: `DELETE FROM quotes WHERE artist_id NOT IN (SELECT artist_id FROM artists)`,
	},
	{
		Kind:      "quotes_missing_source",
		SelectSQL: `SELECT quote_id FROM quotes WHERE source_id <> '' AND source_id NOT IN (SELECT source_id FROM quote_sources) ORDER BY quote_id`,
		DeleteSQL: `DELETE FROM quotes WHERE source_id <> '' AND source_id NOT IN (SELECT source_id FROM quote_sources)`,
	},
	{
		Kind:      "recordings_missing_artist",
		SelectSQL: `SELECT recording_id FROM recordings WHERE artist_id NOT IN (SELECT artist_id FROM artists) ORDER BY recording_id`,
		DeleteSQL: `DELETE FROM recordings WHERE artist_id NOT IN (SELECT artist_id FROM artists)`,
	},
	{
		Kind:      "recordings_missing_work",
		SelectSQL: `SELECT recording_id FROM recordings WHERE work_id <> '' AND work_id NOT IN (SELECT work_id FROM works) ORDER BY recording_id`,
		DeleteSQL: `DELETE FROM recordings WHERE work_id <> '' AND work_id NOT IN (SELECT work_id FROM works)`,
	},
	{
		Kind:      "credits_missing_work",
		SelectSQL: `SELECT credit_id FROM work_credits WHERE work_id NOT IN (SELECT work_id FROM works) ORDER BY credit_id`,
		DeleteSQL: `DELETE FROM work_credits WHERE work_id NOT IN (SELECT work_id FROM works)`,
	},
	{
		Kind:      "performances_missing_artist",
		SelectSQL: `SELECT performance_id FROM performances WHERE artist_id NOT IN (SELECT artist_id FROM artists) ORDER BY performance_id`,
		DeleteSQL: `DELETE FROM performances WHERE artist_id NOT IN (SELECT artist_id FROM artists)`,
	},
	{
		Kind:      "performances_missing_work",
		SelectSQL: `SELECT performance_id FROM performances WHERE work_id <> '' AND work_id NOT IN (SELECT work_id FROM works) ORDER BY performance_id`,
		DeleteSQL: `DELETE FROM performances WHERE work_id <> '' AND work_id NOT IN (SELECT work_id FROM works)`,
	},
	{
		Kind:      "performances_missing_recording",
		SelectSQL: `SELECT performance_id FROM performances WHERE recording_id <> '' AND recording_id NOT IN (SELECT recording_id FROM recordings) ORDER BY performance_id`,
		DeleteSQL: `DELETE FROM performances WHERE recording_id <> '' AND recording_id NOT IN (SELECT recording_id FROM recordings)`,
	},
	{
		Kind:      "claims_missing_subject",
		SelectSQL: `SELECT claim_id FROM claims WHERE ` + claimSubjectMissingPredicate + ` ORDER BY claim_id`,
		DeleteSQL: `DELETE FROM claims WHERE ` + claimSubjectMissingPredicate,
	},
	{
		Kind:      "claims_missing_object",
		SelectSQL: `SELECT claim_id FROM claims WHERE ` + claimObjectMissingPredicate + ` ORDER BY claim_id`,
		DeleteSQL: `DELETE FROM claims WHERE ` + claimObjectMissingPredicate,
	},
}

func orphanTargets(ctx context.Context, querier orphanTargetQuerier, query string) ([]string, error) {
	rows, err := querier.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := []string{}
	for rows.Next() {
		var target string
		if err := rows.Scan(&target); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (s *Store) ProviderRuns(ctx context.Context, provider string, limit int) ([]models.ProviderRun, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, provider, status, started_at, finished_at, details
		FROM provider_runs
		WHERE provider = ?
		ORDER BY started_at DESC
		LIMIT ?
	`, provider, normalizeLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []models.ProviderRun
	for rows.Next() {
		var run models.ProviderRun
		if err := rows.Scan(&run.RunID, &run.Provider, &run.Status, &run.StartedAt, &run.FinishedAt, &run.Details); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Store) ProviderErrors(ctx context.Context, provider string, limit int) ([]models.ProviderError, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT error_id, provider, error_kind, occurred_at, context, message
		FROM provider_errors
		WHERE provider = ?
		ORDER BY occurred_at DESC
		LIMIT ?
	`, provider, normalizeLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var failures []models.ProviderError
	for rows.Next() {
		var failure models.ProviderError
		if err := rows.Scan(&failure.ErrorID, &failure.Provider, &failure.Kind, &failure.OccurredAt, &failure.Context, &failure.Message); err != nil {
			return nil, err
		}
		failures = append(failures, failure)
	}
	return failures, rows.Err()
}

func (s *Store) SetProviderCooldown(ctx context.Context, provider string, until time.Time, reason string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO provider_cooldowns(provider, until, reason, updated_at)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(provider) DO UPDATE SET
			until = excluded.until,
			reason = excluded.reason,
			updated_at = excluded.updated_at
	`, provider, until.UTC().Format(time.RFC3339), reason, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) ProviderCooldown(ctx context.Context, provider string, now time.Time) (*models.ProviderCooldown, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT provider, until, reason, updated_at
		FROM provider_cooldowns
		WHERE provider = ?
	`, provider)
	var cooldown models.ProviderCooldown
	if err := row.Scan(&cooldown.Provider, &cooldown.Until, &cooldown.Reason, &cooldown.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	until, err := time.Parse(time.RFC3339, cooldown.Until)
	if err != nil {
		return &cooldown, false, nil
	}
	return &cooldown, now.UTC().Before(until), nil
}

func (s *Store) ProviderSummaries(ctx context.Context, configured []models.ProviderSummary) ([]models.ProviderSummary, error) {
	summaries := make([]models.ProviderSummary, 0, len(configured))
	for _, item := range configured {
		summary := item
		for _, loader := range providerSummaryLoaders {
			if err := loader.load(ctx, s, item.Provider, &summary); err != nil {
				slog.Warn("provider_summary_field_failed", "provider", item.Provider, "field", loader.field, "err", err)
				return nil, fmt.Errorf("provider summary %s %s: %w", item.Provider, loader.field, err)
			}
		}
		if cooldown, active, err := s.ProviderCooldown(ctx, item.Provider, time.Now().UTC()); err != nil {
			slog.Warn("provider_summary_field_failed", "provider", item.Provider, "field", "cooldown", "err", err)
			return nil, fmt.Errorf("provider summary %s cooldown: %w", item.Provider, err)
		} else if active {
			summary.CooldownUntil = cooldown.Until
			summary.CooldownReason = cooldown.Reason
		}
		summaries = append(summaries, summary)
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].Provider < summaries[j].Provider
	})
	return summaries, nil
}

type providerSummaryLoader struct {
	field string
	load  func(context.Context, *Store, string, *models.ProviderSummary) error
}

var providerSummaryLoaders = []providerSummaryLoader{
	{field: "last_status", load: loadProviderLastStatus},
	{field: "last_successful", load: loadProviderLastSuccessful},
	{field: "recent_error_count", load: loadProviderRecentErrorCount},
	{field: "last_error_at", load: loadProviderLastErrorAt},
}

func loadProviderLastStatus(ctx context.Context, s *Store, provider string, summary *models.ProviderSummary) error {
	var finishedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT status, finished_at
		FROM provider_runs
		WHERE provider = ?
		ORDER BY started_at DESC
		LIMIT 1
	`, provider).Scan(&summary.LastStatus, &finishedAt)
	return ignoreProviderSummaryNoRows(err)
}

func loadProviderLastSuccessful(ctx context.Context, s *Store, provider string, summary *models.ProviderSummary) error {
	err := s.db.QueryRowContext(ctx, `
		SELECT finished_at
		FROM provider_runs
		WHERE provider = ? AND status = 'success'
		ORDER BY finished_at DESC
		LIMIT 1
	`, provider).Scan(&summary.LastSuccessful)
	return ignoreProviderSummaryNoRows(err)
}

func loadProviderRecentErrorCount(ctx context.Context, s *Store, provider string, summary *models.ProviderSummary) error {
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM provider_errors
		WHERE provider = ? AND occurred_at >= ?
	`, provider, time.Now().UTC().Add(-30*24*time.Hour).Format(time.RFC3339)).Scan(&summary.RecentErrorCount)
	return ignoreProviderSummaryNoRows(err)
}

func loadProviderLastErrorAt(ctx context.Context, s *Store, provider string, summary *models.ProviderSummary) error {
	err := s.db.QueryRowContext(ctx, `
		SELECT occurred_at
		FROM provider_errors
		WHERE provider = ?
		ORDER BY occurred_at DESC
		LIMIT 1
	`, provider).Scan(&summary.LastErrorAt)
	return ignoreProviderSummaryNoRows(err)
}

func ignoreProviderSummaryNoRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

func (s *Store) GetProviderCache(ctx context.Context, provider, kind, key string) (string, string, string, bool, error) {
	var payload, refreshedAt, expiresAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT payload, refreshed_at, expires_at
		FROM provider_cache
		WHERE provider = ? AND kind = ? AND cache_key = ? AND expires_at >= ?
	`, provider, kind, key, time.Now().UTC().Format(time.RFC3339)).Scan(&payload, &refreshedAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", false, nil
	}
	if err != nil {
		return "", "", "", false, err
	}
	return payload, refreshedAt, expiresAt, true, nil
}

func (s *Store) SetProviderCache(ctx context.Context, provider, kind, key, payload string, ttl time.Duration) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO provider_cache(provider, kind, cache_key, payload, refreshed_at, expires_at)
		VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider, kind, cache_key) DO UPDATE SET
			payload = excluded.payload,
			refreshed_at = excluded.refreshed_at,
			expires_at = excluded.expires_at
	`, provider, kind, key, payload, now.Format(time.RFC3339), now.Add(ttl).Format(time.RFC3339))
	return err
}

func (s *Store) DeleteProviderCache(ctx context.Context, provider, kind, key string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM provider_cache
		WHERE provider = ? AND kind = ? AND cache_key = ?
	`, provider, kind, key)
	return err
}

func (s *Store) PurgeExpiredProviderCache(ctx context.Context, now time.Time) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM provider_cache
		WHERE expires_at < ?
	`, now.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(removed), nil
}

func (s *Store) RecordJob(ctx context.Context, job models.JobRun) error {
	var previousStatus string
	if err := s.db.QueryRowContext(ctx, `SELECT status FROM jobs WHERE job_id = ?`, job.JobID).Scan(&previousStatus); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jobs(job_id, name, scope, status, started_at, finished_at, details, error_message)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_id) DO UPDATE SET
			name = excluded.name,
			scope = excluded.scope,
			status = excluded.status,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			details = excluded.details,
			error_message = excluded.error_message
	`, job.JobID, job.Name, job.Scope, job.Status, job.StartedAt, job.FinishedAt, job.Details, job.ErrorMessage)
	if err != nil {
		return err
	}
	if isTerminalJobStatus(job.Status) && previousStatus != job.Status {
		s.emitWebhookEvent(ctx, models.WebhookEvent{
			EventID:    search.StableHash("webhook", "job.completed", job.JobID, job.Status, job.FinishedAt),
			Kind:       "job.completed",
			OccurredAt: webhookTimestamp(job.FinishedAt),
			Data:       job,
		})
	}
	return nil
}

func (s *Store) RecordJobItem(ctx context.Context, item models.JobItem) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO job_items(job_item_id, job_id, provider, target, status, started_at, finished_at, details, error_message)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_item_id) DO UPDATE SET
			job_id = excluded.job_id,
			provider = excluded.provider,
			target = excluded.target,
			status = excluded.status,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			details = excluded.details,
			error_message = excluded.error_message
	`, item.JobItemID, item.JobID, item.Provider, item.Target, item.Status, item.StartedAt, item.FinishedAt, item.Details, item.ErrorMessage)
	return err
}

func (s *Store) CaptureIngestionSnapshot(ctx context.Context, jobID, phase string, capturedAt time.Time) (models.IngestionSnapshot, error) {
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	counts, err := s.catalogCounts(ctx)
	if err != nil {
		return models.IngestionSnapshot{}, err
	}
	snapshot := models.IngestionSnapshot{
		SnapshotID: search.StableHash("ingestion-snapshot", jobID, phase, capturedAt.Format(time.RFC3339Nano)),
		JobID:      jobID,
		Phase:      phase,
		CapturedAt: capturedAt.Format(time.RFC3339),
		Counts:     counts,
	}
	countsJSON, err := json.Marshal(snapshot.Counts)
	if err != nil {
		return models.IngestionSnapshot{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO ingestion_snapshots(snapshot_id, job_id, phase, captured_at, counts_json)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(snapshot_id) DO UPDATE SET
			job_id = excluded.job_id,
			phase = excluded.phase,
			captured_at = excluded.captured_at,
			counts_json = excluded.counts_json
	`, snapshot.SnapshotID, snapshot.JobID, snapshot.Phase, snapshot.CapturedAt, string(countsJSON))
	if err != nil {
		return models.IngestionSnapshot{}, err
	}
	return snapshot, nil
}

func (s *Store) ListIngestionSnapshots(ctx context.Context, jobID string, limit int) ([]models.IngestionSnapshot, error) {
	query := `SELECT snapshot_id, job_id, phase, captured_at, counts_json FROM ingestion_snapshots`
	args := []any{}
	if jobID != "" {
		query += ` WHERE job_id = ?`
		args = append(args, jobID)
	}
	query += ` ORDER BY captured_at DESC, snapshot_id DESC LIMIT ?`
	args = append(args, normalizeLimit(limit))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var snapshots []models.IngestionSnapshot
	for rows.Next() {
		var snapshot models.IngestionSnapshot
		var countsJSON string
		if err := rows.Scan(&snapshot.SnapshotID, &snapshot.JobID, &snapshot.Phase, &snapshot.CapturedAt, &countsJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(countsJSON), &snapshot.Counts); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, rows.Err()
}

func (s *Store) RecordIngestionAuditEvent(ctx context.Context, event models.IngestionAuditEvent) error {
	if event.EventID == "" {
		event.EventID = search.StableHash("ingestion-audit", event.JobID, event.JobItemID, event.Provider, event.Target, event.Action, event.Status, event.OccurredAt, event.Details)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ingestion_audit_events(event_id, job_id, job_item_id, provider, target, action, status, occurred_at, details)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(event_id) DO UPDATE SET
			job_id = excluded.job_id,
			job_item_id = excluded.job_item_id,
			provider = excluded.provider,
			target = excluded.target,
			action = excluded.action,
			status = excluded.status,
			occurred_at = excluded.occurred_at,
			details = excluded.details
	`, event.EventID, event.JobID, event.JobItemID, event.Provider, event.Target, event.Action, event.Status, event.OccurredAt, event.Details)
	return err
}

func (s *Store) ListIngestionAuditEvents(ctx context.Context, jobID string, limit int) ([]models.IngestionAuditEvent, error) {
	query := `SELECT event_id, job_id, job_item_id, provider, target, action, status, occurred_at, details FROM ingestion_audit_events`
	args := []any{}
	if jobID != "" {
		query += ` WHERE job_id = ?`
		args = append(args, jobID)
	}
	query += ` ORDER BY occurred_at DESC, event_id DESC LIMIT ?`
	args = append(args, normalizeLimit(limit))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []models.IngestionAuditEvent
	for rows.Next() {
		var event models.IngestionAuditEvent
		if err := rows.Scan(&event.EventID, &event.JobID, &event.JobItemID, &event.Provider, &event.Target, &event.Action, &event.Status, &event.OccurredAt, &event.Details); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) catalogCounts(ctx context.Context) (map[string]int, error) {
	counts := map[string]int{}
	for key, query := range map[string]string{ // #nosec G101 -- keys are table names
		"artists":         `SELECT COUNT(*) FROM artists`,
		"quotes":          `SELECT COUNT(*) FROM quotes`,
		"sources":         `SELECT COUNT(*) FROM quote_sources`,
		"releases":        `SELECT COUNT(*) FROM releases`,
		"related_artists": `SELECT COUNT(*) FROM artist_relations`,
		"jobs":            `SELECT COUNT(*) FROM jobs`,
		"works":           `SELECT COUNT(*) FROM works`,
		"recordings":      `SELECT COUNT(*) FROM recordings`,
		"samples":         `SELECT COUNT(*) FROM samples`,
		"work_credits":    `SELECT COUNT(*) FROM work_credits`,
		"performances":    `SELECT COUNT(*) FROM performances`,
		"claims":          `SELECT COUNT(*) FROM claims`,
		"claim_evidence":  `SELECT COUNT(*) FROM claim_evidence`,
		"quote_merges":    `SELECT COUNT(*) FROM quote_merge_log`,
	} {
		var count int
		if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			return nil, err
		}
		counts[key] = count
	}
	return counts, nil
}

func (s *Store) ListJobs(ctx context.Context, limit int) ([]models.JobRun, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT job_id, name, scope, status, started_at, finished_at, details, error_message
		FROM jobs
		ORDER BY started_at DESC
		LIMIT ?
	`, normalizeLimit(limit))
	if err != nil {
		return nil, err
	}
	var jobs []models.JobRun
	for rows.Next() {
		var job models.JobRun
		if err := rows.Scan(&job.JobID, &job.Name, &job.Scope, &job.Status, &job.StartedAt, &job.FinishedAt, &job.Details, &job.ErrorMessage); err != nil {
			_ = rows.Close()
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for idx := range jobs {
		jobs[idx].Items, _ = s.jobItems(ctx, jobs[idx].JobID)
	}
	return jobs, nil
}

func (s *Store) JobByID(ctx context.Context, jobID string) (*models.JobRun, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT job_id, name, scope, status, started_at, finished_at, details, error_message
		FROM jobs
		WHERE job_id = ?
	`, jobID)
	var job models.JobRun
	if err := row.Scan(&job.JobID, &job.Name, &job.Scope, &job.Status, &job.StartedAt, &job.FinishedAt, &job.Details, &job.ErrorMessage); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	job.Items, _ = s.jobItems(ctx, job.JobID)
	return &job, nil
}

func (s *Store) jobItems(ctx context.Context, jobID string) ([]models.JobItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT job_item_id, job_id, provider, target, status, started_at, finished_at, details, error_message
		FROM job_items
		WHERE job_id = ?
		ORDER BY started_at ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.JobItem
	for rows.Next() {
		var item models.JobItem
		if err := rows.Scan(&item.JobItemID, &item.JobID, &item.Provider, &item.Target, &item.Status, &item.StartedAt, &item.FinishedAt, &item.Details, &item.ErrorMessage); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) searchQuotes(ctx context.Context, query string, limit int) ([]models.Quote, error) {
	fts := ftsQuery(query)
	if fts == "" {
		return nil, nil
	}
	normalized := search.NormalizeText(query)
	rows, err := s.db.QueryContext(ctx, `
		SELECT quote_search.quote_id
		FROM quote_search
		JOIN quotes ON quotes.quote_id = quote_search.quote_id
		JOIN artists ON artists.artist_id = quotes.artist_id
		WHERE quote_search MATCH ?
		ORDER BY
			CASE
				WHEN quotes.normalized_text = ? THEN 0
				WHEN quotes.normalized_text LIKE ? THEN 1
				WHEN lower(artists.name) = ? THEN 2
				ELSE 3
			END ASC,
			bm25(quote_search) ASC,
			CASE quotes.provenance_status
				WHEN 'verified' THEN 0
				WHEN 'source_attributed' THEN 1
				WHEN 'provider_attributed' THEN 2
				WHEN 'ambiguous' THEN 3
				WHEN 'needs_review' THEN 4
				ELSE 5
			END ASC,
			quotes.confidence_score DESC,
			COALESCE(quotes.last_verified_at, '') DESC,
			quotes.quote_id ASC
		LIMIT ?
	`, fts, normalized, normalized+"%", strings.ToLower(strings.TrimSpace(query)), limit)
	if err != nil {
		return nil, err
	}
	quoteIDs := []string{}
	for rows.Next() {
		var quoteID string
		if err := rows.Scan(&quoteID); err != nil {
			_ = rows.Close()
			return nil, err
		}
		quoteIDs = append(quoteIDs, quoteID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return s.quotesByIDs(ctx, quoteIDs)
}

func (s *Store) entitySearchArtists(ctx context.Context, fts string, weight float64, limit int) ([]models.EntitySearchHit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT artist_id, name, snippet(artist_search, 1, '', '', '...', 12), bm25(artist_search)
		FROM artist_search
		WHERE artist_search MATCH ?
		ORDER BY bm25(artist_search), artist_id
		LIMIT ?
	`, fts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := []models.EntitySearchHit{}
	for rows.Next() {
		var id, label, snippet string
		var rank float64
		if err := rows.Scan(&id, &label, &snippet, &rank); err != nil {
			return nil, err
		}
		hits = append(hits, entitySearchHit("artist", id, label, snippet, rank, weight))
	}
	return hits, rows.Err()
}

func (s *Store) entitySearchQuotes(ctx context.Context, fts string, weight float64, limit int) ([]models.EntitySearchHit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT quote_search.quote_id, quotes.text, snippet(quote_search, 3, '', '', '...', 12), bm25(quote_search)
		FROM quote_search
		JOIN quotes ON quotes.quote_id = quote_search.quote_id
		WHERE quote_search MATCH ?
		ORDER BY bm25(quote_search), quote_search.quote_id
		LIMIT ?
	`, fts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := []models.EntitySearchHit{}
	for rows.Next() {
		var id, label, snippet string
		var rank float64
		if err := rows.Scan(&id, &label, &snippet, &rank); err != nil {
			return nil, err
		}
		hits = append(hits, entitySearchHit("quote", id, truncate(label, 96), snippet, rank, weight))
	}
	return hits, rows.Err()
}

func (s *Store) entitySearchWorks(ctx context.Context, fts string, weight float64, limit int) ([]models.EntitySearchHit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT work_id, title, snippet(work_search, 1, '', '', '...', 12), bm25(work_search)
		FROM work_search
		WHERE work_search MATCH ?
		ORDER BY bm25(work_search), work_id
		LIMIT ?
	`, fts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := []models.EntitySearchHit{}
	for rows.Next() {
		var id, label, snippet string
		var rank float64
		if err := rows.Scan(&id, &label, &snippet, &rank); err != nil {
			return nil, err
		}
		hits = append(hits, entitySearchHit("work", id, label, snippet, rank, weight))
	}
	return hits, rows.Err()
}

func (s *Store) entitySearchRecordings(ctx context.Context, fts string, weight float64, limit int) ([]models.EntitySearchHit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT recording_id,
			CASE WHEN artist_name <> '' THEN artist_name || ' - ' || title ELSE title END,
			snippet(recording_search, 3, '', '', '...', 12),
			bm25(recording_search)
		FROM recording_search
		WHERE recording_search MATCH ?
		ORDER BY bm25(recording_search), recording_id
		LIMIT ?
	`, fts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := []models.EntitySearchHit{}
	for rows.Next() {
		var id, label, snippet string
		var rank float64
		if err := rows.Scan(&id, &label, &snippet, &rank); err != nil {
			return nil, err
		}
		hits = append(hits, entitySearchHit("recording", id, label, snippet, rank, weight))
	}
	return hits, rows.Err()
}

func (s *Store) entitySearchPerformances(ctx context.Context, fts string, weight float64, limit int) ([]models.EntitySearchHit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT performance_id,
			trim(artist_name || ' @ ' || CASE WHEN event_name <> '' THEN event_name WHEN venue <> '' THEN venue ELSE city END || ' ' || performed_at),
			snippet(performance_search, 4, '', '', '...', 12),
			bm25(performance_search)
		FROM performance_search
		WHERE performance_search MATCH ?
		ORDER BY bm25(performance_search), performance_id
		LIMIT ?
	`, fts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := []models.EntitySearchHit{}
	for rows.Next() {
		var id, label, snippet string
		var rank float64
		if err := rows.Scan(&id, &label, &snippet, &rank); err != nil {
			return nil, err
		}
		hits = append(hits, entitySearchHit("performance", id, label, snippet, rank, weight))
	}
	return hits, rows.Err()
}

func entitySearchHit(kind, id, label, snippet string, rank, weight float64) models.EntitySearchHit {
	snippet = strings.TrimSpace(snippet)
	if snippet == "" {
		snippet = label
	}
	return models.EntitySearchHit{
		Kind:    kind,
		ID:      id,
		Label:   label,
		Score:   -rank * weight,
		Snippet: snippet,
	}
}

func entitySearchKindSet(kinds []string) map[string]bool {
	defaults := []string{"artist", "quote", "work", "recording", "performance"}
	if len(kinds) == 0 {
		kinds = defaults
	}
	enabled := map[string]bool{}
	for _, kind := range kinds {
		switch kind {
		case "artist", "quote", "work", "recording", "performance":
			enabled[kind] = true
		}
	}
	return enabled
}

func entitySearchWeights() map[string]float64 {
	defaults := map[string]float64{
		"artist":      1.15,
		"quote":       1.0,
		"work":        1.2,
		"recording":   1.05,
		"performance": 0.9,
	}
	weights := map[string]float64{}
	for kind, fallback := range defaults {
		weights[kind] = entitySearchWeight(kind, fallback)
	}
	return weights
}

func entitySearchWeight(kind string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv("TANABATA_SEARCH_WEIGHT_" + strings.ToUpper(kind)))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func normalizeEntitySearchLimit(limit int) int {
	switch {
	case limit <= 0:
		return 100
	case limit > 500:
		return 500
	default:
		return limit
	}
}

func (s *Store) rebuildSearchIndices(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM artist_search`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM quote_search`); err != nil {
		return err
	}

	artists, err := s.ListArtists(ctx, models.ArtistFilters{Limit: 1000})
	if err != nil {
		return err
	}
	for _, artist := range artists.Data {
		if err := s.syncArtistSearch(ctx, artist); err != nil {
			return err
		}
	}
	quotes, err := s.ListQuotes(ctx, models.QuoteFilters{Limit: 1000})
	if err != nil {
		return err
	}
	for _, quote := range quotes.Data {
		if err := s.syncQuoteSearch(ctx, quote); err != nil {
			return err
		}
	}
	if err := s.rebuildRecordingSearch(ctx); err != nil {
		return err
	}
	if err := s.rebuildWorkSearch(ctx); err != nil {
		return err
	}
	if err := s.rebuildPerformanceSearch(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Store) syncArtistSearch(ctx context.Context, artist models.Artist) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM artist_search WHERE artist_id = ?`, artist.ArtistID); err != nil {
		return err
	}
	aliases := strings.Join(dedupeStrings(artist.Aliases), " ")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO artist_search(artist_id, name, aliases)
		VALUES(?, ?, ?)
	`, artist.ArtistID, artist.Name, aliases)
	return err
}

func (s *Store) syncQuoteSearch(ctx context.Context, quote models.Quote) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM quote_search WHERE quote_id = ?`, quote.QuoteID); err != nil {
		return err
	}
	tags := strings.Join(dedupeStrings(quote.Tags), " ")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO quote_search(quote_id, artist_id, artist_name, text, tags)
		VALUES(?, ?, ?, ?, ?)
	`, quote.QuoteID, quote.ArtistID, quote.ArtistName, quote.Text, tags)
	return err
}

func scanQuoteRow(scanner interface{ Scan(dest ...any) error }) (models.Quote, error) {
	var quote models.Quote
	var year string
	var sourceID string
	if err := scanner.Scan(
		&quote.QuoteID,
		&quote.Text,
		&quote.ArtistID,
		&quote.ArtistName,
		&sourceID,
		&quote.SourceType,
		&quote.WorkTitle,
		&year,
		&quote.ProvenanceStatus,
		&quote.ConfidenceScore,
		&quote.ProviderOrigin,
		&quote.License,
		&quote.FirstSeenAt,
		&quote.LastVerifiedAt,
	); err != nil {
		return quote, err
	}
	quote.SourceID = emptyToNull(sourceID)
	quote.Year = parseOptionalYear(year)
	applyQuoteFreshness(&quote, time.Now().UTC())
	return quote, nil
}

func (s *Store) hydrateQuote(ctx context.Context, quote *models.Quote) error {
	var err error
	quote.Tags, err = s.quoteTags(ctx, quote.QuoteID)
	if err != nil {
		return err
	}
	quote.Evidence, err = s.quoteEvidence(ctx, quote.QuoteID)
	if err != nil {
		return err
	}
	if quote.SourceID != "" {
		quote.Source, err = s.SourceByID(ctx, quote.SourceID)
		return err
	}
	return nil
}

func (s *Store) quotesByIDs(ctx context.Context, quoteIDs []string) ([]models.Quote, error) {
	if len(quoteIDs) == 0 {
		return []models.Quote{}, nil
	}
	placeholders := sqlPlaceholders(len(quoteIDs))
	args := stringArgs(quoteIDs)
	// #nosec G202 -- placeholders are generated; values remain bound
	query := `
		SELECT
			quotes.quote_id,
			quotes.text,
			artists.artist_id,
			artists.name,
			quotes.source_id,
			quotes.source_type,
			quotes.work_title,
			quotes.year,
			quotes.provenance_status,
			quotes.confidence_score,
			quotes.provider_origin,
			quotes.license,
			quotes.first_seen_at,
			quotes.last_verified_at,
			quote_sources.source_id,
			quote_sources.provider,
			quote_sources.url,
			quote_sources.title,
			quote_sources.publisher,
			quote_sources.license,
			quote_sources.retrieved_at
		FROM quotes
		JOIN artists ON artists.artist_id = quotes.artist_id
		LEFT JOIN quote_sources ON quote_sources.source_id = quotes.source_id AND quotes.source_id <> ''
		WHERE quotes.quote_id IN (` + placeholders + `)`
	rows, err := s.db.QueryContext(ctx, query, args...) // #nosec G201 -- placeholders only; values remain bound
	if err != nil {
		return nil, err
	}
	quotes, err := s.scanQuoteRowsWithChildren(ctx, rows)
	if err != nil {
		return nil, err
	}
	quotesByID := make(map[string]models.Quote, len(quotes))
	for _, quote := range quotes {
		quotesByID[quote.QuoteID] = quote
	}
	ordered := make([]models.Quote, 0, len(quoteIDs))
	for _, quoteID := range quoteIDs {
		if quote, ok := quotesByID[quoteID]; ok {
			ordered = append(ordered, quote)
		}
	}
	return ordered, nil
}

func (s *Store) scanQuoteRowsWithChildren(ctx context.Context, rows *sql.Rows) ([]models.Quote, error) {
	now := time.Now().UTC()
	quotes := []models.Quote{}
	quoteIDs := []string{}
	quoteIndex := map[string]int{}
	for rows.Next() {
		var quote models.Quote
		var year string
		var sourceID string
		var sourceSourceID, sourceProvider, sourceURL, sourceTitle, sourcePublisher, sourceLicense, sourceRetrievedAt sql.NullString
		if err := rows.Scan(
			&quote.QuoteID,
			&quote.Text,
			&quote.ArtistID,
			&quote.ArtistName,
			&sourceID,
			&quote.SourceType,
			&quote.WorkTitle,
			&year,
			&quote.ProvenanceStatus,
			&quote.ConfidenceScore,
			&quote.ProviderOrigin,
			&quote.License,
			&quote.FirstSeenAt,
			&quote.LastVerifiedAt,
			&sourceSourceID,
			&sourceProvider,
			&sourceURL,
			&sourceTitle,
			&sourcePublisher,
			&sourceLicense,
			&sourceRetrievedAt,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
		quote.SourceID = emptyToNull(sourceID)
		quote.Year = parseOptionalYear(year)
		quote.Tags = []string{}
		quote.Evidence = []string{}
		applyQuoteFreshness(&quote, now)
		if sourceSourceID.Valid {
			quote.Source = &models.Source{
				SourceID:    sourceSourceID.String,
				Provider:    sourceProvider.String,
				URL:         sourceURL.String,
				Title:       sourceTitle.String,
				Publisher:   sourcePublisher.String,
				License:     sourceLicense.String,
				RetrievedAt: sourceRetrievedAt.String,
			}
		}
		quoteIndex[quote.QuoteID] = len(quotes)
		quotes = append(quotes, quote)
		quoteIDs = append(quoteIDs, quote.QuoteID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(quoteIDs) == 0 {
		return quotes, nil
	}
	placeholders := sqlPlaceholders(len(quoteIDs))
	args := stringArgs(quoteIDs)

	// #nosec G202 -- placeholders are generated; values remain bound
	tagQuery := `
		SELECT quote_tags.quote_id, quote_tags.tag
		FROM quote_tags
		WHERE quote_tags.quote_id IN (` + placeholders + `)
		ORDER BY quote_tags.quote_id ASC, quote_tags.tag ASC`
	tagRows, err := s.db.QueryContext(ctx, tagQuery, args...) // #nosec G201 -- placeholders only; values remain bound
	if err != nil {
		return nil, err
	}
	for tagRows.Next() {
		var quoteID, tag string
		if err := tagRows.Scan(&quoteID, &tag); err != nil {
			_ = tagRows.Close()
			return nil, err
		}
		if idx, ok := quoteIndex[quoteID]; ok {
			quotes[idx].Tags = append(quotes[idx].Tags, tag)
		}
	}
	if err := tagRows.Err(); err != nil {
		_ = tagRows.Close()
		return nil, err
	}
	if err := tagRows.Close(); err != nil {
		return nil, err
	}

	// #nosec G202 -- placeholders are generated; values remain bound
	evidenceQuery := `
		SELECT quote_evidence.quote_id, quote_evidence.evidence
		FROM quote_evidence
		WHERE quote_evidence.quote_id IN (` + placeholders + `)
		ORDER BY quote_evidence.quote_id ASC, quote_evidence.position ASC`
	evidenceRows, err := s.db.QueryContext(ctx, evidenceQuery, args...) // #nosec G201 -- placeholders only; values remain bound
	if err != nil {
		return nil, err
	}
	for evidenceRows.Next() {
		var quoteID, evidence string
		if err := evidenceRows.Scan(&quoteID, &evidence); err != nil {
			_ = evidenceRows.Close()
			return nil, err
		}
		if idx, ok := quoteIndex[quoteID]; ok {
			quotes[idx].Evidence = append(quotes[idx].Evidence, evidence)
		}
	}
	if err := evidenceRows.Err(); err != nil {
		_ = evidenceRows.Close()
		return nil, err
	}
	if err := evidenceRows.Close(); err != nil {
		return nil, err
	}
	return quotes, nil
}

func applyQuoteFreshness(quote *models.Quote, now time.Time) {
	if quote.LastVerifiedAt == "" {
		quote.FreshnessStatus = "unknown"
		quote.FreshnessReason = "quote has not been verified against a source"
		return
	}
	verifiedAt, err := time.Parse(time.RFC3339, quote.LastVerifiedAt)
	if err != nil {
		quote.FreshnessStatus = "unknown"
		quote.FreshnessReason = "last_verified_at is not parseable"
		return
	}
	age := int(now.Sub(verifiedAt).Hours() / 24)
	if age < 0 {
		age = 0
	}
	quote.FreshnessAgeDays = &age
	switch {
	case now.Sub(verifiedAt) >= quoteFreshnessStaleAfter:
		quote.FreshnessStatus = "stale"
		quote.FreshnessReason = "last source verification is older than 180 days"
	case now.Sub(verifiedAt) >= quoteFreshnessAgingAfter:
		quote.FreshnessStatus = "aging"
		quote.FreshnessReason = "last source verification is older than 90 days"
	default:
		quote.FreshnessStatus = "fresh"
		quote.FreshnessReason = "recently verified against source metadata"
	}
}

func scanArtistRow(scanner interface{ Scan(dest ...any) error }) (models.Artist, error) {
	var artist models.Artist
	var statusJSON string
	if err := scanner.Scan(
		&artist.ArtistID,
		&artist.Name,
		&artist.MBID,
		&artist.WikidataID,
		&artist.WikiquoteTitle,
		&artist.Country,
		&artist.LifeSpan.Begin,
		&artist.LifeSpan.End,
		&artist.Description,
		&artist.BioSummary,
		&statusJSON,
	); err != nil {
		return artist, err
	}
	artist.ProviderStatus = map[string]string{}
	if statusJSON != "" {
		_ = json.Unmarshal([]byte(statusJSON), &artist.ProviderStatus)
	}
	return artist, nil
}

func (s *Store) hydrateArtist(ctx context.Context, artist *models.Artist) error {
	var err error
	artist.Aliases, err = s.artistAliases(ctx, artist.ArtistID)
	if err != nil {
		return err
	}
	artist.Genres, err = s.artistGenres(ctx, artist.ArtistID)
	if err != nil {
		return err
	}
	artist.Links, err = s.artistLinks(ctx, artist.ArtistID)
	return err
}

func (s *Store) artistAliases(ctx context.Context, artistID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT alias FROM artist_aliases WHERE artist_id = ? ORDER BY alias ASC`, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	aliases := []string{}
	for rows.Next() {
		var alias string
		if err := rows.Scan(&alias); err != nil {
			return nil, err
		}
		aliases = append(aliases, alias)
	}
	return aliases, rows.Err()
}

func (s *Store) artistLinks(ctx context.Context, artistID string) ([]models.ArtistLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT provider, kind, url, external_id FROM artist_links WHERE artist_id = ? ORDER BY provider ASC, kind ASC
	`, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := []models.ArtistLink{}
	for rows.Next() {
		var link models.ArtistLink
		if err := rows.Scan(&link.Provider, &link.Kind, &link.URL, &link.ExternalID); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (s *Store) artistGenres(ctx context.Context, artistID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tag FROM artist_tags WHERE artist_id = ? ORDER BY tag ASC
	`, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	genres := []string{}
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		genres = append(genres, tag)
	}
	return genres, rows.Err()
}

func (s *Store) quoteTags(ctx context.Context, quoteID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT tag FROM quote_tags WHERE quote_id = ? ORDER BY tag ASC`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []string{}
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (s *Store) quoteEvidence(ctx context.Context, quoteID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT evidence FROM quote_evidence WHERE quote_id = ? ORDER BY position ASC
	`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	evidence := []string{}
	for rows.Next() {
		var item string
		if err := rows.Scan(&item); err != nil {
			return nil, err
		}
		evidence = append(evidence, item)
	}
	return evidence, rows.Err()
}

func sqlPlaceholders(count int) string {
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}

func stringArgs(values []string) []any {
	args := make([]any, len(values))
	for idx, value := range values {
		args[idx] = value
	}
	return args
}

func normalizeLimit(limit int) int {
	switch {
	case limit <= 0:
		return 20
	case limit > 100:
		return 100
	default:
		return limit
	}
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func normalizeSimilarLimit(limit int) int {
	switch {
	case limit <= 0:
		return 10
	case limit > 100:
		return 100
	default:
		return limit
	}
}

func mergeQuotes(existing, incoming models.Quote) models.Quote {
	merged := incoming
	merged.QuoteID = existing.QuoteID
	merged.ArtistID = coalesceString(incoming.ArtistID, existing.ArtistID)
	merged.ArtistName = coalesceString(incoming.ArtistName, existing.ArtistName)
	merged.Text = preferLongerString(existing.Text, incoming.Text)
	merged.Tags = dedupeStrings(append(existing.Tags, incoming.Tags...))
	merged.Evidence = dedupeStrings(append(existing.Evidence, incoming.Evidence...))
	merged.ConfidenceScore = maxFloat(existing.ConfidenceScore, incoming.ConfidenceScore)
	merged.FirstSeenAt = earliestTimestamp(existing.FirstSeenAt, incoming.FirstSeenAt)
	merged.LastVerifiedAt = latestTimestamp(existing.LastVerifiedAt, incoming.LastVerifiedAt)

	preferred := incoming
	if quotePriority(existing) > quotePriority(incoming) {
		preferred = existing
	}
	merged.ProvenanceStatus = preferred.ProvenanceStatus
	merged.ProviderOrigin = coalesceString(preferred.ProviderOrigin, coalesceString(existing.ProviderOrigin, incoming.ProviderOrigin))
	merged.SourceID = coalesceString(preferred.SourceID, coalesceString(existing.SourceID, incoming.SourceID))
	merged.SourceType = coalesceString(preferred.SourceType, coalesceString(existing.SourceType, incoming.SourceType))
	merged.WorkTitle = coalesceString(preferred.WorkTitle, coalesceString(existing.WorkTitle, incoming.WorkTitle))
	merged.License = coalesceString(preferred.License, coalesceString(existing.License, incoming.License))
	merged.Source = preferredSource(preferred.Source, existing.Source, incoming.Source)
	if merged.Source != nil && merged.SourceID == "" {
		merged.SourceID = merged.Source.SourceID
	}
	if merged.Year == nil {
		merged.Year = existing.Year
	}
	if merged.Year == nil {
		merged.Year = incoming.Year
	}
	return merged
}

func quotePriority(quote models.Quote) int {
	return provenanceRank(quote.ProvenanceStatus)*1000 + int(quote.ConfidenceScore*100)
}

func provenanceRank(status string) int {
	switch status {
	case "verified":
		return 5
	case "source_attributed":
		return 4
	case "provider_attributed":
		return 3
	case "ambiguous":
		return 2
	case "needs_review", "legacy_unverified":
		return 1
	default:
		return 0
	}
}

func provenanceStatusCounts() map[string]int {
	return map[string]int{
		"verified":            0,
		"source_attributed":   0,
		"provider_attributed": 0,
		"ambiguous":           0,
		"needs_review":        0,
	}
}

func confidenceBucket(confidence float64) int {
	switch {
	case confidence <= 0:
		return 0
	case confidence >= 1:
		return 9
	default:
		return int(confidence * 10)
	}
}

func refreshHint(lastVerifiedAt string, now time.Time) string {
	verifiedAt, err := time.Parse(time.RFC3339, lastVerifiedAt)
	if lastVerifiedAt == "" || err != nil {
		return "stale"
	}
	switch {
	case now.Sub(verifiedAt) >= quoteFreshnessStaleAfter:
		return "stale"
	case now.Sub(verifiedAt) >= quoteFreshnessAgingAfter:
		return "aging"
	default:
		return "fresh"
	}
}

func refreshHintRank(hint string) int {
	switch hint {
	case "stale":
		return 3
	case "aging":
		return 2
	case "fresh":
		return 1
	default:
		return 0
	}
}

func refreshHintFromRank(rank int) string {
	switch rank {
	case 3:
		return "stale"
	case 2:
		return "aging"
	case 1:
		return "fresh"
	default:
		return "unknown"
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func preferredSource(primary *models.Source, fallbacks ...*models.Source) *models.Source {
	if primary != nil {
		return primary
	}
	for _, candidate := range fallbacks {
		if candidate != nil {
			return candidate
		}
	}
	return nil
}

func coalesceString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func preferLongerString(left, right string) string {
	if len(strings.TrimSpace(right)) > len(strings.TrimSpace(left)) {
		return right
	}
	if strings.TrimSpace(left) != "" {
		return left
	}
	return right
}

func earliestTimestamp(left, right string) string {
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	leftTime, leftErr := time.Parse(time.RFC3339, left)
	rightTime, rightErr := time.Parse(time.RFC3339, right)
	if leftErr != nil || rightErr != nil {
		if left <= right {
			return left
		}
		return right
	}
	if leftTime.Before(rightTime) {
		return left
	}
	return right
}

func latestTimestamp(left, right string) string {
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	leftTime, leftErr := time.Parse(time.RFC3339, left)
	rightTime, rightErr := time.Parse(time.RFC3339, right)
	if leftErr != nil || rightErr != nil {
		if left >= right {
			return left
		}
		return right
	}
	if leftTime.After(rightTime) {
		return left
	}
	return right
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func parseOptionalYear(raw string) *int {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &value
}

func nullToEmpty(value string) string {
	return strings.TrimSpace(value)
}

func emptyToNull(value string) string {
	return strings.TrimSpace(value)
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	var deduped []string
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		deduped = append(deduped, trimmed)
	}
	sort.Strings(deduped)
	return deduped
}

func dedupeArtistLinks(links []models.ArtistLink) []models.ArtistLink {
	type linkKey struct {
		provider string
		kind     string
		url      string
	}
	seen := make(map[linkKey]int, len(links))
	deduped := make([]models.ArtistLink, 0, len(links))
	for _, link := range links {
		key := linkKey{provider: link.Provider, kind: link.Kind, url: link.URL}
		if idx, ok := seen[key]; ok {
			deduped[idx] = link
			continue
		}
		seen[key] = len(deduped)
		deduped = append(deduped, link)
	}
	return deduped
}

func ftsQuery(input string) string {
	tokens := strings.Fields(input)
	terms := make([]string, 0, len(tokens))
	for _, token := range tokens {
		term := ftsEscape(token)
		if term == "" {
			continue
		}
		terms = append(terms, term)
	}
	return strings.Join(terms, " ")
}

func ftsEscape(term string) string {
	term = strings.TrimSpace(term)
	if term == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(term, `"`, `""`) + `"*`
}
