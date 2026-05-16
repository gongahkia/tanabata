package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

type Store struct {
	db *sql.DB
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create catalog dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	store := &Store{db: db}
	if err := store.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
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
	defer rows.Close()
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
			return err
		}
		if name == column {
			return nil
		}
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

	content, err := os.ReadFile(legacyPath)
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
	content, err := os.ReadFile(bundlePath)
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
	defer rows.Close()
	var providers []string
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return err
		}
		providers = append(providers, provider)
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
			INSERT INTO artist_aliases(artist_id, alias, normalized_alias)
			VALUES(?, ?, ?)
		`, artist.ArtistID, alias, search.NormalizeText(alias)); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM artist_links WHERE artist_id = ?`, artist.ArtistID); err != nil {
		return err
	}
	for _, link := range artist.Links {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO artist_links(artist_id, provider, kind, url, external_id)
			VALUES(?, ?, ?, ?, ?)
		`, artist.ArtistID, link.Provider, link.Kind, link.URL, link.ExternalID); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM artist_tags WHERE artist_id = ?`, artist.ArtistID); err != nil {
		return err
	}
	for _, genre := range dedupeStrings(artist.Genres) {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO artist_tags(artist_id, tag) VALUES(?, ?)`, artist.ArtistID, genre); err != nil {
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
	for _, table := range []string{"quotes", "artist_aliases", "artist_links", "artist_tags", "releases"} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`UPDATE %s SET artist_id = ? WHERE artist_id = ?`, table), newID, oldID); err != nil {
			return err
		}
	}
	for _, column := range []string{"artist_id", "related_artist_id"} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`UPDATE artist_relations SET %s = ? WHERE %s = ?`, column, column), newID, oldID); err != nil {
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
	existingID, existingStatus, err := s.existingQuote(ctx, quote.ArtistID, quote.Text)
	if err != nil {
		return err
	}
	if existingID != "" {
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
	}
	if quote.QuoteID == "" {
		sourceURL := ""
		if quote.Source != nil {
			sourceURL = quote.Source.URL
		}
		quote.QuoteID = search.QuoteID(quote.ArtistID, search.NormalizeText(quote.Text), sourceURL)
	}
	year := ""
	if quote.Year != nil {
		year = strconv.Itoa(*quote.Year)
	}
	providerOrigin := strings.TrimSpace(quote.ProviderOrigin)
	if providerOrigin == "" && quote.Source != nil {
		providerOrigin = quote.Source.Provider
	}
	if _, err := s.db.ExecContext(ctx, `
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
			provenance_status = excluded.provenance_status,
			confidence_score = excluded.confidence_score,
			provider_origin = excluded.provider_origin,
			license = excluded.license,
			first_seen_at = excluded.first_seen_at,
			last_verified_at = excluded.last_verified_at
	`, quote.QuoteID, quote.Text, search.NormalizeText(quote.Text), quote.ArtistID, nullToEmpty(quote.SourceID), quote.SourceType, quote.WorkTitle, year, quote.ProvenanceStatus, quote.ConfidenceScore, providerOrigin, quote.License, quote.FirstSeenAt, quote.LastVerifiedAt); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM quote_tags WHERE quote_id = ?`, quote.QuoteID); err != nil {
		return err
	}
	for _, tag := range dedupeStrings(quote.Tags) {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO quote_tags(quote_id, tag) VALUES(?, ?)`, quote.QuoteID, tag); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM quote_evidence WHERE quote_id = ?`, quote.QuoteID); err != nil {
		return err
	}
	for idx, evidence := range dedupeStrings(quote.Evidence) {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO quote_evidence(quote_id, evidence, position)
			VALUES(?, ?, ?)
		`, quote.QuoteID, evidence, idx); err != nil {
			return err
		}
	}
	if len(quote.Evidence) == 0 {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO quote_evidence(quote_id, evidence, position)
			VALUES(?, ?, 0)
		`, quote.QuoteID, "Imported without explicit evidence; inspect source metadata."); err != nil {
			return err
		}
	}
	if quote.ArtistName == "" {
		artist, _ := s.ArtistByID(ctx, quote.ArtistID)
		if artist != nil {
			quote.ArtistName = artist.Name
		}
	}
	return s.syncQuoteSearch(ctx, quote)
}

func (s *Store) existingQuote(ctx context.Context, artistID, text string) (string, string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT quote_id, provenance_status, text
		FROM quotes
		WHERE artist_id = ?
	`, artistID)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()

	bestID := ""
	bestProvenance := ""
	bestScore := 0
	for rows.Next() {
		var quoteID, provenance, candidateText string
		if err := rows.Scan(&quoteID, &provenance, &candidateText); err != nil {
			return "", "", err
		}
		score := search.QuoteMergeScore(text, candidateText)
		if score > bestScore {
			bestID = quoteID
			bestProvenance = provenance
			bestScore = score
		}
	}
	if err := rows.Err(); err != nil {
		return "", "", err
	}
	if bestScore < 90 {
		return "", "", nil
	}
	return bestID, bestProvenance, nil
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
	defer rows.Close()
	for rows.Next() {
		quote, err := s.scanQuoteRow(ctx, rows)
		if err != nil {
			return response, err
		}
		response.Data = append(response.Data, quote)
	}
	return response, rows.Err()
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
		WHERE quotes.quote_id = ?
	`, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	quote, err := s.scanQuoteRow(ctx, rows)
	if err != nil {
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
	defer rows.Close()
	for rows.Next() {
		var quoteID string
		var status string
		var confidence float64
		if err := rows.Scan(&quoteID, &status, &confidence); err != nil {
			return response, err
		}
		quote, err := s.QuoteByID(ctx, quoteID)
		if err != nil {
			return response, err
		}
		if quote == nil {
			continue
		}
		response.Data = append(response.Data, models.ReviewQueueItem{
			Quote:     *quote,
			Reason:    reviewReason(status, confidence),
			RiskScore: reviewRiskScore(status, confidence),
		})
	}
	return response, rows.Err()
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
	defer rows.Close()
	for rows.Next() {
		var quoteID string
		if err := rows.Scan(&quoteID); err != nil {
			return response, err
		}
		quote, err := s.QuoteByID(ctx, quoteID)
		if err != nil {
			return response, err
		}
		if quote != nil {
			response.Data = append(response.Data, *quote)
		}
	}
	return response, rows.Err()
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
	defer rows.Close()
	for rows.Next() {
		artist, err := s.scanArtistRow(ctx, rows)
		if err != nil {
			return response, err
		}
		response.Data = append(response.Data, artist)
	}
	return response, rows.Err()
}

func (s *Store) ArtistByID(ctx context.Context, artistID string) (*models.Artist, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT artist_id, name, COALESCE(mbid, ''), COALESCE(wikidata_id, ''), COALESCE(wikiquote_title, ''), COALESCE(country, ''), COALESCE(life_span_begin, ''), COALESCE(life_span_end, ''), COALESCE(description, ''), COALESCE(bio_summary, ''), COALESCE(provider_status, '{}')
		FROM artists WHERE artist_id = ?
	`, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	artist, err := s.scanArtistRow(ctx, rows)
	if err != nil {
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
	response := models.SearchResponse{}
	meta, err := s.Meta(ctx)
	if err != nil {
		return response, err
	}
	response.Meta = meta

	artists, err := s.searchArtists(ctx, query, 10)
	if err != nil {
		return response, err
	}
	quotes, err := s.searchQuotes(ctx, query, 10)
	if err != nil {
		return response, err
	}
	if len(artists) == 0 {
		fallback, err := s.ListArtists(ctx, models.ArtistFilters{Query: query, Limit: 10, Offset: 0})
		if err != nil {
			return response, err
		}
		artists = fallback.Data
	}
	if len(quotes) == 0 {
		fallback, err := s.ListQuotes(ctx, models.QuoteFilters{Query: query, Limit: 10, Offset: 0})
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
	checks := map[string]string{
		"quotes_missing_artist": `SELECT COUNT(*) FROM quotes LEFT JOIN artists ON artists.artist_id = quotes.artist_id WHERE artists.artist_id IS NULL`,
		"quotes_missing_source": `SELECT COUNT(*) FROM quotes WHERE source_id <> '' AND source_id NOT IN (SELECT source_id FROM quote_sources)`,
		"tags_missing_quote":    `SELECT COUNT(*) FROM quote_tags LEFT JOIN quotes ON quotes.quote_id = quote_tags.quote_id WHERE quotes.quote_id IS NULL`,
		"evidence_missing_quote": `SELECT COUNT(*) FROM quote_evidence
			LEFT JOIN quotes ON quotes.quote_id = quote_evidence.quote_id
			WHERE quotes.quote_id IS NULL`,
		"job_items_missing_job": `SELECT COUNT(*) FROM job_items LEFT JOIN jobs ON jobs.job_id = job_items.job_id WHERE jobs.job_id IS NULL`,
	}
	for name, query := range checks {
		var count int
		if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			return report, err
		}
		report.Counts[name] = count
		if count > 0 {
			report.OK = false
			report.Issues = append(report.Issues, name)
		}
	}
	sort.Strings(report.Issues)
	return report, nil
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
		_ = s.db.QueryRowContext(ctx, `
			SELECT status, finished_at
			FROM provider_runs
			WHERE provider = ?
			ORDER BY started_at DESC
			LIMIT 1
		`, item.Provider).Scan(&summary.LastStatus, new(string))

		_ = s.db.QueryRowContext(ctx, `
			SELECT finished_at
			FROM provider_runs
			WHERE provider = ? AND status = 'success'
			ORDER BY finished_at DESC
			LIMIT 1
		`, item.Provider).Scan(&summary.LastSuccessful)

		_ = s.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM provider_errors
			WHERE provider = ? AND occurred_at >= ?
		`, item.Provider, time.Now().UTC().Add(-30*24*time.Hour).Format(time.RFC3339)).Scan(&summary.RecentErrorCount)

		_ = s.db.QueryRowContext(ctx, `
			SELECT occurred_at
			FROM provider_errors
			WHERE provider = ?
			ORDER BY occurred_at DESC
			LIMIT 1
		`, item.Provider).Scan(&summary.LastErrorAt)
		if cooldown, active, err := s.ProviderCooldown(ctx, item.Provider, time.Now().UTC()); err == nil && active {
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
	return err
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
	for key, query := range map[string]string{
		"artists":         `SELECT COUNT(*) FROM artists`,
		"quotes":          `SELECT COUNT(*) FROM quotes`,
		"sources":         `SELECT COUNT(*) FROM quote_sources`,
		"releases":        `SELECT COUNT(*) FROM releases`,
		"related_artists": `SELECT COUNT(*) FROM artist_relations`,
		"jobs":            `SELECT COUNT(*) FROM jobs`,
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
	defer rows.Close()
	var jobs []models.JobRun
	for rows.Next() {
		var job models.JobRun
		if err := rows.Scan(&job.JobID, &job.Name, &job.Scope, &job.Status, &job.StartedAt, &job.FinishedAt, &job.Details, &job.ErrorMessage); err != nil {
			return nil, err
		}
		job.Items, _ = s.jobItems(ctx, job.JobID)
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
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

func (s *Store) searchArtists(ctx context.Context, query string, limit int) ([]models.Artist, error) {
	fts := ftsQuery(query)
	if fts == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT artist_id
		FROM artist_search
		WHERE artist_search MATCH ?
		ORDER BY bm25(artist_search), artist_id
		LIMIT ?
	`, fts, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var artists []models.Artist
	for rows.Next() {
		var artistID string
		if err := rows.Scan(&artistID); err != nil {
			return nil, err
		}
		artist, err := s.ArtistByID(ctx, artistID)
		if err != nil || artist == nil {
			continue
		}
		artists = append(artists, *artist)
	}
	return artists, rows.Err()
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
	defer rows.Close()
	var quotes []models.Quote
	for rows.Next() {
		var quoteID string
		if err := rows.Scan(&quoteID); err != nil {
			return nil, err
		}
		quote, err := s.QuoteByID(ctx, quoteID)
		if err != nil || quote == nil {
			continue
		}
		quotes = append(quotes, *quote)
	}
	return quotes, rows.Err()
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

func (s *Store) scanQuoteRow(ctx context.Context, scanner interface{ Scan(dest ...any) error }) (models.Quote, error) {
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
	quote.Tags, _ = s.quoteTags(ctx, quote.QuoteID)
	quote.Evidence, _ = s.quoteEvidence(ctx, quote.QuoteID)
	if quote.SourceID != "" {
		quote.Source, _ = s.SourceByID(ctx, quote.SourceID)
	}
	quote.Year = parseOptionalYear(year)
	applyQuoteFreshness(&quote, time.Now().UTC())
	return quote, nil
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

func (s *Store) scanArtistRow(ctx context.Context, scanner interface{ Scan(dest ...any) error }) (models.Artist, error) {
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
	artist.Aliases, _ = s.artistAliases(ctx, artist.ArtistID)
	artist.Genres, _ = s.artistGenres(ctx, artist.ArtistID)
	artist.Links, _ = s.artistLinks(ctx, artist.ArtistID)
	artist.ProviderStatus = map[string]string{}
	if statusJSON != "" {
		_ = json.Unmarshal([]byte(statusJSON), &artist.ProviderStatus)
	}
	return artist, nil
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

func ftsQuery(input string) string {
	terms := strings.Fields(search.NormalizeText(input))
	if len(terms) == 0 {
		return ""
	}
	for i, term := range terms {
		terms[i] = term + "*"
	}
	return strings.Join(terms, " ")
}
