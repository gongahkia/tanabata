package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

func seededServer(t *testing.T) (*Server, *catalog.Store) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "catalog.sqlite")
	legacyPath := filepath.Join(tempDir, "quotes.json")
	payload := []models.LegacyQuote{
		{Author: "Frank Ocean", Text: "Work hard in silence."},
		{Author: "Taylor Swift", Text: "Just keep dancing."},
	}
	content, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := os.WriteFile(legacyPath, content, 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	store, err := catalog.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.SeedFromLegacyJSON(context.Background(), legacyPath); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	return NewServer(store, nil), store
}

func TestLegacyQuotesEndpoint(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/quotes", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var quotes []models.LegacyQuote
	if err := json.Unmarshal(recorder.Body.Bytes(), &quotes); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(quotes) != 2 {
		t.Fatalf("expected 2 legacy quotes, got %d", len(quotes))
	}
}

func TestLivenessAndReadinessEndpoints(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	for _, path := range []string{"/livez", "/readyz"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		server.Router().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, recorder.Code)
		}
	}
}

func TestV1SearchEndpoint(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search?q=frank", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var response models.APIResponse[models.SearchResults]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.Artists) == 0 || len(response.Data.Quotes) == 0 {
		t.Fatalf("expected artist and quote search results")
	}
}

func TestV1QuotesRandomEndpoint(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/quotes/random?artist=Frank%20Ocean", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var response models.APIResponse[models.Quote]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ArtistName != "Frank Ocean" {
		t.Fatalf("artist = %q, want Frank Ocean", response.Data.ArtistName)
	}
}

func TestQuoteProvenanceEndpoint(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	quotes, err := store.ListQuotes(context.Background(), models.QuoteFilters{Limit: 10})
	if err != nil || len(quotes.Data) == 0 {
		t.Fatalf("list quotes err=%v count=%d", err, len(quotes.Data))
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/quotes/"+quotes.Data[0].QuoteID+"/provenance", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var response models.APIResponse[models.QuoteProvenance]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ProvenanceStatus == "" || len(response.Data.Evidence) == 0 {
		t.Fatalf("expected provenance metadata, got %+v", response.Data)
	}
}

func TestProvidersEndpoint(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	if err := store.RecordProviderRun(context.Background(), catalog.ProviderRun{
		RunID:      "run-1",
		Provider:   "wikiquote",
		Status:     "success",
		StartedAt:  time.Now().UTC().Add(-time.Hour),
		FinishedAt: time.Now().UTC(),
		Details:    "quotes=1",
	}); err != nil {
		t.Fatalf("RecordProviderRun() error = %v", err)
	}
	if err := store.RecordProviderError(context.Background(), catalog.ProviderError{
		ErrorID:    "error-1",
		Provider:   "wikiquote",
		OccurredAt: time.Now().UTC(),
		Context:    "Frank Ocean",
		Message:    "rate limited",
	}); err != nil {
		t.Fatalf("RecordProviderError() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/providers", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var response models.APIResponse[[]models.ProviderSummary]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) == 0 {
		t.Fatalf("expected provider summaries")
	}
}

func TestProviderRunsErrorsAndJobsEndpoints(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	ctx := context.Background()
	if err := store.RecordProviderRun(ctx, catalog.ProviderRun{
		RunID:      "run-1",
		Provider:   "wikiquote",
		Status:     "success",
		StartedAt:  time.Now().UTC().Add(-time.Hour),
		FinishedAt: time.Now().UTC(),
		Details:    "quotes=1",
	}); err != nil {
		t.Fatalf("RecordProviderRun() error = %v", err)
	}
	if err := store.RecordProviderError(ctx, catalog.ProviderError{
		ErrorID:    "error-1",
		Provider:   "wikiquote",
		OccurredAt: time.Now().UTC(),
		Context:    "Frank Ocean",
		Message:    "rate limited",
	}); err != nil {
		t.Fatalf("RecordProviderError() error = %v", err)
	}
	job := models.JobRun{
		JobID:      "job-1",
		Name:       "catalog-refresh",
		Scope:      "bootstrap",
		Status:     "succeeded",
		StartedAt:  time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := store.RecordJob(ctx, job); err != nil {
		t.Fatalf("RecordJob() error = %v", err)
	}
	if err := store.RecordJobItem(ctx, models.JobItem{
		JobItemID:  "item-1",
		JobID:      "job-1",
		Provider:   "quotefancy",
		Target:     "bootstrap:data",
		Status:     "succeeded",
		StartedAt:  job.StartedAt,
		FinishedAt: job.FinishedAt,
		Details:    "bootstrapped",
	}); err != nil {
		t.Fatalf("RecordJobItem() error = %v", err)
	}

	tests := []struct {
		path string
		want string
	}{
		{path: "/v1/providers/wikiquote/runs", want: "run-1"},
		{path: "/v1/providers/wikiquote/errors", want: "error-1"},
		{path: "/v1/jobs", want: "job-1"},
		{path: "/v1/jobs/job-1", want: "job-1"},
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
				t.Fatalf("expected body to contain %q, got %s", tc.want, body)
			}
		})
	}
}

func TestErrorEnvelopeAndRequestID(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	tests := []struct {
		path       string
		requestID  string
		statusCode int
		errorCode  string
	}{
		{path: "/v1/search", requestID: "req-search", statusCode: http.StatusBadRequest, errorCode: "missing_query"},
		{path: "/v1/lyrics", requestID: "req-lyrics", statusCode: http.StatusBadRequest, errorCode: "missing_lyrics_params"},
		{path: "/v1/sources/missing", requestID: "req-source", statusCode: http.StatusNotFound, errorCode: "source_not_found"},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			request.Header.Set("X-Request-ID", tc.requestID)
			server.Router().ServeHTTP(recorder, request)
			if recorder.Code != tc.statusCode {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.statusCode)
			}
			if got := recorder.Header().Get("X-Request-ID"); got != tc.requestID {
				t.Fatalf("X-Request-ID = %q, want %q", got, tc.requestID)
			}
			var response models.APIResponse[any]
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Error == nil || response.Error.Code != tc.errorCode {
				t.Fatalf("unexpected error payload %+v", response.Error)
			}
		})
	}
}

func TestLyricsEndpointUsesCache(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	key := search.StableHash("coldplay", "yellow")
	if err := store.SetProviderCache(context.Background(), "lrclib", "lyrics", key, `{"provider":"lrclib","artist":"Coldplay","track":"Yellow","lyrics":"Look at the stars","source_url":"https://lrclib.net"}`, time.Hour); err != nil {
		t.Fatalf("SetProviderCache() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/lyrics?artist=Coldplay&track=Yellow&provider=lrclib", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"cached":true`) {
		t.Fatalf("expected cached response, got %s", recorder.Body.String())
	}
}

func TestDisabledSetlistsEndpoint(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/artists/tanabata:frank-ocean/setlists", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
}
