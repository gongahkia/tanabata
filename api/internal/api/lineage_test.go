package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
)

// seededLineageServer builds a fresh API server seeded only with the curated lineage bundles.
// We intentionally skip the legacy quote bootstrap so the surfaces under test are exercised in isolation.
func seededLineageServer(t *testing.T) (*Server, *catalog.Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := catalog.Open(filepath.Join(dir, "catalog.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	ctx := context.Background()
	dataDir := filepath.Join("..", "..", "data")
	if _, err := store.SeedCuratedSamples(ctx, filepath.Join(dataDir, "curated_samples.json"), "test-job-samples"); err != nil {
		t.Fatalf("SeedCuratedSamples error = %v", err)
	}
	if _, err := store.SeedCuratedWorks(ctx, filepath.Join(dataDir, "curated_works.json"), "test-job-works"); err != nil {
		t.Fatalf("SeedCuratedWorks error = %v", err)
	}
	if _, err := store.SeedCuratedPerformances(ctx, filepath.Join(dataDir, "curated_performances.json"), "test-job-perfs"); err != nil {
		t.Fatalf("SeedCuratedPerformances error = %v", err)
	}
	if _, err := store.SeedCuratedMisquotes(ctx, filepath.Join(dataDir, "curated_misquotes.json"), "test-job-misquotes"); err != nil {
		t.Fatalf("SeedCuratedMisquotes error = %v", err)
	}
	if err := store.RefreshSearchIndices(ctx); err != nil {
		t.Fatalf("RefreshSearchIndices error = %v", err)
	}
	server, err := NewServer(store, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	return server, store
}

func mustJSON(t *testing.T, body []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("json decode error = %v body=%s", err, string(body))
	}
}

func get(t *testing.T, server *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s status = %d body=%s", path, recorder.Code, recorder.Body.String())
	}
	return recorder
}

func TestListWorksAndCoverLineage(t *testing.T) {
	server, store := seededLineageServer(t)
	defer store.Close()

	recorder := get(t, server, "/v1/works?q=Hallelujah&limit=5")
	var listed struct {
		Data       []models.Work     `json:"data"`
		Pagination models.Pagination `json:"pagination"`
	}
	mustJSON(t, recorder.Body.Bytes(), &listed)
	if len(listed.Data) == 0 {
		t.Fatalf("expected Hallelujah in /v1/works results")
	}
	workID := listed.Data[0].WorkID
	recorder = get(t, server, "/v1/works/"+workID+"/recordings")
	var coverPayload struct {
		Data []models.Recording `json:"data"`
	}
	mustJSON(t, recorder.Body.Bytes(), &coverPayload)
	if len(coverPayload.Data) < 2 {
		t.Fatalf("expected multiple covers of Hallelujah, got %d", len(coverPayload.Data))
	}
}

func TestSampleLineageFlowsBothDirections(t *testing.T) {
	server, store := seededLineageServer(t)
	defer store.Close()

	recorder := get(t, server, "/v1/recordings?q=Rapper&limit=5")
	var listed struct {
		Data []models.Recording `json:"data"`
	}
	mustJSON(t, recorder.Body.Bytes(), &listed)
	if len(listed.Data) == 0 {
		t.Fatalf("expected to find Rapper's Delight recording")
	}
	derivative := listed.Data[0]
	recorder = get(t, server, "/v1/recordings/"+derivative.RecordingID+"/samples")
	var outgoing struct {
		Data []models.SampleEdge `json:"data"`
	}
	mustJSON(t, recorder.Body.Bytes(), &outgoing)
	if len(outgoing.Data) == 0 {
		t.Fatalf("expected outgoing sample edges (ancestors) for Rapper's Delight")
	}
	ancestor := outgoing.Data[0].SourceRecording
	recorder = get(t, server, "/v1/recordings/"+ancestor.RecordingID+"/sampled_by")
	var incoming struct {
		Data []models.SampleEdge `json:"data"`
	}
	mustJSON(t, recorder.Body.Bytes(), &incoming)
	if len(incoming.Data) == 0 {
		t.Fatalf("expected the ancestor (%s — %s) to surface descendants", ancestor.ArtistName, ancestor.Title)
	}
}

func TestSampleCycleAuditEventVisible(t *testing.T) {
	dir := t.TempDir()
	store, err := catalog.Open(filepath.Join(dir, "catalog.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	bundlePath := filepath.Join(dir, "samples.json")
	fixture := `[
		{
			"source_artist_name": "Cycle Source",
			"source_track_title": "Source Loop",
			"derivative_artist_name": "Cycle Derivative",
			"derivative_track_title": "Derivative Loop",
			"kind": "direct_sample",
			"status": "source_attributed",
			"confidence_score": 0.9
		},
		{
			"source_artist_name": "Cycle Derivative",
			"source_track_title": "Derivative Loop",
			"derivative_artist_name": "Cycle Source",
			"derivative_track_title": "Source Loop",
			"kind": "direct_sample",
			"status": "source_attributed",
			"confidence_score": 0.9
		}
	]`
	if err := os.WriteFile(bundlePath, []byte(fixture), 0o600); err != nil {
		t.Fatalf("write sample fixture: %v", err)
	}
	imported, err := store.SeedCuratedSamples(context.Background(), bundlePath, "job-cycle")
	if err != nil {
		t.Fatalf("SeedCuratedSamples() error = %v", err)
	}
	if imported != 1 {
		t.Fatalf("imported = %d, want 1", imported)
	}
	server, err := NewServer(store, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	recorder := get(t, server, "/v1/jobs/job-cycle/audit?limit=10")
	var payload struct {
		Data []models.IngestionAuditEvent `json:"data"`
	}
	mustJSON(t, recorder.Body.Bytes(), &payload)
	for _, event := range payload.Data {
		if event.Action == "record_sample_cycle" && event.Status == "rejected" {
			return
		}
	}
	t.Fatalf("record_sample_cycle audit event missing: %+v", payload.Data)
}

func TestDisputesFeedIncludesMultipleKinds(t *testing.T) {
	server, store := seededLineageServer(t)
	defer store.Close()

	recorder := get(t, server, "/v1/disputes?limit=50")
	var payload struct {
		Data []models.Dispute `json:"data"`
	}
	mustJSON(t, recorder.Body.Bytes(), &payload)
	if len(payload.Data) == 0 {
		t.Fatalf("expected at least one dispute in the curated catalog")
	}
	kinds := map[string]bool{}
	for _, dispute := range payload.Data {
		kinds[dispute.Claim.Kind] = true
	}
	if !kinds["attribution"] {
		t.Fatalf("expected at least one disputed attribution (misquote) in disputes feed")
	}
}

func TestQuoteLineageEndpointShape(t *testing.T) {
	server, store := seededLineageServer(t)
	defer store.Close()

	recorder := get(t, server, "/v1/quotes?q=knowledge&limit=5")
	var listed struct {
		Data []models.Quote `json:"data"`
	}
	mustJSON(t, recorder.Body.Bytes(), &listed)
	if len(listed.Data) == 0 {
		t.Fatalf("expected the seeded Hendrix misquote to appear in /v1/quotes")
	}
	quoteID := listed.Data[0].QuoteID
	recorder = get(t, server, "/v1/quotes/"+quoteID+"/lineage")
	var lineage struct {
		Data models.QuoteLineage `json:"data"`
	}
	mustJSON(t, recorder.Body.Bytes(), &lineage)
	if lineage.Data.QuoteID != quoteID {
		t.Fatalf("lineage quote_id mismatch want=%s got=%s", quoteID, lineage.Data.QuoteID)
	}
	if len(lineage.Data.Refuting) == 0 {
		t.Fatalf("expected refuting evidence in lineage response")
	}
}

func TestArtistPerformanceStatsEndpoint(t *testing.T) {
	server, store := seededLineageServer(t)
	defer store.Close()

	recorder := get(t, server, "/v1/artists?q=Radiohead&limit=1")
	var listed struct {
		Data []models.Artist `json:"data"`
	}
	mustJSON(t, recorder.Body.Bytes(), &listed)
	if len(listed.Data) == 0 {
		t.Fatalf("expected Radiohead in /v1/artists results")
	}
	artistID := listed.Data[0].ArtistID
	recorder = get(t, server, "/v1/artists/"+artistID+"/performances/stats")
	var stats struct {
		Data models.PerformanceStats `json:"data"`
	}
	mustJSON(t, recorder.Body.Bytes(), &stats)
	if stats.Data.TotalPerformed == 0 {
		t.Fatalf("expected Radiohead performance stats to be non-zero")
	}
}
