package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gongahkia/tanabata/api/internal/models"
)

func TestEmbedQuoteRendersHTMLCard(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	quotes, err := store.ListQuotes(context.Background(), models.QuoteFilters{Limit: 1})
	if err != nil || len(quotes.Data) == 0 {
		t.Fatalf("ListQuotes() err=%v count=%d", err, len(quotes.Data))
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/embed/quote/"+quotes.Data[0].QuoteID+"?theme=dark", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html; charset=utf-8") {
		t.Fatalf("Content-Type = %q, want text/html", contentType)
	}
	if recorder.Header().Get("X-Frame-Options") != "" {
		t.Fatalf("X-Frame-Options should be omitted")
	}
	if csp := recorder.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors *") {
		t.Fatalf("CSP = %q, want frame-ancestors *", csp)
	}
	if cache := recorder.Header().Get("Cache-Control"); cache != "public, max-age=3600, immutable" {
		t.Fatalf("Cache-Control = %q", cache)
	}
	body := recorder.Body.String()
	for _, want := range []string{"tanabata-embed-card", "theme-dark", quotes.Data[0].Text, quotes.Data[0].ArtistName, "View on Tanabata"} {
		if !strings.Contains(body, want) {
			t.Fatalf("embed body missing %q: %s", want, body)
		}
	}
}

func TestEmbedQuoteEscapesHTML(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	artistID, err := store.ResolveArtistID(context.Background(), "Frank Ocean")
	if err != nil {
		t.Fatalf("ResolveArtistID() error = %v", err)
	}
	if err := store.UpsertQuote(context.Background(), models.Quote{
		QuoteID:          "quote-xss",
		Text:             `<script>alert("x")</script>`,
		ArtistID:         artistID,
		ArtistName:       "Frank Ocean",
		ProvenanceStatus: "needs_review",
		ConfidenceScore:  0.2,
		ProviderOrigin:   "tanabata_test",
	}); err != nil {
		t.Fatalf("UpsertQuote() error = %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/embed/quote/quote-xss", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := recorder.Body.String()
	if strings.Contains(body, `<script>alert("x")</script>`) {
		t.Fatalf("body contains unescaped script: %s", body)
	}
	if !strings.Contains(body, `&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;`) {
		t.Fatalf("body missing escaped script: %s", body)
	}
}

func TestEmbedQuoteMissingIDReturnsHTML404(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/embed/quote/missing", nil)
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "tanabata-embed-error") {
		t.Fatalf("expected embed error body, got %s", recorder.Body.String())
	}
}
