package providers

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

type WikidataProvider struct {
	searchClient *HTTPClient
	entityClient *HTTPClient
}

func NewWikidataProvider() *WikidataProvider {
	return &WikidataProvider{
		searchClient: NewHTTPClient("https://www.wikidata.org"),
		entityClient: NewHTTPClient("https://www.wikidata.org"),
	}
}

func (p *WikidataProvider) Name() string {
	return "wikidata"
}

type wikidataSearchResponse struct {
	Search []struct {
		ID          string `json:"id"`
		Label       string `json:"label"`
		Description string `json:"description"`
	} `json:"search"`
}

type wikidataEntityResponse struct {
	Entities map[string]struct {
		ID           string `json:"id"`
		Descriptions map[string]struct {
			Value string `json:"value"`
		} `json:"descriptions"`
		Sitelinks map[string]struct {
			Title string `json:"title"`
		} `json:"sitelinks"`
		Claims map[string][]struct {
			Mainsnak struct {
				Datavalue struct {
					Value any `json:"value"`
				} `json:"datavalue"`
			} `json:"mainsnak"`
		} `json:"claims"`
	} `json:"entities"`
}

type WikidataArtistData struct {
	EntityID       string
	Description    string
	WikiquoteTitle string
	Links          []models.ArtistLink
}

func (p *WikidataProvider) SearchArtist(ctx context.Context, query string) (*WikidataArtistData, error) {
	params := url.Values{}
	params.Set("action", "wbsearchentities")
	params.Set("format", "json")
	params.Set("language", "en")
	params.Set("type", "item")
	params.Set("search", query)
	var response wikidataSearchResponse
	if err := p.searchClient.JSON(ctx, "/w/api.php", params, nil, &response); err != nil {
		return nil, err
	}
	if len(response.Search) == 0 {
		return nil, nil
	}
	best := response.Search[0]
	bestScore := 0
	for _, candidate := range response.Search {
		score := search.SimilarityScore(query, candidate.Label) + wikidataBonus(candidate.Description)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	data, err := p.Entity(ctx, best.ID)
	if err != nil || data == nil {
		return data, err
	}
	if data.WikiquoteTitle != "" && search.SimilarityScore(query, data.WikiquoteTitle) < 45 {
		data.WikiquoteTitle = ""
		var filtered []models.ArtistLink
		for _, link := range data.Links {
			if !(link.Provider == "wikiquote" && link.Kind == "page") {
				filtered = append(filtered, link)
			}
		}
		data.Links = filtered
	}
	return data, nil
}

func (p *WikidataProvider) Entity(ctx context.Context, entityID string) (*WikidataArtistData, error) {
	if strings.TrimSpace(entityID) == "" {
		return nil, nil
	}
	params := url.Values{}
	params.Set("action", "wbgetentities")
	params.Set("format", "json")
	params.Set("ids", entityID)
	params.Set("languages", "en")
	params.Set("sitefilter", "enwikiquote|enwiki")
	var response wikidataEntityResponse
	if err := p.entityClient.JSON(ctx, "/w/api.php", params, map[string]string{"Api-User-Agent": defaultUserAgent}, &response); err != nil {
		return nil, err
	}
	entity, ok := response.Entities[entityID]
	if !ok {
		return nil, nil
	}
	description := ""
	if english, ok := entity.Descriptions["en"]; ok {
		description = english.Value
	}
	wikiquoteTitle := ""
	if sitelink, ok := entity.Sitelinks["enwikiquote"]; ok {
		wikiquoteTitle = sitelink.Title
	}
	data := &WikidataArtistData{
		EntityID:       entity.ID,
		Description:    description,
		WikiquoteTitle: wikiquoteTitle,
		Links: []models.ArtistLink{
			{
				Provider:   "wikidata",
				Kind:       "entity",
				URL:        fmt.Sprintf("https://www.wikidata.org/wiki/%s", entity.ID),
				ExternalID: entity.ID,
			},
		},
	}
	if sitelink, ok := entity.Sitelinks["enwiki"]; ok {
		data.Links = append(data.Links, models.ArtistLink{
			Provider: "wikipedia",
			Kind:     "article",
			URL:      "https://en.wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(sitelink.Title, " ", "_")),
		})
	}
	if wikiquoteTitle != "" {
		data.Links = append(data.Links, models.ArtistLink{
			Provider: "wikiquote",
			Kind:     "page",
			URL:      "https://en.wikiquote.org/wiki/" + url.PathEscape(strings.ReplaceAll(wikiquoteTitle, " ", "_")),
		})
	}
	return data, nil
}

func wikidataBonus(description string) int {
	normalized := strings.ToLower(description)
	keywords := []string{
		"singer", "rapper", "songwriter", "band", "musician", "musical group", "record producer", "artist",
	}
	for _, keyword := range keywords {
		if strings.Contains(normalized, keyword) {
			return 30
		}
	}
	return 0
}
