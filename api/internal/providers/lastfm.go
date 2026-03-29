package providers

import (
	"context"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

type LastFMProvider struct {
	client *HTTPClient
	apiKey string
}

func NewLastFMProvider() *LastFMProvider {
	return &LastFMProvider{
		client: NewHTTPClient("https://ws.audioscrobbler.com"),
		apiKey: os.Getenv("LASTFM_API_KEY"),
	}
}

func (p *LastFMProvider) Name() string {
	return "lastfm"
}

func (p *LastFMProvider) Enabled() bool {
	return strings.TrimSpace(p.apiKey) != ""
}

type lastFMArtistInfo struct {
	Artist struct {
		Name string `json:"name"`
		MBID string `json:"mbid"`
		URL  string `json:"url"`
		Tags struct {
			Tag []struct {
				Name string `json:"name"`
			} `json:"tag"`
		} `json:"tags"`
		Similar struct {
			Artist []struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"artist"`
		} `json:"similar"`
		Bio struct {
			Summary string `json:"summary"`
		} `json:"bio"`
	} `json:"artist"`
}

type LastFMArtistData struct {
	Summary string
	Tags    []string
	Related []models.RelatedArtist
	Links   []models.ArtistLink
}

func (p *LastFMProvider) ArtistInfo(ctx context.Context, artist models.Artist) (*LastFMArtistData, error) {
	if !p.Enabled() {
		return nil, nil
	}
	params := url.Values{}
	params.Set("method", "artist.getInfo")
	if artist.MBID != "" {
		params.Set("mbid", artist.MBID)
	} else {
		params.Set("artist", artist.Name)
	}
	params.Set("api_key", p.apiKey)
	params.Set("format", "json")
	var response lastFMArtistInfo
	if err := p.client.JSON(ctx, "/2.0/", params, nil, &response); err != nil {
		return nil, err
	}
	data := &LastFMArtistData{
		Summary: strings.TrimSpace(stripHTML(response.Artist.Bio.Summary)),
		Links: []models.ArtistLink{
			{
				Provider:   "lastfm",
				Kind:       "artist",
				URL:        response.Artist.URL,
				ExternalID: response.Artist.MBID,
			},
		},
	}
	for _, tag := range response.Artist.Tags.Tag {
		data.Tags = append(data.Tags, tag.Name)
	}
	sort.Strings(data.Tags)
	for _, related := range response.Artist.Similar.Artist {
		data.Related = append(data.Related, models.RelatedArtist{
			ArtistID: search.ArtistID(related.Name, ""),
			Name:     related.Name,
			Relation: "similar",
			Score:    0.5,
			Provider: p.Name(),
		})
	}
	return data, nil
}

func stripHTML(input string) string {
	replacer := strings.NewReplacer(
		"<a href=\"", "",
		"</a>", "",
		"<b>", "",
		"</b>", "",
		"&quot;", "\"",
		"&amp;", "&",
	)
	output := replacer.Replace(input)
	output = strings.ReplaceAll(output, "<br />", " ")
	output = strings.ReplaceAll(output, "<br/>", " ")
	output = strings.TrimSpace(output)
	if len(output) > 300 {
		return strings.TrimSpace(output[:300])
	}
	return output
}
