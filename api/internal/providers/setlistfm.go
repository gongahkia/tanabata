package providers

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/gongahkia/tanabata/api/internal/observability"
)

type SetlistFMProvider struct {
	client *HTTPClient
	apiKey string
}

func NewSetlistFMProvider() *SetlistFMProvider {
	return NewSetlistFMProviderWithTelemetry(nil)
}

func NewSetlistFMProviderWithTelemetry(telemetry *observability.Telemetry) *SetlistFMProvider {
	return &SetlistFMProvider{
		client: NewHTTPClient("https://api.setlist.fm").ConfigureProvider("setlistfm", telemetry),
		apiKey: os.Getenv("SETLISTFM_API_KEY"),
	}
}

func (p *SetlistFMProvider) Name() string {
	return "setlistfm"
}

func (p *SetlistFMProvider) Enabled() bool {
	return strings.TrimSpace(p.apiKey) != ""
}

type SetlistArtist struct {
	Name string `json:"name"`
	MBID string `json:"mbid"`
}

type SetlistVenue struct {
	Name string `json:"name"`
	City struct {
		Name    string `json:"name"`
		Country struct {
			Name string `json:"name"`
		} `json:"country"`
	} `json:"city"`
}

type Setlist struct {
	ID          string        `json:"id"`
	EventDate   string        `json:"eventDate"`
	URL         string        `json:"url"`
	Artist      SetlistArtist `json:"artist"`
	Venue       SetlistVenue  `json:"venue"`
	Sets        any           `json:"sets,omitempty"`
	LastUpdated string        `json:"lastUpdated"`
}

type setlistResponse struct {
	Setlist []Setlist `json:"setlist"`
}

func (p *SetlistFMProvider) ArtistSetlists(ctx context.Context, mbid string) ([]Setlist, error) {
	if !p.Enabled() {
		return nil, nil
	}
	params := url.Values{}
	params.Set("p", "1")
	var response setlistResponse
	headers := map[string]string{
		"x-api-key": p.apiKey,
		"Accept":    "application/json",
	}
	if err := p.client.JSON(ctx, fmt.Sprintf("/1.0/artist/%s/setlists", url.PathEscape(mbid)), params, headers, &response); err != nil {
		return nil, err
	}
	return response.Setlist, nil
}
