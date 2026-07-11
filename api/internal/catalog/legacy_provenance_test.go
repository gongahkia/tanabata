package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gongahkia/tanabata/api/internal/models"
)

func TestSeedFromLegacyJSONPreservesWikiquoteProvenance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quotes.json")
	content := `[{"author":"Nina Simone","text":"You have to learn to get up from the table when love is no longer being served.","source_url":"https://en.wikiquote.org/wiki/Nina_Simone","license":"CC-BY-SA-4.0","retrieved_at":"2026-07-11T00:00:00Z","attribution_text":"Wikiquote contributors, Nina Simone, CC-BY-SA-4.0"}]`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(filepath.Join(dir, "catalog.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.SeedFromLegacyJSON(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	quotes, err := store.ListQuotes(context.Background(), models.QuoteFilters{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes.Data) != 1 || quotes.Data[0].Source == nil {
		t.Fatalf("quote provenance missing: %+v", quotes.Data)
	}
	source := quotes.Data[0].Source
	if source.Provider != "wikiquote" || source.License != "CC-BY-SA-4.0" || source.RetrievedAt != "2026-07-11T00:00:00Z" {
		t.Fatalf("source = %+v", source)
	}
	if len(quotes.Data[0].Evidence) != 1 || quotes.Data[0].Evidence[0] != "Wikiquote contributors, Nina Simone, CC-BY-SA-4.0" {
		t.Fatalf("evidence = %+v", quotes.Data[0].Evidence)
	}
}
