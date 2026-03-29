package catalog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

func newSeededStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "catalog.sqlite")
	legacyPath := filepath.Join(tempDir, "quotes.json")
	payload := []models.LegacyQuote{
		{Author: "Frank Ocean", Text: "Work hard in silence."},
		{Author: "Frank Ocean", Text: "Be yourself."},
		{Author: "Taylor Swift", Text: "Just keep dancing."},
	}
	content, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal legacy payload: %v", err)
	}
	if err := osWriteFile(legacyPath, content); err != nil {
		t.Fatalf("write legacy payload: %v", err)
	}
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	ctx := context.Background()
	if err := store.SeedFromLegacyJSON(ctx, legacyPath); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	return store, ctx
}

func TestSeedFromLegacyJSONAndFilters(t *testing.T) {
	store, ctx := newSeededStore(t)
	defer store.Close()

	artists, err := store.ListArtists(ctx, models.ArtistFilters{Limit: 10})
	if err != nil {
		t.Fatalf("ListArtists() error = %v", err)
	}
	if artists.Pagination.Total != 2 {
		t.Fatalf("ListArtists() total = %d, want 2", artists.Pagination.Total)
	}

	quotes, err := store.ListQuotes(ctx, models.QuoteFilters{
		Artist: "frnak ocean",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListQuotes() error = %v", err)
	}
	if len(quotes.Data) != 0 {
		t.Fatalf("ListQuotes() with artist typo should not match before resolution, got %d", len(quotes.Data))
	}

	resolved, err := store.ResolveArtistID(ctx, "frnak ocean")
	if err != nil {
		t.Fatalf("ResolveArtistID() error = %v", err)
	}
	if resolved == "" {
		t.Fatalf("ResolveArtistID() should resolve typo")
	}
	quotes, err = store.ListQuotes(ctx, models.QuoteFilters{
		ArtistID: resolved,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListQuotes() error = %v", err)
	}
	if len(quotes.Data) != 2 {
		t.Fatalf("ListQuotes() len = %d, want 2", len(quotes.Data))
	}
}

func TestUpsertQuoteReplacesLegacyRecord(t *testing.T) {
	store, ctx := newSeededStore(t)
	defer store.Close()

	artistID, err := store.ResolveArtistID(ctx, "Frank Ocean")
	if err != nil {
		t.Fatalf("ResolveArtistID() error = %v", err)
	}
	legacy, err := store.ListQuotes(ctx, models.QuoteFilters{ArtistID: artistID, Limit: 10})
	if err != nil {
		t.Fatalf("ListQuotes() error = %v", err)
	}
	if len(legacy.Data) == 0 {
		t.Fatalf("expected seeded quotes")
	}

	source := models.Source{
		SourceID:    search.SourceID("wikiquote", "https://en.wikiquote.org/wiki/Frank_Ocean"),
		Provider:    "wikiquote",
		URL:         "https://en.wikiquote.org/wiki/Frank_Ocean",
		Title:       "Frank Ocean - Quotes",
		Publisher:   "Wikiquote",
		License:     "CC-BY-SA-4.0",
		RetrievedAt: "2026-03-29T00:00:00Z",
	}
	if err := store.UpsertSource(ctx, source); err != nil {
		t.Fatalf("UpsertSource() error = %v", err)
	}
	if err := store.UpsertQuote(ctx, models.Quote{
		Text:             "Work hard in silence.",
		ArtistID:         artistID,
		ArtistName:       "Frank Ocean",
		SourceID:         source.SourceID,
		SourceType:       "wikiquote",
		WorkTitle:        "Quotes",
		ProvenanceStatus: "source_attributed",
		ConfidenceScore:  0.9,
		License:          source.License,
		FirstSeenAt:      source.RetrievedAt,
		LastVerifiedAt:   source.RetrievedAt,
		Source:           &source,
	}); err != nil {
		t.Fatalf("UpsertQuote() error = %v", err)
	}

	quotes, err := store.ListQuotes(ctx, models.QuoteFilters{ArtistID: artistID, Limit: 10})
	if err != nil {
		t.Fatalf("ListQuotes() error = %v", err)
	}
	if len(quotes.Data) != 2 {
		t.Fatalf("expected quote reconciliation to keep total at 2, got %d", len(quotes.Data))
	}
	var upgraded bool
	for _, quote := range quotes.Data {
		if quote.Text == "Work hard in silence." && quote.ProvenanceStatus == "source_attributed" {
			upgraded = true
		}
	}
	if !upgraded {
		t.Fatalf("expected legacy quote to be upgraded with provenance")
	}
}

func osWriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
