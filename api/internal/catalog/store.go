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

func (s *Store) init(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON;`,
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
			occurred_at TEXT NOT NULL,
			context TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL
		);`,
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	return nil
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
		if _, seen := artists[artistID]; seen {
			continue
		}
		artists[artistID] = struct{}{}
		status, _ := json.Marshal(map[string]string{"legacy": "imported"})
		if _, err := tx.ExecContext(ctx, `
		INSERT INTO artists(artist_id, name, slug, provider_status)
			VALUES(?, ?, ?, ?)
		`, artistID, name, search.Slug(name), string(status)); err != nil {
			return fmt.Errorf("insert artist %s: %w", name, err)
		}
		aliases := dedupeStrings([]string{name, strings.ReplaceAll(name, " ", "-")})
		for _, alias := range aliases {
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

	for _, quote := range quotes {
		artistID := search.ArtistID(quote.Author, "")
		normalizedText := search.NormalizeText(quote.Text)
		quoteID := search.QuoteID(artistID, normalizedText, sourceURL)
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO quotes(
				quote_id, text, normalized_text, artist_id, source_id, source_type, work_title, year,
				provenance_status, confidence_score, license, first_seen_at, last_verified_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, quoteID, strings.TrimSpace(quote.Text), normalizedText, artistID, sourceID, "legacy_scrape", "", "", "legacy_unverified", 0.25, "unknown", now, now); err != nil {
			return fmt.Errorf("insert quote: %w", err)
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

	return tx.Commit()
}

func (s *Store) quoteCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM quotes`).Scan(&count)
	return count, err
}

func (s *Store) Meta(ctx context.Context) (models.ListMeta, error) {
	meta := models.ListMeta{}
	var snapshot string
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM catalog_meta WHERE key = ?`, snapshotVersionKey).Scan(&snapshot); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return meta, err
	}
	var providers string
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM catalog_meta WHERE key = ?`, activeProvidersKey).Scan(&providers); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return meta, err
	}
	meta.SnapshotVersion = snapshot
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
		INSERT INTO provider_errors(error_id, provider, occurred_at, context, message)
		VALUES(?, ?, ?, ?, ?)
	`, failure.ErrorID, failure.Provider, failure.OccurredAt.UTC().Format(time.RFC3339), failure.Context, failure.Message)
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
	_, err = s.db.ExecContext(ctx, `
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
	`, artist.ArtistID, artist.Name, search.Slug(artist.Name), artist.MBID, artist.WikidataID, artist.WikiquoteTitle, artist.Country, artist.LifeSpan.Begin, artist.LifeSpan.End, artist.Description, artist.BioSummary, string(statusJSON))
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM artist_aliases WHERE artist_id = ?`, artist.ArtistID); err != nil {
		return err
	}
	for _, alias := range dedupeStrings(append(artist.Aliases, artist.Name)) {
		if strings.TrimSpace(alias) == "" {
			continue
		}
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
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO artist_tags(artist_id, tag) VALUES(?, ?)
		`, artist.ArtistID, genre); err != nil {
			return err
		}
	}
	return nil
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

	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO artists(artist_id, name, slug)
		SELECT ?, COALESCE(NULLIF(name, ''), ?), ?
		FROM artists
		WHERE artist_id = ?
	`, newID, canonicalName, search.Slug(canonicalName)+"-"+search.StableHash(newID)[:8], oldID)
	if err != nil {
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
	return tx.Commit()
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
	existingID, existingStatus, err := s.existingQuote(ctx, quote.ArtistID, search.NormalizeText(quote.Text))
	if err != nil {
		return err
	}
	if existingID != "" && (existingStatus == "legacy_unverified" || quote.QuoteID == "") {
		quote.QuoteID = existingID
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
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO quotes(
			quote_id, text, normalized_text, artist_id, source_id, source_type, work_title, year,
			provenance_status, confidence_score, license, first_seen_at, last_verified_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			license = excluded.license,
			first_seen_at = excluded.first_seen_at,
			last_verified_at = excluded.last_verified_at
	`, quote.QuoteID, quote.Text, search.NormalizeText(quote.Text), quote.ArtistID, nullToEmpty(quote.SourceID), quote.SourceType, quote.WorkTitle, year, quote.ProvenanceStatus, quote.ConfidenceScore, quote.License, quote.FirstSeenAt, quote.LastVerifiedAt)
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM quote_tags WHERE quote_id = ?`, quote.QuoteID); err != nil {
		return err
	}
	for _, tag := range dedupeStrings(quote.Tags) {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO quote_tags(quote_id, tag) VALUES(?, ?)
		`, quote.QuoteID, tag); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) existingQuote(ctx context.Context, artistID, normalizedText string) (string, string, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT quote_id, provenance_status
		FROM quotes
		WHERE artist_id = ? AND normalized_text = ?
		LIMIT 1
	`, artistID, normalizedText)
	var quoteID, provenance string
	if err := row.Scan(&quoteID, &provenance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", nil
		}
		return "", "", err
	}
	return quoteID, provenance, nil
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
		where = append(where, `(quotes.normalized_text LIKE ? OR artists.artist_id IN (
			SELECT artist_id FROM artist_aliases WHERE normalized_alias LIKE ?
		))`)
		pattern := "%" + search.NormalizeText(filters.Query) + "%"
		args = append(args, pattern, pattern)
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

	order := ` ORDER BY quotes.confidence_score DESC, artists.name ASC, quotes.quote_id ASC`
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
		normalized := "%" + search.NormalizeText(filters.Query) + "%"
		where = append(where, `(artists.artist_id IN (
			SELECT artist_id FROM artist_aliases WHERE normalized_alias LIKE ?
		) OR artists.name LIKE ?)`)
		args = append(args, normalized, "%"+filters.Query+"%")
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

	rows, err := s.db.QueryContext(ctx, `
		SELECT artist_id, alias FROM artist_aliases
	`)
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

	artists, err := s.ListArtists(ctx, models.ArtistFilters{Query: query, Limit: 10, Offset: 0})
	if err != nil {
		return response, err
	}
	quotes, err := s.ListQuotes(ctx, models.QuoteFilters{Query: query, Limit: 10, Offset: 0})
	if err != nil {
		return response, err
	}
	response.Data.Artists = artists.Data
	response.Data.Quotes = quotes.Data
	return response, nil
}

func (s *Store) Stats(ctx context.Context) (map[string]any, error) {
	meta, err := s.Meta(ctx)
	if err != nil {
		return nil, err
	}
	var artists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM artists`).Scan(&artists); err != nil {
		return nil, err
	}
	var quotes int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM quotes`).Scan(&quotes); err != nil {
		return nil, err
	}
	var sources int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM quote_sources`).Scan(&sources); err != nil {
		return nil, err
	}
	var releases int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM releases`).Scan(&releases); err != nil {
		return nil, err
	}
	var related int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM artist_relations`).Scan(&related); err != nil {
		return nil, err
	}
	return map[string]any{
		"artists":          artists,
		"quotes":           quotes,
		"sources":          sources,
		"releases":         releases,
		"related_artists":  related,
		"snapshot_version": meta.SnapshotVersion,
		"active_providers": meta.ActiveProviders,
	}, nil
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
		&quote.License,
		&quote.FirstSeenAt,
		&quote.LastVerifiedAt,
	); err != nil {
		return quote, err
	}
	quote.SourceID = emptyToNull(sourceID)
	quote.Tags, _ = s.quoteTags(ctx, quote.QuoteID)
	if quote.SourceID != "" {
		quote.Source, _ = s.SourceByID(ctx, quote.SourceID)
	}
	quote.Year = parseOptionalYear(year)
	return quote, nil
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
	rows, err := s.db.QueryContext(ctx, `
		SELECT alias FROM artist_aliases WHERE artist_id = ? ORDER BY alias ASC
	`, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var aliases []string
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
	var links []models.ArtistLink
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
		SELECT tag
		FROM artist_tags
		WHERE artist_id = ?
		ORDER BY tag ASC
	`, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var genres []string
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
	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
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
