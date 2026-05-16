package catalog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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
	if err := os.WriteFile(path, content, 0o644); err != nil {
		b.Fatalf("WriteFile(%s) error = %v", path, err)
	}
	return path
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
		if _, err := store.Search(ctx, "discipline"); err != nil {
			b.Fatalf("Search() error = %v", err)
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
