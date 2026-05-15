package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apiServer "github.com/gongahkia/tanabata/api/internal/api"
	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/providers"
	"github.com/gongahkia/tanabata/api/internal/search"
	"github.com/gongahkia/tanabata/api/internal/testutil"
)

type fakeEnricher struct {
	store *catalog.Store
}

func (f fakeEnricher) EnrichArtist(ctx context.Context, query string) (providers.EnrichResult, error) {
	artistID, err := f.store.ResolveArtistID(ctx, query)
	if err != nil {
		return providers.EnrichResult{}, err
	}
	source := models.Source{
		SourceID:    search.SourceID("wikiquote", "https://archive.tanabata.dev/enriched/"+search.Slug(query)),
		Provider:    "wikiquote",
		URL:         "https://archive.tanabata.dev/enriched/" + search.Slug(query),
		Title:       query + " Enriched Quotes",
		Publisher:   "Tanabata Archive",
		License:     "editorial_excerpt",
		RetrievedAt: "2026-04-05T00:00:00Z",
	}
	if err := f.store.UpsertSource(ctx, source); err != nil {
		return providers.EnrichResult{}, err
	}
	if err := f.store.UpsertQuote(ctx, models.Quote{
		Text:             "Discipline makes the feeling repeatable.",
		ArtistID:         artistID,
		ArtistName:       query,
		SourceID:         source.SourceID,
		SourceType:       "wikiquote",
		WorkTitle:        "Enriched Quotes",
		Tags:             []string{"discipline"},
		ProvenanceStatus: "source_attributed",
		ConfidenceScore:  0.94,
		ProviderOrigin:   "wikiquote",
		Evidence:         []string{"Injected by fake enricher for ingestion E2E coverage."},
		License:          source.License,
		FirstSeenAt:      source.RetrievedAt,
		LastVerifiedAt:   source.RetrievedAt,
		Source:           &source,
	}); err != nil {
		return providers.EnrichResult{}, err
	}
	if err := f.store.RecordProviderRun(ctx, catalog.ProviderRun{
		RunID:      "wikiquote-run-1",
		Provider:   "wikiquote",
		Status:     "success",
		StartedAt:  time.Now().UTC().Add(-time.Minute),
		FinishedAt: time.Now().UTC(),
		Details:    "quotes=1",
	}); err != nil {
		return providers.EnrichResult{}, err
	}
	return providers.EnrichResult{
		ArtistID: artistID,
		Target:   query,
		Status:   "succeeded",
		Details:  "wikiquote:quotes=1",
	}, nil
}

func TestJobHelpers(t *testing.T) {
	item := newJobItem("job-1", "wikiquote", "Frank Ocean")
	if item.JobID != "job-1" || item.Provider != "wikiquote" || item.Status != "running" || item.JobItemID == "" || item.StartedAt == "" {
		t.Fatalf("unexpected job item %+v", item)
	}

	if got := overallJobStatus([]string{"succeeded", "partial"}); got != "partial" {
		t.Fatalf("overallJobStatus() = %q, want partial", got)
	}
	if got := overallJobStatus([]string{"succeeded", "failed"}); got != "failed" {
		t.Fatalf("overallJobStatus() = %q, want failed", got)
	}
	if got := overallJobStatus(nil); got != "succeeded" {
		t.Fatalf("overallJobStatus() = %q, want succeeded", got)
	}

	if got := jobScope(true, true, " Frank Ocean "); got != "bootstrap,all,artist:Frank Ocean" {
		t.Fatalf("jobScope() = %q", got)
	}
}

func TestFinalizeJobPersistsState(t *testing.T) {
	tempDir := t.TempDir()
	store, err := catalog.Open(filepath.Join(tempDir, "catalog.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	job := models.JobRun{
		JobID:     "job-1",
		Name:      "catalog-refresh",
		Status:    "running",
		StartedAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
	}
	if err := store.RecordJob(ctx, job); err != nil {
		t.Fatalf("RecordJob() error = %v", err)
	}

	finalizeJob(ctx, store, job, "partial", "bootstrap,partial", nil)

	stored, err := store.JobByID(ctx, "job-1")
	if err != nil || stored == nil {
		t.Fatalf("JobByID() err=%v job=%+v", err, stored)
	}
	if stored.Status != "partial" || stored.Details != "bootstrap,partial" || stored.FinishedAt == "" {
		t.Fatalf("unexpected stored job %+v", stored)
	}
}

func TestRunBootstrapsCuratedQuotesAndExposesAPIState(t *testing.T) {
	tempDir := t.TempDir()
	store, err := catalog.Open(filepath.Join(tempDir, "catalog.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	opts := options{
		bootstrap:   true,
		allArtists:  false,
		artistName:  "Frank Ocean",
		catalogPath: filepath.Join(tempDir, "catalog.sqlite"),
		legacyPath:  testutil.WriteLegacyQuotes(t, tempDir),
		curatedPath: testutil.WriteCuratedQuotes(t, tempDir),
		jobName:     "catalog-e2e",
	}
	if err := run(context.Background(), opts, store, fakeEnricher{store: store}); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	jobs, err := store.ListJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) != 1 || len(jobs[0].Items) != 3 {
		t.Fatalf("expected one job with three items, got %+v", jobs)
	}
	if jobs[0].Status != "succeeded" {
		t.Fatalf("job status = %q, want succeeded", jobs[0].Status)
	}

	server := apiServer.NewServer(store, nil)
	tests := []struct {
		path string
		want string
	}{
		{path: "/v1/jobs", want: "catalog-e2e"},
		{path: "/v1/providers", want: "tanabata_curated"},
		{path: "/v1/search?q=discipline", want: "Discipline makes the feeling repeatable."},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			server.Router().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", recorder.Code)
			}
			if body := recorder.Body.String(); !strings.Contains(body, tc.want) {
				t.Fatalf("expected %q in %s", tc.want, body)
			}
		})
	}

	quotes, err := store.ListQuotes(context.Background(), models.QuoteFilters{
		Query:            "precise enough",
		ProvenanceStatus: "verified",
		Limit:            10,
	})
	if err != nil || len(quotes.Data) == 0 {
		t.Fatalf("ListQuotes() err=%v quotes=%+v", err, quotes.Data)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/quotes/"+quotes.Data[0].QuoteID+"/provenance", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("provenance status = %d, want 200", recorder.Code)
	}
	var response models.APIResponse[models.QuoteProvenance]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ProvenanceStatus != "verified" || response.Data.ProviderOrigin != "tanabata_curated" {
		t.Fatalf("unexpected provenance %+v", response.Data)
	}
}
