package api

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestDisputesAtomFeed(t *testing.T) {
	server, store := seededLineageServer(t)
	defer store.Close()

	recorder := get(t, server, "/v1/disputes.atom?limit=20")
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/atom+xml") {
		t.Fatalf("Content-Type = %q, want application/atom+xml", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=300" {
		t.Fatalf("Cache-Control = %q, want public, max-age=300", got)
	}
	lastModified, err := time.Parse(http.TimeFormat, recorder.Header().Get("Last-Modified"))
	if err != nil {
		t.Fatalf("Last-Modified parse error = %v", err)
	}
	var feed atomFeed
	if err := xml.Unmarshal(recorder.Body.Bytes(), &feed); err != nil {
		t.Fatalf("xml decode error = %v body=%s", err, recorder.Body.String())
	}
	if feed.ID == "" || feed.Title == "" || feed.Updated == "" || feed.Author.Name == "" {
		t.Fatalf("feed missing required Atom fields: %+v", feed)
	}
	if len(feed.Entries) == 0 {
		t.Fatalf("expected dispute entries in Atom feed")
	}
	feedUpdated, err := time.Parse(time.RFC3339, feed.Updated)
	if err != nil {
		t.Fatalf("feed updated parse error = %v", err)
	}
	maxEntryUpdated := time.Unix(0, 0).UTC()
	for _, entry := range feed.Entries {
		if entry.ID == "" || entry.Title == "" || entry.Updated == "" {
			t.Fatalf("entry missing required Atom fields: %+v", entry)
		}
		if !strings.HasPrefix(entry.ID, "tag:tanabata.dev,") || !strings.Contains(entry.ID, ":claim/") {
			t.Fatalf("entry id = %q, want stable tag claim id", entry.ID)
		}
		if entry.Content.Type != "application/json" {
			t.Fatalf("entry content type = %q, want application/json", entry.Content.Type)
		}
		var claim models.Claim
		if err := json.Unmarshal([]byte(entry.Content.Body), &claim); err != nil {
			t.Fatalf("entry content JSON decode error = %v body=%s", err, entry.Content.Body)
		}
		if claim.ClaimID == "" || !strings.HasSuffix(entry.ID, "/"+claim.ClaimID) {
			t.Fatalf("entry id %q does not contain claim id %q", entry.ID, claim.ClaimID)
		}
		updated, err := time.Parse(time.RFC3339, entry.Updated)
		if err != nil {
			t.Fatalf("entry updated parse error = %v", err)
		}
		if updated.After(maxEntryUpdated) {
			maxEntryUpdated = updated
		}
	}
	if !feedUpdated.Equal(maxEntryUpdated) {
		t.Fatalf("feed updated = %s, want max entry updated %s", feedUpdated, maxEntryUpdated)
	}
	if !lastModified.Equal(feedUpdated.Truncate(time.Second)) {
		t.Fatalf("Last-Modified = %s, want feed updated %s", lastModified, feedUpdated)
	}

	recorder2 := get(t, server, "/v1/disputes.atom?limit=20")
	if recorder.Body.String() != recorder2.Body.String() {
		t.Fatalf("feed is not deterministic between reads")
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

func TestEntityGraphEndpoint(t *testing.T) {
	server, store := seededLineageServer(t)
	defer store.Close()

	recorder := get(t, server, "/v1/graph/tanabata:frank-ocean?depth=2")
	var payload struct {
		Data models.EntityGraph `json:"data"`
		Meta models.CursorMeta  `json:"meta"`
	}
	mustJSON(t, recorder.Body.Bytes(), &payload)
	if len(payload.Data.Nodes) < 2 || len(payload.Data.Edges) == 0 {
		t.Fatalf("expected graph nodes and edges, got %+v", payload.Data)
	}
	hasArtist := false
	hasAttribution := false
	for _, node := range payload.Data.Nodes {
		if node.ID == "tanabata:frank-ocean" && node.Kind == "artist" && node.Label == "Frank Ocean" {
			hasArtist = true
		}
	}
	for _, edge := range payload.Data.Edges {
		if edge.To == "tanabata:frank-ocean" && edge.Kind == "attribution" && edge.ClaimID != "" {
			hasAttribution = true
		}
	}
	if !hasArtist || !hasAttribution {
		t.Fatalf("graph missing Frank Ocean attribution: %+v", payload.Data)
	}

	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/graph/tanabata:frank-ocean?depth=4", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("depth status = %d, want 400 body=%s", recorder.Code, recorder.Body.String())
	}
	var problem models.ProblemDetails
	mustJSON(t, recorder.Body.Bytes(), &problem)
	if problem.Code != "depth_too_large" {
		t.Fatalf("problem code = %q, want depth_too_large", problem.Code)
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
