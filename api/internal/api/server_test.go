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
	return NewServer(store), store
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

func TestV1SearchEndpoint(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search?q=frank", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var response models.SearchResponse
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
	var response models.Quote
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ArtistName != "Frank Ocean" {
		t.Fatalf("artist = %q, want Frank Ocean", response.ArtistName)
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
