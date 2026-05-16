package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/testutil"
)

func newServiceStore(t *testing.T) (*catalog.Store, context.Context) {
	t.Helper()
	tempDir := t.TempDir()
	store, err := catalog.Open(filepath.Join(tempDir, "catalog.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	ctx := context.Background()
	if err := store.SeedFromLegacyJSON(ctx, testutil.WriteLegacyQuotes(t, tempDir)); err != nil {
		t.Fatalf("SeedFromLegacyJSON() error = %v", err)
	}
	return store, ctx
}

func TestServiceEnrichArtistRecordsPartialFailure(t *testing.T) {
	store, ctx := newServiceStore(t)
	defer store.Close()

	failingMusicBrainz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream failed", http.StatusBadGateway)
	}))
	defer failingMusicBrainz.Close()

	emptyWikidata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("action") {
		case "wbsearchentities":
			_ = json.NewEncoder(w).Encode(map[string]any{"search": []any{}})
		default:
			t.Fatalf("unexpected wikidata action %q", r.URL.Query().Get("action"))
		}
	}))
	defer emptyWikidata.Close()

	emptyWikiquote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`["frank ocean",[],[],[]]`))
	}))
	defer emptyWikiquote.Close()

	service := NewService(store, nil)
	service.musicBrainz.SetHTTPClient(NewHTTPClient(failingMusicBrainz.URL))
	service.wikidata.SetHTTPClients(NewHTTPClient(emptyWikidata.URL), NewHTTPClient(emptyWikidata.URL))
	service.wikiquote.SetHTTPClient(NewHTTPClient(emptyWikiquote.URL))

	result, err := service.EnrichArtist(ctx, "Frank Ocean")
	if err != nil {
		t.Fatalf("EnrichArtist() error = %v", err)
	}
	if result.Status != "partial" {
		t.Fatalf("status = %q, want partial", result.Status)
	}

	failures, err := store.ProviderErrors(ctx, "musicbrainz", 10)
	if err != nil {
		t.Fatalf("ProviderErrors() error = %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("expected one provider failure, got %+v", failures)
	}
}

func TestServiceSkipsProviderDuringCooldown(t *testing.T) {
	store, ctx := newServiceStore(t)
	defer store.Close()

	musicBrainzHits := 0
	musicBrainz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		musicBrainzHits++
		t.Fatalf("musicbrainz should not be called during cooldown")
	}))
	defer musicBrainz.Close()

	emptyWikidata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"search": []any{}})
	}))
	defer emptyWikidata.Close()

	emptyWikiquote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`["frank ocean",[],[],[]]`))
	}))
	defer emptyWikiquote.Close()

	if err := store.SetProviderCooldown(ctx, "musicbrainz", time.Now().UTC().Add(time.Hour), "previous timeout"); err != nil {
		t.Fatalf("SetProviderCooldown() error = %v", err)
	}
	service := NewService(store, nil)
	service.musicBrainz.SetHTTPClient(NewHTTPClient(musicBrainz.URL))
	service.wikidata.SetHTTPClients(NewHTTPClient(emptyWikidata.URL), NewHTTPClient(emptyWikidata.URL))
	service.wikiquote.SetHTTPClient(NewHTTPClient(emptyWikiquote.URL))

	result, err := service.EnrichArtist(ctx, "Frank Ocean")
	if err != nil {
		t.Fatalf("EnrichArtist() error = %v", err)
	}
	if musicBrainzHits != 0 {
		t.Fatalf("musicbrainz hits = %d, want 0", musicBrainzHits)
	}
	if !strings.Contains(result.Details, "musicbrainz:cooldown") {
		t.Fatalf("expected cooldown detail, got %+v", result)
	}
}

func TestServiceEnrichArtistUpsertsAttributedQuote(t *testing.T) {
	store, ctx := newServiceStore(t)
	defer store.Close()

	noMatchMusicBrainz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ws/2/artist":
			_ = json.NewEncoder(w).Encode(map[string]any{"artists": []any{}})
		case "/ws/2/release-group":
			_ = json.NewEncoder(w).Encode(map[string]any{"release-groups": []any{}})
		default:
			t.Fatalf("unexpected musicbrainz path %q", r.URL.Path)
		}
	}))
	defer noMatchMusicBrainz.Close()

	noMatchWikidata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"search": []any{}})
	}))
	defer noMatchWikidata.Close()

	wikiquoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")
		switch action {
		case "opensearch":
			_, _ = w.Write([]byte(`["frank ocean",["Frank Ocean"],[""],["https://en.wikiquote.org/wiki/Frank_Ocean"]]`))
		case "parse":
			if r.URL.Query().Get("prop") == "sections" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"parse": map[string]any{
						"sections": []map[string]string{{"index": "1", "line": "Quotes"}},
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"parse": map[string]any{
					"text": map[string]string{
						"*": `<div><h2>Quotes</h2><ul><li>Discipline keeps the idea from collapsing.</li></ul></div>`,
					},
				},
			})
		default:
			t.Fatalf("unexpected action %q", action)
		}
	}))
	defer wikiquoteServer.Close()

	service := NewService(store, nil)
	service.musicBrainz.SetHTTPClient(NewHTTPClient(noMatchMusicBrainz.URL))
	service.wikidata.SetHTTPClients(NewHTTPClient(noMatchWikidata.URL), NewHTTPClient(noMatchWikidata.URL))
	service.wikiquote.SetHTTPClient(NewHTTPClient(wikiquoteServer.URL))

	result, err := service.EnrichArtist(ctx, "Frank Ocean")
	if err != nil {
		t.Fatalf("EnrichArtist() error = %v", err)
	}
	if result.Status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", result.Status)
	}

	quotes, err := store.ListQuotes(ctx, models.QuoteFilters{
		Artist:           "Frank Ocean",
		ProvenanceStatus: "source_attributed",
		Limit:            10,
	})
	if err != nil {
		t.Fatalf("ListQuotes() error = %v", err)
	}
	found := false
	for _, quote := range quotes.Data {
		if quote.Text == "Discipline keeps the idea from collapsing." && quote.ProviderOrigin == "wikiquote" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected attributed quote, got %+v", quotes.Data)
	}
}
