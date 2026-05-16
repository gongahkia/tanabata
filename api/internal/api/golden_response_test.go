package api

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite API golden response fixtures")

func TestAPIGoldenResponses(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()
	seedGoldenOperationalState(t, store)

	tests := []struct {
		name string
		path string
	}{
		{name: "search", path: "/v1/search?q=frank"},
		{name: "provenance", path: goldenProvenancePath(t, store)},
		{name: "providers", path: "/v1/providers"},
		{name: "timeline", path: "/v1/timeline?limit=6"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			server.Router().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s status = %d, want 200 body=%s", tc.path, recorder.Code, recorder.Body.String())
			}
			actual := canonicalGoldenJSON(t, recorder.Body.Bytes())
			path := filepath.Join("testdata", "golden", tc.name+".json")
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("MkdirAll() error = %v", err)
				}
				if err := os.WriteFile(path, actual, 0o644); err != nil {
					t.Fatalf("WriteFile(%s) error = %v", path, err)
				}
				return
			}
			expected, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%s) error = %v", path, err)
			}
			if !bytes.Equal(bytes.TrimSpace(expected), bytes.TrimSpace(actual)) {
				t.Fatalf("golden mismatch for %s\nexpected:\n%s\nactual:\n%s", tc.name, expected, actual)
			}
		})
	}
}

func seedGoldenOperationalState(t *testing.T, store *catalog.Store) {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)
	if err := store.RecordProviderRun(ctx, catalog.ProviderRun{
		RunID:      "golden-provider-run",
		Provider:   "wikiquote",
		Status:     "success",
		StartedAt:  now.Add(-time.Hour),
		FinishedAt: now,
		Details:    "quotes=2",
	}); err != nil {
		t.Fatalf("RecordProviderRun() error = %v", err)
	}
	job := models.JobRun{
		JobID:      "golden-job",
		Name:       "golden-catalog-refresh",
		Scope:      "bootstrap",
		Status:     "succeeded",
		StartedAt:  now.Add(-time.Hour).Format(time.RFC3339),
		FinishedAt: now.Format(time.RFC3339),
		Details:    "golden fixture",
	}
	if err := store.RecordJob(ctx, job); err != nil {
		t.Fatalf("RecordJob() error = %v", err)
	}
	if err := store.RecordJobItem(ctx, models.JobItem{
		JobItemID:  "golden-job-item",
		JobID:      job.JobID,
		Provider:   "tanabata_curated",
		Target:     "bootstrap:data/curated_quotes.json",
		Status:     "succeeded",
		StartedAt:  job.StartedAt,
		FinishedAt: job.FinishedAt,
		Details:    "imported curated quotes",
	}); err != nil {
		t.Fatalf("RecordJobItem() error = %v", err)
	}
	if _, err := store.CaptureIngestionSnapshot(ctx, job.JobID, "after", now); err != nil {
		t.Fatalf("CaptureIngestionSnapshot() error = %v", err)
	}
}

func goldenProvenancePath(t *testing.T, store *catalog.Store) string {
	t.Helper()
	quotes, err := store.ListQuotes(context.Background(), models.QuoteFilters{
		Artist:           "Frank Ocean",
		ProvenanceStatus: "verified",
		Limit:            1,
	})
	if err != nil {
		t.Fatalf("ListQuotes() error = %v", err)
	}
	if len(quotes.Data) == 0 {
		t.Fatalf("expected verified quote for golden provenance fixture")
	}
	return "/v1/quotes/" + quotes.Data[0].QuoteID + "/provenance"
}

func canonicalGoldenJSON(t *testing.T, content []byte) []byte {
	t.Helper()
	var payload any
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v body=%s", err, content)
	}
	normalized := normalizeGoldenValue(payload, "")
	content, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	return append(content, '\n')
}

func normalizeGoldenValue(value any, key string) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for nestedKey := range typed {
			keys = append(keys, nestedKey)
		}
		sort.Strings(keys)
		normalized := map[string]any{}
		for _, nestedKey := range keys {
			normalized[nestedKey] = normalizeGoldenValue(typed[nestedKey], nestedKey)
		}
		return normalized
	case []any:
		for i := range typed {
			typed[i] = normalizeGoldenValue(typed[i], key)
		}
		return typed
	case string:
		if shouldRedactGoldenTimestamp(key, typed) {
			return "<timestamp>"
		}
		return typed
	case float64:
		if key == "freshness_age_days" {
			return float64(0)
		}
		return typed
	default:
		return typed
	}
}

func shouldRedactGoldenTimestamp(key, value string) bool {
	if value == "" {
		return false
	}
	if key == "snapshot_version" || key == "at" || strings.HasSuffix(key, "_at") || strings.HasSuffix(key, "_until") || strings.HasSuffix(key, "_successful") {
		return true
	}
	return false
}
