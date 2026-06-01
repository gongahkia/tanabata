package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

func TestOpenAPIContractRuntimeResponses(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	ctx := context.Background()
	if err := store.RecordProviderRun(ctx, catalog.ProviderRun{
		RunID:      "contract-run-1",
		Provider:   "wikiquote",
		Status:     "success",
		StartedAt:  time.Now().UTC().Add(-time.Minute),
		FinishedAt: time.Now().UTC(),
		Details:    "quotes=1",
	}); err != nil {
		t.Fatalf("RecordProviderRun() error = %v", err)
	}
	if err := store.RecordJob(ctx, models.JobRun{
		JobID:      "contract-job-1",
		Name:       "contract-refresh",
		Scope:      "bootstrap",
		Status:     "succeeded",
		StartedAt:  time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Details:    "bootstrap,succeeded",
	}); err != nil {
		t.Fatalf("RecordJob() error = %v", err)
	}
	if err := store.RecordJobItem(ctx, models.JobItem{
		JobItemID:  "contract-item-1",
		JobID:      "contract-job-1",
		Provider:   "tanabata_curated",
		Target:     "bootstrap:data/curated_quotes.json",
		Status:     "succeeded",
		StartedAt:  time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Details:    "imported=4 curated quotes",
	}); err != nil {
		t.Fatalf("RecordJobItem() error = %v", err)
	}
	if _, err := store.CaptureIngestionSnapshot(ctx, "contract-job-1", "after", time.Now().UTC()); err != nil {
		t.Fatalf("CaptureIngestionSnapshot() error = %v", err)
	}
	if err := store.RecordIngestionAuditEvent(ctx, models.IngestionAuditEvent{
		EventID:    "contract-audit-1",
		JobID:      "contract-job-1",
		JobItemID:  "contract-item-1",
		Provider:   "tanabata_curated",
		Target:     "bootstrap:data/curated_quotes.json",
		Action:     "import",
		Status:     "succeeded",
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
		Details:    "imported=4 curated quotes",
	}); err != nil {
		t.Fatalf("RecordIngestionAuditEvent() error = %v", err)
	}

	quotes, err := store.ListQuotes(ctx, models.QuoteFilters{Limit: 10})
	if err != nil || len(quotes.Data) == 0 {
		t.Fatalf("ListQuotes() err=%v count=%d", err, len(quotes.Data))
	}
	artistID := quotes.Data[0].ArtistID
	quoteID := quotes.Data[0].QuoteID
	sourceID := quotes.Data[0].SourceID

	// Seed lineage entities so the new contract endpoints have records to return.
	dataDir := filepath.Join("..", "..", "data")
	if _, err := store.SeedCuratedSamples(ctx, filepath.Join(dataDir, "curated_samples.json"), "contract-lineage-samples"); err != nil {
		t.Fatalf("SeedCuratedSamples error = %v", err)
	}
	if _, err := store.SeedCuratedWorks(ctx, filepath.Join(dataDir, "curated_works.json"), "contract-lineage-works"); err != nil {
		t.Fatalf("SeedCuratedWorks error = %v", err)
	}
	if _, err := store.SeedCuratedPerformances(ctx, filepath.Join(dataDir, "curated_performances.json"), "contract-lineage-perfs"); err != nil {
		t.Fatalf("SeedCuratedPerformances error = %v", err)
	}
	if _, err := store.SeedCuratedMisquotes(ctx, filepath.Join(dataDir, "curated_misquotes.json"), "contract-lineage-misquotes"); err != nil {
		t.Fatalf("SeedCuratedMisquotes error = %v", err)
	}
	if err := store.RefreshSearchIndices(ctx); err != nil {
		t.Fatalf("RefreshSearchIndices error = %v", err)
	}

	works, err := store.ListWorks(ctx, models.WorkFilters{Limit: 5})
	if err != nil || len(works.Data) == 0 {
		t.Fatalf("ListWorks() err=%v count=%d", err, len(works.Data))
	}
	workID := works.Data[0].WorkID
	recordings, err := store.ListRecordings(ctx, models.RecordingFilters{Limit: 100})
	if err != nil || len(recordings.Data) == 0 {
		t.Fatalf("ListRecordings() err=%v count=%d", err, len(recordings.Data))
	}
	// Pick a recording with a documented sample edge for the sample lookup.
	var recordingID, sampleID string
	for _, recording := range recordings.Data {
		edges, err := store.OutgoingSamples(ctx, recording.RecordingID)
		if err != nil {
			t.Fatalf("OutgoingSamples error = %v", err)
		}
		if len(edges) > 0 {
			recordingID = recording.RecordingID
			sampleID = edges[0].SampleID
			break
		}
	}
	if sampleID == "" {
		t.Fatalf("expected at least one sample edge after seeding lineage bundles")
	}
	performances, err := store.ListPerformances(ctx, models.PerformanceFilters{Limit: 5})
	if err != nil || len(performances.Data) == 0 {
		t.Fatalf("ListPerformances() err=%v count=%d", err, len(performances.Data))
	}
	performanceID := performances.Data[0].PerformanceID
	performanceArtistID := performances.Data[0].ArtistID
	claims, err := store.ListClaims(ctx, models.ClaimFilters{Limit: 5})
	if err != nil || len(claims.Data) == 0 {
		t.Fatalf("ListClaims() err=%v count=%d", err, len(claims.Data))
	}
	claimID := claims.Data[0].ClaimID

	validator := newOpenAPIContractValidator(t)
	tests := []struct {
		name string
		path string
	}{
		{name: "list artists", path: "/v1/artists"},
		{name: "artist detail", path: "/v1/artists/" + artistID},
		{name: "artist quotes", path: "/v1/artists/" + artistID + "/quotes?limit=5"},
		{name: "artist recordings", path: "/v1/artists/" + performanceArtistID + "/recordings?limit=5"},
		{name: "artist performances", path: "/v1/artists/" + performanceArtistID + "/performances?limit=5"},
		{name: "artist performance stats", path: "/v1/artists/" + performanceArtistID + "/performances/stats"},
		{name: "quote list", path: "/v1/quotes?limit=5"},
		{name: "quote detail", path: "/v1/quotes/" + quoteID},
		{name: "quote provenance", path: "/v1/quotes/" + quoteID + "/provenance"},
		{name: "quote lineage", path: "/v1/quotes/" + quoteID + "/lineage"},
		{name: "works list", path: "/v1/works?limit=5"},
		{name: "work detail", path: "/v1/works/" + workID},
		{name: "work recordings (covers)", path: "/v1/works/" + workID + "/recordings"},
		{name: "work credits", path: "/v1/works/" + workID + "/credits"},
		{name: "work performances", path: "/v1/works/" + workID + "/performances?limit=5"},
		{name: "recordings list", path: "/v1/recordings?limit=5"},
		{name: "recording detail", path: "/v1/recordings/" + recordingID},
		{name: "recording outgoing samples", path: "/v1/recordings/" + recordingID + "/samples"},
		{name: "recording incoming samples", path: "/v1/recordings/" + recordingID + "/sampled_by"},
		{name: "sample detail", path: "/v1/samples/" + sampleID},
		{name: "performance detail", path: "/v1/performances/" + performanceID},
		{name: "claims list", path: "/v1/claims?limit=5"},
		{name: "claim detail", path: "/v1/claims/" + claimID},
		{name: "disputes", path: "/v1/disputes?limit=10"},
		{name: "source detail", path: "/v1/sources/" + sourceID},
		{name: "providers", path: "/v1/providers"},
		{name: "provider runs", path: "/v1/providers/wikiquote/runs?limit=5"},
		{name: "jobs", path: "/v1/jobs?limit=5"},
		{name: "job detail", path: "/v1/jobs/contract-job-1"},
		{name: "job snapshots", path: "/v1/jobs/contract-job-1/snapshots?limit=5"},
		{name: "job audit", path: "/v1/jobs/contract-job-1/audit?limit=5"},
		{name: "timeline", path: "/v1/timeline?limit=5"},
		{name: "review queue", path: "/v1/review/queue?limit=5"},
		{name: "stale quote review", path: "/v1/review/stale?limit=5"},
		{name: "search", path: "/v1/search?q=frank"},
		{name: "stats", path: "/v1/stats"},
		{name: "integrity", path: "/v1/integrity"},
		{name: "lyrics", path: "/v1/lyrics?artist=Coldplay&track=Yellow&provider=lrclib"},
	}

	if err := store.SetProviderCache(ctx, "lrclib", "lyrics", search.StableHash("coldplay", "yellow"), `{"provider":"lrclib","artist":"Coldplay","track":"Yellow","lyrics":"Look at the stars","source_url":"https://lrclib.net"}`, time.Hour); err != nil {
		t.Fatalf("SetProviderCache() error = %v", err)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			server.Router().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s status = %d, want 200 body=%s", tc.path, recorder.Code, recorder.Body.String())
			}
			validator.validateResponse(t, request, recorder)
		})
	}
}

type openAPIContractValidator struct {
	t      *testing.T
	router routers.Router
}

func newOpenAPIContractValidator(t *testing.T) *openAPIContractValidator {
	t.Helper()

	specPath := filepath.Join("..", "..", "..", "openapi", "openapi.json")
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("LoadFromFile(%s) error = %v", specPath, err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("Validate(%s) error = %v", specPath, err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	return &openAPIContractValidator{t: t, router: router}
}

func (v *openAPIContractValidator) validateResponse(t *testing.T, request *http.Request, recorder *httptest.ResponseRecorder) {
	t.Helper()

	contractRequest, err := http.NewRequestWithContext(
		request.Context(),
		request.Method,
		"http://localhost:8080"+request.URL.RequestURI(),
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	route, pathParams, err := v.router.FindRoute(contractRequest)
	if err != nil {
		t.Fatalf("FindRoute(%s) error = %v", request.URL.RequestURI(), err)
	}
	requestInput := &openapi3filter.RequestValidationInput{
		Request:    contractRequest,
		PathParams: pathParams,
		Route:      route,
	}
	if err := openapi3filter.ValidateRequest(context.Background(), requestInput); err != nil {
		t.Fatalf("ValidateRequest(%s) error = %v", request.URL.RequestURI(), err)
	}
	responseInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestInput,
		Status:                 recorder.Code,
		Header:                 recorder.Header(),
	}
	responseInput.SetBodyBytes(recorder.Body.Bytes())
	if err := openapi3filter.ValidateResponse(context.Background(), responseInput); err != nil {
		t.Fatalf("ValidateResponse(%s) error = %v body=%s", request.URL.RequestURI(), err, recorder.Body.String())
	}
}
