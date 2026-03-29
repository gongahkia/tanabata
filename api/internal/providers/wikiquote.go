package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

type WikiquoteProvider struct {
	client *HTTPClient
}

func NewWikiquoteProvider() *WikiquoteProvider {
	return &WikiquoteProvider{client: NewHTTPClient("https://en.wikiquote.org")}
}

func (p *WikiquoteProvider) Name() string {
	return "wikiquote"
}

type mediaWikiSectionsResponse struct {
	Parse struct {
		Sections []struct {
			Index string `json:"index"`
			Line  string `json:"line"`
		} `json:"sections"`
	} `json:"parse"`
}

type mediaWikiHTMLResponse struct {
	Parse struct {
		Text map[string]string `json:"text"`
	} `json:"parse"`
}

type WikiquoteQuote struct {
	Text       string
	Section    string
	Source     models.Source
	SourceType string
}

func (p *WikiquoteProvider) SearchPage(ctx context.Context, query string) (string, error) {
	params := url.Values{}
	params.Set("action", "opensearch")
	params.Set("format", "json")
	params.Set("search", query)
	params.Set("limit", "5")
	body, err := p.client.Text(ctx, "/w/api.php", params, map[string]string{"Api-User-Agent": defaultUserAgent})
	if err != nil {
		return "", err
	}
	var raw []any
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return "", err
	}
	if len(raw) < 2 {
		return "", nil
	}
	results, ok := raw[1].([]any)
	if !ok || len(results) == 0 {
		return "", nil
	}
	best := strings.TrimSpace(fmt.Sprint(results[0]))
	bestScore := 0
	for _, candidate := range results {
		title := strings.TrimSpace(fmt.Sprint(candidate))
		score := search.SimilarityScore(query, title)
		if score > bestScore {
			best = title
			bestScore = score
		}
	}
	return best, nil
}

func (p *WikiquoteProvider) Quotes(ctx context.Context, pageTitle string) ([]WikiquoteQuote, error) {
	if strings.TrimSpace(pageTitle) == "" {
		return nil, nil
	}
	params := url.Values{}
	params.Set("action", "parse")
	params.Set("page", pageTitle)
	params.Set("prop", "sections")
	params.Set("format", "json")
	var sections mediaWikiSectionsResponse
	if err := p.client.JSON(ctx, "/w/api.php", params, map[string]string{"Api-User-Agent": defaultUserAgent}, &sections); err != nil {
		return nil, err
	}

	var quotes []WikiquoteQuote
	seen := map[string]struct{}{}
	retrievedAt := time.Now().UTC().Format(time.RFC3339)
	for _, section := range sections.Parse.Sections {
		if !isQuoteSection(section.Line) {
			continue
		}
		sectionQuotes, err := p.sectionQuotes(ctx, pageTitle, section.Index)
		if err != nil {
			return nil, err
		}
		for _, quote := range sectionQuotes {
			quote.Section = section.Line
			quote.Source = models.Source{
				SourceID:    search.SourceID("wikiquote", quote.Source.URL),
				Provider:    "wikiquote",
				URL:         quote.Source.URL,
				Title:       pageTitle + " - " + section.Line,
				Publisher:   "Wikiquote",
				License:     "CC-BY-SA-4.0",
				RetrievedAt: retrievedAt,
			}
			if _, ok := seen[quote.Text]; ok {
				continue
			}
			seen[quote.Text] = struct{}{}
			quotes = append(quotes, quote)
		}
	}
	sort.SliceStable(quotes, func(i, j int) bool {
		if quotes[i].Section == quotes[j].Section {
			return quotes[i].Text < quotes[j].Text
		}
		return quotes[i].Section < quotes[j].Section
	})
	return quotes, nil
}

func (p *WikiquoteProvider) sectionQuotes(ctx context.Context, pageTitle, sectionIndex string) ([]WikiquoteQuote, error) {
	params := url.Values{}
	params.Set("action", "parse")
	params.Set("page", pageTitle)
	params.Set("prop", "text")
	params.Set("section", sectionIndex)
	params.Set("format", "json")
	var response mediaWikiHTMLResponse
	if err := p.client.JSON(ctx, "/w/api.php", params, map[string]string{"Api-User-Agent": defaultUserAgent}, &response); err != nil {
		return nil, err
	}
	html := response.Parse.Text["*"]
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	var quotes []WikiquoteQuote
	doc.Find("ul > li").Each(func(_ int, selection *goquery.Selection) {
		if selection.ParentsFiltered("li").Length() > 0 {
			return
		}
		item := selection.Clone()
		item.Find("ul, ol, dl, sup").Remove()
		text := strings.TrimSpace(item.Text())
		text = strings.Join(strings.Fields(text), " ")
		if text == "" || len(text) < 12 || looksLikeNavigation(text) {
			return
		}
		anchor := strings.ReplaceAll(selection.Parent().PrevAllFiltered("h2,h3").First().Text(), " ", "_")
		if anchor == "" {
			anchor = "Quotes"
		}
		quotes = append(quotes, WikiquoteQuote{
			Text:       text,
			SourceType: "wikiquote",
			Source: models.Source{
				URL: "https://en.wikiquote.org/wiki/" + url.PathEscape(strings.ReplaceAll(pageTitle, " ", "_")) + "#" + url.PathEscape(anchor),
			},
		})
	})
	return quotes, nil
}

func isQuoteSection(section string) bool {
	normalized := search.NormalizeText(section)
	if normalized == "" {
		return false
	}
	skip := []string{
		"about", "external links", "references", "see also", "quotes about", "works about", "gallery", "notes",
	}
	for _, blocked := range skip {
		if normalized == blocked || strings.Contains(normalized, blocked) {
			return false
		}
	}
	return true
}

func looksLikeNavigation(text string) bool {
	normalized := search.NormalizeText(text)
	if normalized == "" {
		return true
	}
	prefixes := []string{
		"this page", "wikipedia", "see also", "external links", "references", "source", "citation needed",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}
