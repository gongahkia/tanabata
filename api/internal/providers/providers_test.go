package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gongahkia/tanabata/api/internal/models"
)

func TestMusicBrainzProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Fatalf("missing User-Agent header")
		}
		switch r.URL.Path {
		case "/ws/2/artist":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"artists": []map[string]any{{
					"id":      "mbid-frank",
					"name":    "Frank Ocean",
					"country": "US",
					"aliases": []map[string]string{{"name": "Christopher Francis Ocean"}},
					"life-span": map[string]string{
						"begin": "1987-10-28",
					},
				}},
			})
		case "/ws/2/release-group":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"release-groups": []map[string]any{{
					"id":                 "blonde",
					"title":              "Blonde",
					"primary-type":       "Album",
					"first-release-date": "2016-08-20",
				}},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := NewMusicBrainzProvider()
	provider.client = NewHTTPClient(server.URL)

	artist, err := provider.SearchArtist(context.Background(), "Frank Ocean")
	if err != nil {
		t.Fatalf("SearchArtist() error = %v", err)
	}
	if artist == nil || artist.ArtistID != "mbid-frank" {
		t.Fatalf("expected mapped artist, got %+v", artist)
	}

	releases, err := provider.Releases(context.Background(), "mbid-frank")
	if err != nil {
		t.Fatalf("Releases() error = %v", err)
	}
	if len(releases) != 1 || releases[0].Title != "Blonde" {
		t.Fatalf("unexpected releases %+v", releases)
	}
}

func TestWikidataProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Api-User-Agent"); got == "" && strings.Contains(r.URL.RawQuery, "wbgetentities") {
			t.Fatalf("missing Api-User-Agent header")
		}
		query := r.URL.Query().Get("action")
		switch query {
		case "wbsearchentities":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"search": []map[string]string{{
					"id":          "Q123",
					"label":       "Frank Ocean",
					"description": "American singer",
				}},
			})
		case "wbgetentities":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"entities": map[string]any{
					"Q123": map[string]any{
						"id": "Q123",
						"descriptions": map[string]any{
							"en": map[string]string{"value": "American singer"},
						},
						"sitelinks": map[string]any{
							"enwikiquote": map[string]string{"title": "Frank Ocean"},
							"enwiki":      map[string]string{"title": "Frank Ocean"},
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected action %s", query)
		}
	}))
	defer server.Close()

	provider := NewWikidataProvider()
	provider.searchClient = NewHTTPClient(server.URL)
	provider.entityClient = NewHTTPClient(server.URL)

	artist, err := provider.SearchArtist(context.Background(), "Frank Ocean")
	if err != nil {
		t.Fatalf("SearchArtist() error = %v", err)
	}
	if artist == nil || artist.EntityID != "Q123" || artist.WikiquoteTitle != "Frank Ocean" {
		t.Fatalf("unexpected artist %+v", artist)
	}
}

func TestWikiquoteProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")
		switch action {
		case "opensearch":
			_, _ = w.Write([]byte(`["frank ocean",["Frank Ocean"],[""],["https://en.wikiquote.org/wiki/Frank_Ocean"]]`))
		case "parse":
			prop := r.URL.Query().Get("prop")
			if prop == "sections" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"parse": map[string]any{
						"sections": []map[string]string{
							{"index": "1", "line": "Quotes"},
							{"index": "2", "line": "References"},
						},
					},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"parse": map[string]any{
					"text": map[string]string{
						"*": `<div><h2>Quotes</h2><ul><li>Work hard in silence.<ul><li>Nested citation</li></ul></li></ul></div>`,
					},
				},
			})
		default:
			t.Fatalf("unexpected action %s", action)
		}
	}))
	defer server.Close()

	provider := NewWikiquoteProvider()
	provider.client = NewHTTPClient(server.URL)

	page, err := provider.SearchPage(context.Background(), "Frank Ocean")
	if err != nil {
		t.Fatalf("SearchPage() error = %v", err)
	}
	if page != "Frank Ocean" {
		t.Fatalf("SearchPage() = %q, want Frank Ocean", page)
	}
	quotes, err := provider.Quotes(context.Background(), page)
	if err != nil {
		t.Fatalf("Quotes() error = %v", err)
	}
	if len(quotes) != 1 || quotes[0].Text != "Work hard in silence." {
		t.Fatalf("unexpected quotes %+v", quotes)
	}
}

func TestLastFMProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "test-key" {
			t.Fatalf("expected api key")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"artist": map[string]any{
				"name": "Frank Ocean",
				"mbid": "mbid-frank",
				"url":  "https://www.last.fm/music/Frank+Ocean",
				"tags": map[string]any{
					"tag": []map[string]string{{"name": "rnb"}, {"name": "soul"}},
				},
				"similar": map[string]any{
					"artist": []map[string]string{{"name": "SZA", "url": "https://www.last.fm/music/SZA"}},
				},
				"bio": map[string]string{
					"summary": "American singer-songwriter",
				},
			},
		})
	}))
	defer server.Close()

	provider := NewLastFMProvider()
	provider.client = NewHTTPClient(server.URL)
	provider.apiKey = "test-key"

	data, err := provider.ArtistInfo(context.Background(), models.Artist{Name: "Frank Ocean"})
	if err != nil {
		t.Fatalf("ArtistInfo() error = %v", err)
	}
	if data == nil || len(data.Tags) != 2 || len(data.Related) != 1 {
		t.Fatalf("unexpected last.fm data %+v", data)
	}
}

func TestLyricsProviders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/get"):
			_ = json.NewEncoder(w).Encode(map[string]string{
				"trackName":   "Yellow",
				"artistName":  "Coldplay",
				"plainLyrics": "Look at the stars",
			})
		case strings.HasPrefix(r.URL.Path, "/v1/Coldplay/Yellow"):
			_ = json.NewEncoder(w).Encode(map[string]string{
				"lyrics": "Look at the stars",
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	lrclib := NewLRCLIBProvider()
	lrclib.client = NewHTTPClient(server.URL)
	result, err := lrclib.Lyrics(context.Background(), "Coldplay", "Yellow")
	if err != nil || result == nil || result.Lyrics == "" {
		t.Fatalf("LRCLIBProvider.Lyrics() err=%v result=%+v", err, result)
	}

	lyricsOVH := NewLyricsOVHProvider()
	lyricsOVH.client = NewHTTPClient(server.URL)
	result, err = lyricsOVH.Lyrics(context.Background(), "Coldplay", "Yellow")
	if err != nil || !strings.Contains(result.Lyrics, "Look at the stars") {
		t.Fatalf("LyricsOVHProvider.Lyrics() err=%v result=%+v", err, result)
	}
}

func TestSetlistProviderDisabledByDefault(t *testing.T) {
	provider := NewSetlistFMProvider()
	if provider.Enabled() {
		t.Fatalf("SetlistFM provider should be disabled without key")
	}
}

func TestHTTPClientRetriesTransientFailure(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			http.Error(w, "try again", http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	var payload map[string]string
	if err := client.JSON(context.Background(), "/", url.Values{}, nil, &payload); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected retry, got %d attempts", attempts.Load())
	}
	if payload["status"] != "ok" {
		t.Fatalf("unexpected payload %+v", payload)
	}
}

func TestHTTPClientReturnsErrorAfterRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	client.backoff = 5 * time.Millisecond
	var payload map[string]string
	err := client.JSON(context.Background(), "/", nil, nil, &payload)
	if err == nil || !strings.Contains(err.Error(), "provider request failed") {
		t.Fatalf("expected upstream error, got %v", err)
	}
}

func TestHTTPClientContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "late"})
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	client.attempts = 1
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	var payload map[string]string
	err := client.JSON(ctx, "/", nil, nil, &payload)
	if err == nil || (!errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline exceeded")) {
		t.Fatalf("expected deadline error, got %v", err)
	}
}
