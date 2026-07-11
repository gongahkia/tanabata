package api

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

var embedQuoteTemplate = template.Must(template.New("quote_embed").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
html,body{margin:0;padding:0;background:transparent;font-family:Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
.tanabata-embed-card{box-sizing:border-box;width:400px;min-height:160px;max-width:100vw;padding:18px 20px;border:1px solid #d8dde6;border-radius:8px;background:#fbfcfe;color:#111827;display:flex;flex-direction:column;gap:12px;box-shadow:0 10px 28px rgba(17,24,39,.10)}
.tanabata-embed-card.theme-dark{border-color:#303846;background:#111827;color:#f8fafc;box-shadow:0 10px 28px rgba(0,0,0,.30)}
.tanabata-quote{margin:0;font-size:18px;line-height:1.35;font-weight:650;letter-spacing:0;display:-webkit-box;-webkit-line-clamp:3;-webkit-box-orient:vertical;overflow:hidden}
.tanabata-meta{display:flex;align-items:center;justify-content:space-between;gap:10px;font-size:13px;line-height:1.2}
.tanabata-artist{font-weight:700;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.tanabata-chips{display:flex;gap:6px;flex:0 0 auto}
.tanabata-chip{border:1px solid currentColor;border-radius:999px;padding:4px 8px;font-size:11px;font-weight:750;text-transform:uppercase;letter-spacing:.04em;opacity:.82}
.tanabata-link{color:inherit;text-decoration:none;border-top:1px solid rgba(100,116,139,.28);padding-top:10px;font-size:12px;font-weight:650;opacity:.78}
.tanabata-link:focus,.tanabata-link:hover{opacity:1;text-decoration:underline}
.tanabata-embed-error{box-sizing:border-box;width:400px;min-height:120px;max-width:100vw;padding:18px 20px;border:1px solid #fecaca;border-radius:8px;background:#fff1f2;color:#991b1b;font:600 14px/1.35 ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
</style>
<title>Tanabata quote embed</title>
</head>
<body>
{{if .Error}}<div class="tanabata-embed-error">{{.Error}}</div>{{else}}
<article class="tanabata-embed-card theme-{{.Theme}}" aria-label="Tanabata quote">
<p class="tanabata-quote">{{.Text}}</p>
<div class="tanabata-meta">
<span class="tanabata-artist">{{.ArtistName}}</span>
<span class="tanabata-chips">
<span class="tanabata-chip">{{.ProvenanceStatus}}</span>
<span class="tanabata-chip">{{.Confidence}}</span>
</span>
</div>
<a class="tanabata-link" href="{{.Link}}" target="_blank" rel="noopener noreferrer">View on Tanabata</a>
</article>
{{end}}
</body>
</html>`))

type embedQuoteView struct {
	Text             string
	ArtistName       string
	ProvenanceStatus string
	Confidence       string
	Theme            string
	Link             string
	Error            string
}

func (s *Server) embedQuote(c *gin.Context) {
	quoteID := c.Param("quote_id")
	quote, err := s.store.QuoteByID(c.Request.Context(), quoteID)
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "quote_embed_failed", "failed to render quote embed", nil, err)
		return
	}
	if quote == nil {
		s.writeEmbedHTML(c, http.StatusNotFound, embedQuoteView{Error: "Quote not found."})
		return
	}
	s.writeEmbedHTML(c, http.StatusOK, embedQuoteView{
		Text:             quote.Text,
		ArtistName:       quote.ArtistName,
		ProvenanceStatus: quote.ProvenanceStatus,
		Confidence:       fmt.Sprintf("%.0f%%", quote.ConfidenceScore*100),
		Theme:            embedTheme(c.Query("theme")),
		Link:             "https://tanabata.dev/quotes/" + url.PathEscape(quote.QuoteID),
	})
}

func (s *Server) writeEmbedHTML(c *gin.Context, status int, view embedQuoteView) {
	if view.Theme == "" {
		view.Theme = "light"
	}
	var body bytes.Buffer
	if err := embedQuoteTemplate.Execute(&body, view); err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "quote_embed_failed", "failed to render quote embed", nil, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=3600, immutable")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors *; base-uri 'none'; form-action 'none'")
	c.Data(status, "text/html; charset=utf-8", body.Bytes())
}

func embedTheme(theme string) string {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "dark":
		return "dark"
	default:
		return "light"
	}
}
