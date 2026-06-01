package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

// newLineageStore opens a fresh catalog and runs all curated lineage seeders against it.
func newLineageStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "catalog.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	// data/ lives at api/data — tests run from api/internal/catalog, so step up two levels.
	dataDir := filepath.Join("..", "..", "data")
	bundles := []struct {
		name string
		seed func() error
	}{
		{"samples", func() error {
			_, err := store.SeedCuratedSamples(ctx, filepath.Join(dataDir, "curated_samples.json"), "test-job-samples")
			return err
		}},
		{"works", func() error {
			_, err := store.SeedCuratedWorks(ctx, filepath.Join(dataDir, "curated_works.json"), "test-job-works")
			return err
		}},
		{"performances", func() error {
			_, err := store.SeedCuratedPerformances(ctx, filepath.Join(dataDir, "curated_performances.json"), "test-job-perfs")
			return err
		}},
		{"misquotes", func() error {
			_, err := store.SeedCuratedMisquotes(ctx, filepath.Join(dataDir, "curated_misquotes.json"), "test-job-misquotes")
			return err
		}},
	}
	for _, bundle := range bundles {
		path := filepath.Join(dataDir, "curated_"+bundle.name+".json")
		if _, err := os.Stat(path); err != nil {
			t.Skipf("seed bundle %s not present at %s: %v", bundle.name, path, err)
		}
		if err := bundle.seed(); err != nil {
			t.Fatalf("seed %s: %v", bundle.name, err)
		}
	}
	if err := store.RefreshSearchIndices(ctx); err != nil {
		t.Fatalf("RefreshSearchIndices() error = %v", err)
	}
	return store
}

func TestCuratedSamplesProduceVerifiableLineage(t *testing.T) {
	store := newLineageStore(t)
	ctx := context.Background()
	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if got := stats["samples"].(int); got < 5 {
		t.Fatalf("samples count = %d, want >= 5", got)
	}

	// Find Rapper's Delight by title and inspect outgoing samples (its ancestors).
	recordings, err := store.ListRecordings(ctx, models.RecordingFilters{Query: "Rapper's Delight"})
	if err != nil {
		t.Fatalf("ListRecordings() error = %v", err)
	}
	if len(recordings.Data) == 0 {
		t.Fatalf("expected Rapper's Delight recording to be seeded")
	}
	derivative := recordings.Data[0]
	edges, err := store.OutgoingSamples(ctx, derivative.RecordingID)
	if err != nil {
		t.Fatalf("OutgoingSamples() error = %v", err)
	}
	if len(edges) == 0 {
		t.Fatalf("expected Rapper's Delight to have a documented sample source")
	}
	if edges[0].SourceRecording.Title == "" {
		t.Fatalf("sample edge missing hydrated source recording")
	}
	if edges[0].Claim == nil || edges[0].Claim.SupportingCount == 0 {
		t.Fatalf("sample edge missing supporting claim evidence")
	}
}

func TestCuratedWorksProduceCoverLineage(t *testing.T) {
	store := newLineageStore(t)
	ctx := context.Background()

	works, err := store.ListWorks(ctx, models.WorkFilters{Query: "Hallelujah"})
	if err != nil {
		t.Fatalf("ListWorks() error = %v", err)
	}
	if len(works.Data) == 0 {
		t.Fatalf("expected Hallelujah work to be seeded")
	}
	work := works.Data[0]
	recordings, err := store.WorkRecordings(ctx, work.WorkID)
	if err != nil {
		t.Fatalf("WorkRecordings() error = %v", err)
	}
	if len(recordings) < 2 {
		t.Fatalf("expected at least 2 covers of Hallelujah, got %d", len(recordings))
	}
	if !recordings[0].IsOriginal {
		t.Fatalf("first recording should be the original; got artist=%s title=%s", recordings[0].ArtistName, recordings[0].Title)
	}
	credits, err := store.WorkCredits(ctx, work.WorkID)
	if err != nil {
		t.Fatalf("WorkCredits() error = %v", err)
	}
	if len(credits) == 0 {
		t.Fatalf("expected at least one credit for Hallelujah")
	}
}

func TestDisputedCreditsAppearInDisputes(t *testing.T) {
	store := newLineageStore(t)
	ctx := context.Background()
	disputes, err := store.Disputes(ctx, 100)
	if err != nil {
		t.Fatalf("Disputes() error = %v", err)
	}
	if len(disputes) == 0 {
		t.Fatalf("expected curated disputes (Bittersweet Symphony credits, misquotes, Blurred Lines sample)")
	}
	kinds := map[string]int{}
	for _, dispute := range disputes {
		kinds[dispute.Claim.Kind]++
	}
	if kinds["credit"] == 0 {
		t.Fatalf("expected at least one disputed credit claim; got kinds=%v", kinds)
	}
	if kinds["attribution"] == 0 {
		t.Fatalf("expected at least one disputed attribution claim (misquote); got kinds=%v", kinds)
	}
}

func TestPerformanceStatsExposesGap(t *testing.T) {
	store := newLineageStore(t)
	ctx := context.Background()
	artistID, err := store.ResolveArtistID(ctx, "Radiohead")
	if err != nil {
		t.Fatalf("ResolveArtistID() error = %v", err)
	}
	if artistID == "" {
		t.Fatalf("expected Radiohead artist to be seeded via performance bundle")
	}
	workID, err := store.ResolveOrCreateWork(ctx, "Creep", artistID, "")
	if err != nil {
		t.Fatalf("ResolveOrCreateWork() error = %v", err)
	}
	stats, err := store.PerformanceStats(ctx, artistID, workID)
	if err != nil {
		t.Fatalf("PerformanceStats() error = %v", err)
	}
	if stats.TotalPerformed < 2 {
		t.Fatalf("expected at least 2 Creep performances seeded, got %d", stats.TotalPerformed)
	}
	if stats.FirstPerformedAt == "" || stats.LastPerformedAt == "" {
		t.Fatalf("first/last performed timestamps must be set")
	}
	if stats.GapDays <= 0 {
		t.Fatalf("expected a positive gap between Creep performances, got %d", stats.GapDays)
	}
}

func TestMisquoteLineageCarriesRefutingEvidence(t *testing.T) {
	store := newLineageStore(t)
	ctx := context.Background()

	hendrixID, err := store.ResolveArtistID(ctx, "Jimi Hendrix")
	if err != nil {
		t.Fatalf("ResolveArtistID() error = %v", err)
	}
	if hendrixID == "" {
		t.Fatalf("expected Jimi Hendrix artist to be seeded")
	}
	quoteID := search.QuoteID(hendrixID, search.NormalizeText("Knowledge speaks, but wisdom listens."), "")
	lineage, err := store.QuoteLineage(ctx, quoteID)
	if err != nil {
		t.Fatalf("QuoteLineage() error = %v", err)
	}
	if lineage == nil {
		t.Fatalf("expected Hendrix misquote lineage to exist for quote_id=%s", quoteID)
	}
	if len(lineage.Refuting) == 0 {
		t.Fatalf("expected refuting evidence for the Hendrix misquote; got %+v", lineage)
	}

	// The Cobain misquote should carry a rival claim toward André Gide.
	cobainID, err := store.ResolveArtistID(ctx, "Kurt Cobain")
	if err != nil {
		t.Fatalf("ResolveArtistID() error = %v", err)
	}
	cobainQuoteID := search.QuoteID(cobainID, search.NormalizeText("I'd rather be hated for who I am than loved for who I am not."), "")
	cobainLineage, err := store.QuoteLineage(ctx, cobainQuoteID)
	if err != nil || cobainLineage == nil {
		t.Fatalf("QuoteLineage(cobain) error = %v lineage=%v", err, cobainLineage)
	}
	if len(cobainLineage.RivalClaims) == 0 {
		t.Fatalf("expected rival claim toward André Gide for the Cobain misquote")
	}
}

func TestQuoteMergeLogPersists(t *testing.T) {
	store := newLineageStore(t)
	ctx := context.Background()
	if err := store.RecordQuoteMerge(ctx, models.QuoteMergeLog{
		WinnerQuoteID: "winner-1",
		LoserQuoteID:  "loser-1",
		MergeScore:    98,
		Reason:        "fingerprint match",
		JobID:         "test-job",
	}); err != nil {
		t.Fatalf("RecordQuoteMerge() error = %v", err)
	}
	logs, err := store.QuoteMergeHistory(ctx, "winner-1")
	if err != nil || len(logs) != 1 {
		t.Fatalf("QuoteMergeHistory() err=%v len=%d", err, len(logs))
	}
}
