package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/providers"
	"github.com/gongahkia/tanabata/api/internal/search"
	"github.com/gongahkia/tanabata/api/internal/testutil"
)

func seededServer(t *testing.T) (*Server, *catalog.Store) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "catalog.sqlite")
	legacyPath := testutil.WriteLegacyQuotes(t, tempDir)
	curatedPath := testutil.WriteCuratedQuotes(t, tempDir)
	store, err := catalog.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.SeedFromLegacyJSON(context.Background(), legacyPath); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if _, err := store.ImportCuratedQuotes(context.Background(), curatedPath); err != nil {
		t.Fatalf("import curated quotes: %v", err)
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
	if len(quotes) != 8 {
		t.Fatalf("expected 8 quotes in legacy compatibility endpoint, got %d", len(quotes))
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

func TestSearchCursorPagination(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search?q=frank&limit=1", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	var first struct {
		Data models.SearchResults `json:"data"`
		Meta models.CursorMeta    `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if first.Meta.NextCursor == "" || len(first.Data.Quotes) != 1 {
		t.Fatalf("expected first page cursor and one quote, got %+v", first)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/v1/search?q=frank&limit=1&cursor="+first.Meta.NextCursor, nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	var second struct {
		Data models.SearchResults `json:"data"`
		Meta models.CursorMeta    `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if len(second.Data.Quotes) != 1 || second.Data.Quotes[0].QuoteID == first.Data.Quotes[0].QuoteID {
		t.Fatalf("expected stable second page, first=%+v second=%+v", first.Data.Quotes, second.Data.Quotes)
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
	if _, err := store.CaptureIngestionSnapshot(ctx, "job-1", "after", time.Now().UTC()); err != nil {
		t.Fatalf("CaptureIngestionSnapshot() error = %v", err)
	}
	if err := store.RecordIngestionAuditEvent(ctx, models.IngestionAuditEvent{
		EventID:    "audit-1",
		JobID:      "job-1",
		JobItemID:  "item-1",
		Provider:   "quotefancy",
		Target:     "bootstrap:data",
		Action:     "seed",
		Status:     "succeeded",
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
		Details:    "seeded",
	}); err != nil {
		t.Fatalf("RecordIngestionAuditEvent() error = %v", err)
	}

	tests := []struct {
		path string
		want string
	}{
		{path: "/v1/providers/wikiquote/runs", want: "run-1"},
		{path: "/v1/providers/wikiquote/errors", want: "error-1"},
		{path: "/v1/jobs", want: "job-1"},
		{path: "/v1/jobs/job-1", want: "job-1"},
		{path: "/v1/jobs/job-1?include=audit,snapshots", want: "audit_events"},
		{path: "/v1/jobs/job-1?include=audit,snapshots", want: "snapshots"},
		{path: "/v1/jobs/job-1/snapshots", want: "after"},
		{path: "/v1/jobs/job-1/audit", want: "audit-1"},
		{path: "/v1/timeline", want: "catalog-refresh"},
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

func TestTimelineCursorPagination(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		timestamp := now.Add(time.Duration(-i) * time.Minute).Format(time.RFC3339)
		if err := store.RecordJob(ctx, models.JobRun{
			JobID:      "timeline-job-" + strconv.Itoa(i),
			Name:       "timeline-job",
			Status:     "succeeded",
			StartedAt:  timestamp,
			FinishedAt: timestamp,
		}); err != nil {
			t.Fatalf("RecordJob() error = %v", err)
		}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/timeline?limit=1", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	var first struct {
		Data []models.TimelineEvent `json:"data"`
		Meta models.CursorMeta      `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if len(first.Data) != 1 || first.Meta.NextCursor == "" {
		t.Fatalf("expected first timeline cursor, got %+v", first)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/v1/timeline?limit=1&cursor="+first.Meta.NextCursor, nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	var second struct {
		Data []models.TimelineEvent `json:"data"`
		Meta models.CursorMeta      `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if len(second.Data) != 1 || second.Data[0].EventID == first.Data[0].EventID {
		t.Fatalf("expected stable second timeline page, first=%+v second=%+v", first.Data, second.Data)
	}
}

func TestReviewQueueEndpoint(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/review/queue?limit=5", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data []models.ReviewQueueItem `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) == 0 {
		t.Fatalf("expected review queue items")
	}
	if response.Data[0].Reason == "" {
		t.Fatalf("expected queue reason")
	}
}

func TestStaleQuotesEndpoint(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	artistID, err := store.ResolveArtistID(context.Background(), "Frank Ocean")
	if err != nil {
		t.Fatalf("ResolveArtistID() error = %v", err)
	}
	if err := store.UpsertQuote(context.Background(), models.Quote{
		Text:             "A deliberately stale verification marker.",
		ArtistID:         artistID,
		ArtistName:       "Frank Ocean",
		ProvenanceStatus: "source_attributed",
		ConfidenceScore:  0.9,
		LastVerifiedAt:   time.Now().UTC().AddDate(0, 0, -220).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("UpsertQuote() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/review/stale?limit=10", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data []models.Quote `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var found bool
	for _, quote := range response.Data {
		if quote.Text == "A deliberately stale verification marker." && quote.FreshnessStatus == "stale" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected stale quote with freshness metadata, got %+v", response.Data)
	}
}

func TestIntegrityEndpoint(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/integrity", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	var response models.APIResponse[models.IntegrityReport]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Data.OK || response.Data.SQLite != "ok" {
		t.Fatalf("unexpected integrity response %+v", response.Data)
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

func TestLyricsEndpointRefreshesExpiredCache(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	key := search.StableHash("coldplay", "yellow")
	if err := store.SetProviderCache(context.Background(), "lrclib", "lyrics", key, `{"provider":"lrclib","artist":"Coldplay","track":"Yellow","lyrics":"stale"}`, -time.Minute); err != nil {
		t.Fatalf("SetProviderCache() error = %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"trackName":   "Yellow",
			"artistName":  "Coldplay",
			"plainLyrics": "Fresh lyrics",
		})
	}))
	defer upstream.Close()
	server.lrclib.SetHTTPClient(providers.NewHTTPClient(upstream.URL))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/lyrics?artist=Coldplay&track=Yellow&provider=lrclib", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"cached":false`) || !strings.Contains(body, "Fresh lyrics") {
		t.Fatalf("expected refreshed lyrics, got %s", body)
	}
}

func TestLyricsEndpointFallsBackFromMalformedCacheAndProviderFailure(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	key := search.StableHash("coldplay", "yellow")
	if err := store.SetProviderCache(context.Background(), "lrclib", "lyrics", key, `{"lyrics":`, time.Hour); err != nil {
		t.Fatalf("SetProviderCache() error = %v", err)
	}

	lrclibUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "broken", http.StatusBadGateway)
	}))
	defer lrclibUpstream.Close()
	server.lrclib.SetHTTPClient(providers.NewHTTPClient(lrclibUpstream.URL))

	lyricsOVHUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"lyrics": "Fallback lyrics"})
	}))
	defer lyricsOVHUpstream.Close()
	server.lyricsOVH.SetHTTPClient(providers.NewHTTPClient(lyricsOVHUpstream.URL))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/lyrics?artist=Coldplay&track=Yellow", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"provider":"lyricsovh"`) || !strings.Contains(body, "Fallback lyrics") {
		t.Fatalf("expected fallback provider response, got %s", body)
	}
}

func TestLyricsEndpointRequestedProviderFailureReturnsError(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "broken", http.StatusBadGateway)
	}))
	defer upstream.Close()
	server.lrclib.SetHTTPClient(providers.NewHTTPClient(upstream.URL))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/lyrics?artist=Coldplay&track=Yellow&provider=lrclib", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", recorder.Code)
	}
	var response models.APIResponse[any]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "provider_request_failed" {
		t.Fatalf("unexpected error payload %+v", response.Error)
	}
}

func TestSetlistsEndpointUsesProviderAfterExpiredCache(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	artistID, err := store.ResolveArtistID(context.Background(), "Frank Ocean")
	if err != nil || artistID == "" {
		t.Fatalf("ResolveArtistID() err=%v artistID=%q", err, artistID)
	}
	artist, err := store.ArtistByID(context.Background(), artistID)
	if err != nil || artist == nil {
		t.Fatalf("ArtistByID() err=%v artist=%+v", err, artist)
	}
	artist.MBID = "mbid-frank"
	if err := store.UpsertArtist(context.Background(), *artist); err != nil {
		t.Fatalf("UpsertArtist() error = %v", err)
	}

	if err := store.SetProviderCache(context.Background(), "setlistfm", "setlists", "mbid-frank", `{"stale":true}`, -time.Minute); err != nil {
		t.Fatalf("SetProviderCache() error = %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"setlist": []map[string]any{{
				"id":        "set-1",
				"eventDate": "15-05-2026",
				"url":       "https://setlist.fm/set-1",
				"artist":    map[string]any{"name": "Frank Ocean", "mbid": "mbid-frank"},
				"venue":     map[string]any{"name": "Madison Square Garden", "city": map[string]any{"name": "New York", "country": map[string]any{"name": "United States"}}},
			}},
		})
	}))
	defer upstream.Close()
	server.setlistFM.SetAPIKey("test-key")
	server.setlistFM.SetHTTPClient(providers.NewHTTPClient(upstream.URL))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/artists/"+artistID+"/setlists", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"cached":false`) || !strings.Contains(body, "Madison Square Garden") {
		t.Fatalf("expected fetched setlists, got %s", body)
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
