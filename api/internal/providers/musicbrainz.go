package providers

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

type MusicBrainzProvider struct {
	client *HTTPClient
}

func NewMusicBrainzProvider() *MusicBrainzProvider {
	return &MusicBrainzProvider{client: NewHTTPClient("https://musicbrainz.org")}
}

func (p *MusicBrainzProvider) Name() string {
	return "musicbrainz"
}

type musicBrainzSearchResponse struct {
	Artists []struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		Country        string `json:"country"`
		Disambiguation string `json:"disambiguation"`
		LifeSpan       struct {
			Begin string `json:"begin"`
			End   string `json:"end"`
		} `json:"life-span"`
		Aliases []struct {
			Name string `json:"name"`
		} `json:"aliases"`
	} `json:"artists"`
}

type musicBrainzReleaseResponse struct {
	ReleaseGroups []struct {
		ID               string `json:"id"`
		PrimaryType      string `json:"primary-type"`
		Title            string `json:"title"`
		FirstReleaseDate string `json:"first-release-date"`
	} `json:"release-groups"`
}

func (p *MusicBrainzProvider) SearchArtist(ctx context.Context, query string) (*models.Artist, error) {
	var response musicBrainzSearchResponse
	params := url.Values{}
	params.Set("query", fmt.Sprintf("artist:%s", query))
	params.Set("fmt", "json")
	params.Set("limit", "5")
	if err := p.client.JSON(ctx, "/ws/2/artist", params, nil, &response); err != nil {
		return nil, err
	}
	if len(response.Artists) == 0 {
		return nil, nil
	}
	best := response.Artists[0]
	bestScore := 0
	for _, candidate := range response.Artists {
		score := search.SimilarityScore(query, candidate.Name)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	return mapMusicBrainzArtist(best), nil
}

func (p *MusicBrainzProvider) Releases(ctx context.Context, mbid string) ([]models.Release, error) {
	if strings.TrimSpace(mbid) == "" {
		return nil, nil
	}
	var response musicBrainzReleaseResponse
	params := url.Values{}
	params.Set("artist", mbid)
	params.Set("fmt", "json")
	params.Set("limit", "25")
	if err := p.client.JSON(ctx, "/ws/2/release-group", params, nil, &response); err != nil {
		return nil, err
	}
	releases := make([]models.Release, 0, len(response.ReleaseGroups))
	for _, release := range response.ReleaseGroups {
		releaseCopy := release
		var year *int
		if len(releaseCopy.FirstReleaseDate) >= 4 {
			if parsed := parseYear(releaseCopy.FirstReleaseDate[:4]); parsed != nil {
				year = parsed
			}
		}
		releases = append(releases, models.Release{
			ReleaseID: releaseCopy.ID,
			Title:     releaseCopy.Title,
			Year:      year,
			Kind:      releaseCopy.PrimaryType,
			Provider:  p.Name(),
			URL:       "https://musicbrainz.org/release-group/" + releaseCopy.ID,
		})
	}
	sort.SliceStable(releases, func(i, j int) bool {
		left := 0
		if releases[i].Year != nil {
			left = *releases[i].Year
		}
		right := 0
		if releases[j].Year != nil {
			right = *releases[j].Year
		}
		if left == right {
			return releases[i].Title < releases[j].Title
		}
		return left > right
	})
	return releases, nil
}

func mapMusicBrainzArtist(artist struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Country        string `json:"country"`
	Disambiguation string `json:"disambiguation"`
	LifeSpan       struct {
		Begin string `json:"begin"`
		End   string `json:"end"`
	} `json:"life-span"`
	Aliases []struct {
		Name string `json:"name"`
	} `json:"aliases"`
}) *models.Artist {
	aliases := make([]string, 0, len(artist.Aliases)+1)
	aliases = append(aliases, artist.Name)
	for _, alias := range artist.Aliases {
		aliases = append(aliases, alias.Name)
	}
	return &models.Artist{
		ArtistID: artist.ID,
		Name:     artist.Name,
		Aliases:  aliases,
		MBID:     artist.ID,
		Country:  artist.Country,
		LifeSpan: models.LifeSpan{
			Begin: artist.LifeSpan.Begin,
			End:   artist.LifeSpan.End,
		},
		Links: []models.ArtistLink{
			{
				Provider:   "musicbrainz",
				Kind:       "artist",
				URL:        "https://musicbrainz.org/artist/" + artist.ID,
				ExternalID: artist.ID,
			},
		},
		ProviderStatus: map[string]string{
			"musicbrainz": "fetched",
		},
	}
}

func parseYear(raw string) *int {
	if len(raw) != 4 {
		return nil
	}
	value := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return nil
		}
		value = (value * 10) + int(r-'0')
	}
	return &value
}
