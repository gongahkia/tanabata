package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/testutil"
)

func newBenchmarkStore(b *testing.B) (*Store, context.Context, string) {
	b.Helper()
	tempDir := b.TempDir()
	store, err := Open(filepath.Join(tempDir, "catalog.sqlite"))
	if err != nil {
		b.Fatalf("Open() error = %v", err)
	}
	ctx := context.Background()
	legacyPath := writeBenchmarkJSON(b, filepath.Join(tempDir, "quotes.json"), testutil.LegacyQuotes())
	curatedPath := writeBenchmarkJSON(b, filepath.Join(tempDir, "curated_quotes.json"), testutil.CuratedQuotes())
	if err := store.SeedFromLegacyJSON(ctx, legacyPath); err != nil {
		b.Fatalf("SeedFromLegacyJSON() error = %v", err)
	}
	if _, err := store.ImportCuratedQuotes(ctx, curatedPath); err != nil {
		b.Fatalf("ImportCuratedQuotes() error = %v", err)
	}
	seedBenchmarkCatalog(b, store, ctx, 120)
	quotes, err := store.ListQuotes(ctx, models.QuoteFilters{Limit: 1})
	if err != nil || len(quotes.Data) == 0 {
		b.Fatalf("ListQuotes() err=%v len=%d", err, len(quotes.Data))
	}
	return store, ctx, quotes.Data[0].QuoteID
}

func writeBenchmarkJSON(b *testing.B, path string, payload any) string {
	b.Helper()
	content, err := json.Marshal(payload)
	if err != nil {
		b.Fatalf("Marshal(%s) error = %v", path, err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		b.Fatalf("WriteFile(%s) error = %v", path, err)
	}
	return path
}

func seedBenchmarkCatalog(b *testing.B, store *Store, ctx context.Context, count int) {
	b.Helper()
	statuses := []string{"needs_review", "ambiguous", "provider_attributed"}
	for i := 0; i < count; i++ {
		artistID := fmt.Sprintf("bench:artist:%02d", i)
		artistName := fmt.Sprintf("Frank Benchmark %02d", i)
		source := models.Source{
			SourceID:    fmt.Sprintf("bench:source:%02d", i),
			Provider:    "benchmark",
			URL:         fmt.Sprintf("https://bench.tanabata.dev/source/%02d", i),
			Title:       fmt.Sprintf("Benchmark Source %02d", i),
			Publisher:   "Tanabata Bench",
			License:     "benchmark",
			RetrievedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		}
		if err := store.UpsertSource(ctx, source); err != nil {
			b.Fatalf("UpsertSource() error = %v", err)
		}
		artist := models.Artist{
			ArtistID:       artistID,
			Name:           artistName,
			ProviderStatus: map[string]string{"benchmark": "seeded"},
		}
		if err := store.UpsertArtist(ctx, artist); err != nil {
			b.Fatalf("UpsertArtist() error = %v", err)
		}
		year := 2020 + i%5
		quote := models.Quote{
			QuoteID:          fmt.Sprintf("bench:quote:%02d", i),
			Text:             fmt.Sprintf("Frank benchmark line %02d uses token %08x for hydration coverage.", i, i*7919),
			ArtistID:         artistID,
			ArtistName:       artistName,
			SourceID:         source.SourceID,
			Source:           &source,
			SourceType:       "benchmark",
			WorkTitle:        fmt.Sprintf("Benchmark Work %02d", i),
			Year:             &year,
			ProvenanceStatus: statuses[i%len(statuses)],
			ConfidenceScore:  float64(i%100) / 100,
			ProviderOrigin:   "benchmark",
			License:          "benchmark",
			FirstSeenAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
			LastVerifiedAt:   time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
		}
		if err := store.UpsertQuote(ctx, quote); err != nil {
			b.Fatalf("UpsertQuote() error = %v", err)
		}
	}
	if _, err := store.db.ExecContext(ctx, `
		DELETE FROM quote_evidence WHERE quote_id LIKE 'bench:quote:%';
		DELETE FROM artist_aliases WHERE artist_id LIKE 'bench:artist:%';
	`); err != nil {
		b.Fatalf("trim benchmark child rows: %v", err)
	}
}

func BenchmarkListQuotes(b *testing.B) {
	store, ctx, _ := newBenchmarkStore(b)
	defer store.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.ListQuotes(ctx, models.QuoteFilters{Limit: 25}); err != nil {
			b.Fatalf("ListQuotes() error = %v", err)
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	store, ctx, _ := newBenchmarkStore(b)
	defer store.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.SearchWithLimit(ctx, "frank", 100); err != nil {
			b.Fatalf("SearchWithLimit() error = %v", err)
		}
	}
}

func BenchmarkReviewQueue(b *testing.B) {
	store, ctx, _ := newBenchmarkStore(b)
	defer store.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.ReviewQueue(ctx, models.ReviewQueueFilters{Limit: 100}); err != nil {
			b.Fatalf("ReviewQueue() error = %v", err)
		}
	}
}

func BenchmarkStaleQuotes(b *testing.B) {
	store, ctx, _ := newBenchmarkStore(b)
	defer store.Close()
	if _, err := store.db.ExecContext(ctx, `UPDATE quotes SET last_verified_at = '2025-01-01T00:00:00Z'`); err != nil {
		b.Fatalf("mark benchmark quotes stale: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.StaleQuotes(ctx, models.ReviewQueueFilters{Limit: 100}); err != nil {
			b.Fatalf("StaleQuotes() error = %v", err)
		}
	}
}

func BenchmarkQuoteProvenance(b *testing.B) {
	store, ctx, quoteID := newBenchmarkStore(b)
	defer store.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.QuoteProvenance(ctx, quoteID); err != nil {
			b.Fatalf("QuoteProvenance() error = %v", err)
		}
	}
}

func BenchmarkProviderSummaries(b *testing.B) {
	store, ctx, _ := newBenchmarkStore(b)
	defer store.Close()
	configured := []models.ProviderSummary{
		{Provider: "wikiquote", Enabled: true},
		{Provider: "musicbrainz", Enabled: true},
		{Provider: "tanabata_curated", Enabled: true},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.ProviderSummaries(ctx, configured); err != nil {
			b.Fatalf("ProviderSummaries() error = %v", err)
		}
	}
}
