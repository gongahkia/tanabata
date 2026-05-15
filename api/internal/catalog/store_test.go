package catalog

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
	"github.com/gongahkia/tanabata/api/internal/testutil"
)

func newSeededStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "catalog.sqlite")
	legacyPath := testutil.WriteLegacyQuotes(t, tempDir)
	curatedPath := testutil.WriteCuratedQuotes(t, tempDir)
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	ctx := context.Background()
	if err := store.SeedFromLegacyJSON(ctx, legacyPath); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if imported, err := store.ImportCuratedQuotes(ctx, curatedPath); err != nil {
		t.Fatalf("import curated quotes: %v", err)
	} else if imported != len(testutil.CuratedQuotes()) {
		t.Fatalf("imported %d curated quotes, want %d", imported, len(testutil.CuratedQuotes()))
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
	if artists.Pagination.Total != 4 {
		t.Fatalf("ListArtists() total = %d, want 4", artists.Pagination.Total)
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
	if len(quotes.Data) != 3 {
		t.Fatalf("ListQuotes() len = %d, want 3", len(quotes.Data))
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
	if len(quotes.Data) != 3 {
		t.Fatalf("expected quote reconciliation to keep total at 3, got %d", len(quotes.Data))
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

func TestUpsertQuoteMergesNearDuplicateAcrossProviders(t *testing.T) {
	store, ctx := newSeededStore(t)
	defer store.Close()

	artistID, err := store.ResolveArtistID(ctx, "Frank Ocean")
	if err != nil {
		t.Fatalf("ResolveArtistID() error = %v", err)
	}
	source := models.Source{
		SourceID:    search.SourceID("wikiquote", "https://en.wikiquote.org/wiki/Frank_Ocean#Quotes"),
		Provider:    "wikiquote",
		URL:         "https://en.wikiquote.org/wiki/Frank_Ocean#Quotes",
		Title:       "Frank Ocean - Quotes",
		Publisher:   "Wikiquote",
		License:     "CC-BY-SA-4.0",
		RetrievedAt: "2026-05-01T00:00:00Z",
	}
	if err := store.UpsertSource(ctx, source); err != nil {
		t.Fatalf("UpsertSource() error = %v", err)
	}
	if err := store.UpsertQuote(ctx, models.Quote{
		Text:             "Work hard in silence",
		ArtistID:         artistID,
		ArtistName:       "Frank Ocean",
		SourceID:         source.SourceID,
		SourceType:       "wikiquote",
		WorkTitle:        "Quotes",
		Tags:             []string{"discipline"},
		ProvenanceStatus: "source_attributed",
		ConfidenceScore:  0.92,
		ProviderOrigin:   "wikiquote",
		Evidence:         []string{"Matched against a maintained Wikiquote page."},
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
	if len(quotes.Data) != 3 {
		t.Fatalf("expected near-duplicate merge to keep total at 3, got %d", len(quotes.Data))
	}
	var merged *models.Quote
	for idx := range quotes.Data {
		if search.ShouldMergeQuotes(quotes.Data[idx].Text, "Work hard in silence.") {
			merged = &quotes.Data[idx]
			break
		}
	}
	if merged == nil {
		t.Fatalf("expected merged quote in %+v", quotes.Data)
	}
	if merged.ProvenanceStatus != "source_attributed" || merged.ProviderOrigin != "wikiquote" {
		t.Fatalf("unexpected merged quote %+v", merged)
	}
	if len(merged.Evidence) < 2 || len(merged.Tags) != 1 || merged.Source == nil || merged.Source.Provider != "wikiquote" {
		t.Fatalf("expected merged evidence, tags, and source metadata, got %+v", merged)
	}
}

func TestUpsertQuoteKeepsDistinctQuotesWhenBelowMergeThreshold(t *testing.T) {
	store, ctx := newSeededStore(t)
	defer store.Close()

	artistID, err := store.ResolveArtistID(ctx, "Frank Ocean")
	if err != nil {
		t.Fatalf("ResolveArtistID() error = %v", err)
	}
	if err := store.UpsertQuote(ctx, models.Quote{
		Text:             "Work harder in private.",
		ArtistID:         artistID,
		ArtistName:       "Frank Ocean",
		ProvenanceStatus: "provider_attributed",
		ConfidenceScore:  0.65,
		ProviderOrigin:   "tanabata_curated",
		Evidence:         []string{"Intentionally distinct quote variant for merge-boundary coverage."},
		FirstSeenAt:      "2026-05-02T00:00:00Z",
		LastVerifiedAt:   "2026-05-02T00:00:00Z",
	}); err != nil {
		t.Fatalf("UpsertQuote() error = %v", err)
	}

	quotes, err := store.ListQuotes(ctx, models.QuoteFilters{ArtistID: artistID, Limit: 10})
	if err != nil {
		t.Fatalf("ListQuotes() error = %v", err)
	}
	if len(quotes.Data) != 4 {
		t.Fatalf("expected distinct quote to increase total to 4, got %d", len(quotes.Data))
	}
}

func TestUpsertQuoteDoesNotDowngradeBetterProvenanceOnMerge(t *testing.T) {
	store, ctx := newSeededStore(t)
	defer store.Close()

	artistID, err := store.ResolveArtistID(ctx, "Frank Ocean")
	if err != nil {
		t.Fatalf("ResolveArtistID() error = %v", err)
	}
	if err := store.UpsertQuote(ctx, models.Quote{
		Text:             "Make the work precise enough that it can survive your mood",
		ArtistID:         artistID,
		ArtistName:       "Frank Ocean",
		ProvenanceStatus: "provider_attributed",
		ConfidenceScore:  0.5,
		ProviderOrigin:   "provider_digest",
		Evidence:         []string{"Provider-only paraphrase candidate."},
		FirstSeenAt:      "2026-04-05T00:00:00Z",
		LastVerifiedAt:   "2026-04-05T00:00:00Z",
	}); err != nil {
		t.Fatalf("UpsertQuote() error = %v", err)
	}

	quotes, err := store.ListQuotes(ctx, models.QuoteFilters{
		Query:  "precise enough",
		Limit:  10,
		Offset: 0,
	})
	if err != nil || len(quotes.Data) == 0 {
		t.Fatalf("ListQuotes() err=%v quotes=%+v", err, quotes.Data)
	}
	if quotes.Data[0].ProvenanceStatus != "verified" || quotes.Data[0].ProviderOrigin != "tanabata_curated" {
		t.Fatalf("expected stronger provenance to survive merge, got %+v", quotes.Data[0])
	}
	if len(quotes.Data[0].Evidence) < 3 {
		t.Fatalf("expected evidence to accumulate, got %+v", quotes.Data[0].Evidence)
	}
}

func TestImportCuratedQuotesExpandsProvenanceCoverage(t *testing.T) {
	store, ctx := newSeededStore(t)
	defer store.Close()

	tests := []struct {
		status string
		want   string
	}{
		{status: "verified", want: "Make the work precise enough that it can survive your mood."},
		{status: "source_attributed", want: "The part people keep is usually the part that felt dangerous to say."},
		{status: "provider_attributed", want: "You can hear when a song is trying to be brave instead of simply being honest."},
		{status: "ambiguous", want: "If the quote keeps changing between sources, the disagreement is part of the record."},
		{status: "needs_review", want: "Work hard in silence."},
	}
	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			quotes, err := store.ListQuotes(ctx, models.QuoteFilters{
				ProvenanceStatus: tc.status,
				Limit:            10,
			})
			if err != nil {
				t.Fatalf("ListQuotes() error = %v", err)
			}
			if len(quotes.Data) == 0 {
				t.Fatalf("expected quote for status %s", tc.status)
			}
			found := false
			for _, quote := range quotes.Data {
				if quote.Text == tc.want {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected %q in results %+v", tc.want, quotes.Data)
			}
		})
	}
}

func TestProviderCacheJobsAndSummaries(t *testing.T) {
	store, ctx := newSeededStore(t)
	defer store.Close()

	if err := store.SetProviderCache(ctx, "lrclib", "lyrics", "coldplay-yellow", `{"lyrics":"Look at the stars"}`, time.Hour); err != nil {
		t.Fatalf("SetProviderCache() error = %v", err)
	}
	payload, refreshedAt, expiresAt, ok, err := store.GetProviderCache(ctx, "lrclib", "lyrics", "coldplay-yellow")
	if err != nil {
		t.Fatalf("GetProviderCache() error = %v", err)
	}
	if !ok || payload == "" || refreshedAt == "" || expiresAt == "" {
		t.Fatalf("expected cached payload, got ok=%v payload=%q", ok, payload)
	}

	startedAt := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	finishedAt := time.Now().UTC().Format(time.RFC3339)
	job := models.JobRun{
		JobID:        "job-1",
		Name:         "catalog-refresh",
		Scope:        "bootstrap",
		Status:       "partial",
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
		Details:      "bootstrap,partial",
		ErrorMessage: "",
	}
	if err := store.RecordJob(ctx, job); err != nil {
		t.Fatalf("RecordJob() error = %v", err)
	}
	item := models.JobItem{
		JobItemID:  "item-1",
		JobID:      job.JobID,
		Provider:   "quotefancy",
		Target:     "bootstrap:data",
		Status:     "succeeded",
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		Details:    "bootstrapped",
	}
	if err := store.RecordJobItem(ctx, item); err != nil {
		t.Fatalf("RecordJobItem() error = %v", err)
	}
	jobs, err := store.ListJobs(ctx, 10)
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) == 0 || len(jobs[0].Items) == 0 {
		t.Fatalf("expected jobs with items, got %+v", jobs)
	}
	jobByID, err := store.JobByID(ctx, "job-1")
	if err != nil || jobByID == nil {
		t.Fatalf("JobByID() err=%v job=%+v", err, jobByID)
	}

	runStarted := time.Now().UTC().Add(-2 * time.Hour)
	if err := store.RecordProviderRun(ctx, ProviderRun{
		RunID:      "run-1",
		Provider:   "wikiquote",
		Status:     "success",
		StartedAt:  runStarted,
		FinishedAt: runStarted.Add(time.Minute),
		Details:    "quotes=2",
	}); err != nil {
		t.Fatalf("RecordProviderRun() error = %v", err)
	}
	if err := store.RecordProviderError(ctx, ProviderError{
		ErrorID:    "error-1",
		Provider:   "wikiquote",
		OccurredAt: time.Now().UTC(),
		Context:    "Frank Ocean",
		Message:    "rate limited",
	}); err != nil {
		t.Fatalf("RecordProviderError() error = %v", err)
	}

	runs, err := store.ProviderRuns(ctx, "wikiquote", 10)
	if err != nil || len(runs) != 1 {
		t.Fatalf("ProviderRuns() err=%v runs=%+v", err, runs)
	}
	failures, err := store.ProviderErrors(ctx, "wikiquote", 10)
	if err != nil || len(failures) != 1 {
		t.Fatalf("ProviderErrors() err=%v failures=%+v", err, failures)
	}
	summaries, err := store.ProviderSummaries(ctx, []models.ProviderSummary{{Provider: "wikiquote", Category: "enrichment", Enabled: true}})
	if err != nil {
		t.Fatalf("ProviderSummaries() error = %v", err)
	}
	if len(summaries) != 1 || summaries[0].RecentErrorCount != 1 || summaries[0].LastSuccessful == "" {
		t.Fatalf("unexpected provider summary %+v", summaries)
	}
}

func TestProviderCacheExpiresAndRefreshes(t *testing.T) {
	store, ctx := newSeededStore(t)
	defer store.Close()

	if err := store.SetProviderCache(ctx, "lrclib", "lyrics", "coldplay-yellow", `{"lyrics":"stale"}`, -time.Minute); err != nil {
		t.Fatalf("SetProviderCache() error = %v", err)
	}
	payload, refreshedAt, expiresAt, ok, err := store.GetProviderCache(ctx, "lrclib", "lyrics", "coldplay-yellow")
	if err != nil {
		t.Fatalf("GetProviderCache() error = %v", err)
	}
	if ok || payload != "" || refreshedAt != "" || expiresAt != "" {
		t.Fatalf("expected expired cache miss, got ok=%v payload=%q refreshed=%q expires=%q", ok, payload, refreshedAt, expiresAt)
	}
}

func TestQuoteProvenanceFiltersAndRefreshSearch(t *testing.T) {
	store, ctx := newSeededStore(t)
	defer store.Close()

	artistID, err := store.ResolveArtistID(ctx, "Frank Ocean")
	if err != nil {
		t.Fatalf("ResolveArtistID() error = %v", err)
	}
	source := models.Source{
		SourceID:    search.SourceID("wikiquote", "https://en.wikiquote.org/wiki/Frank_Ocean#Quotes"),
		Provider:    "wikiquote",
		URL:         "https://en.wikiquote.org/wiki/Frank_Ocean#Quotes",
		Title:       "Frank Ocean - Quotes",
		Publisher:   "Wikiquote",
		License:     "CC-BY-SA-4.0",
		RetrievedAt: "2026-03-29T00:00:00Z",
	}
	if err := store.UpsertSource(ctx, source); err != nil {
		t.Fatalf("UpsertSource() error = %v", err)
	}
	if err := store.UpsertQuote(ctx, models.Quote{
		Text:             "Be yourself.",
		ArtistID:         artistID,
		ArtistName:       "Frank Ocean",
		SourceID:         source.SourceID,
		SourceType:       "wikiquote",
		WorkTitle:        "Quotes",
		ProvenanceStatus: "verified",
		ConfidenceScore:  1,
		ProviderOrigin:   "wikiquote",
		Evidence:         []string{"Verified manually", "Source URL: " + source.URL},
		License:          source.License,
		FirstSeenAt:      source.RetrievedAt,
		LastVerifiedAt:   source.RetrievedAt,
		Source:           &source,
	}); err != nil {
		t.Fatalf("UpsertQuote() error = %v", err)
	}

	filtered, err := store.ListQuotes(ctx, models.QuoteFilters{
		ArtistID:         artistID,
		ProvenanceStatus: "verified",
		Limit:            10,
	})
	if err != nil {
		t.Fatalf("ListQuotes() error = %v", err)
	}
	if len(filtered.Data) == 0 {
		t.Fatalf("unexpected filtered quotes %+v", filtered.Data)
	}
	var selected *models.Quote
	for idx := range filtered.Data {
		if filtered.Data[idx].Text == "Be yourself." {
			selected = &filtered.Data[idx]
			break
		}
	}
	if selected == nil {
		t.Fatalf("expected verified upgraded quote in %+v", filtered.Data)
	}

	provenance, err := store.QuoteProvenance(ctx, selected.QuoteID)
	if err != nil {
		t.Fatalf("QuoteProvenance() error = %v", err)
	}
	if provenance == nil || provenance.ProviderOrigin != "wikiquote" || len(provenance.Evidence) < 2 {
		t.Fatalf("unexpected provenance %+v", provenance)
	}

	if err := store.RefreshSearchIndices(ctx); err != nil {
		t.Fatalf("RefreshSearchIndices() error = %v", err)
	}
	searchResponse, err := store.Search(ctx, "yourself")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(searchResponse.Data.Quotes) == 0 {
		t.Fatalf("expected refreshed search indices to find quote")
	}
}
