package providers

import (
	"context"
	"net/url"
	"strings"
)

type LyricsResult struct {
	Provider     string `json:"provider"`
	Artist       string `json:"artist"`
	Track        string `json:"track"`
	Lyrics       string `json:"lyrics,omitempty"`
	SyncedLyrics string `json:"synced_lyrics,omitempty"`
	SourceURL    string `json:"source_url,omitempty"`
}

type LRCLIBProvider struct {
	client *HTTPClient
}

func NewLRCLIBProvider() *LRCLIBProvider {
	return &LRCLIBProvider{client: NewHTTPClient("https://lrclib.net")}
}

func (p *LRCLIBProvider) Name() string {
	return "lrclib"
}

type lrclibResponse struct {
	TrackName    string `json:"trackName"`
	ArtistName   string `json:"artistName"`
	PlainLyrics  string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
}

func (p *LRCLIBProvider) Lyrics(ctx context.Context, artist, track string) (*LyricsResult, error) {
	params := url.Values{}
	params.Set("artist_name", artist)
	params.Set("track_name", track)
	var response lrclibResponse
	if err := p.client.JSON(ctx, "/api/get", params, nil, &response); err != nil {
		return nil, err
	}
	return &LyricsResult{
		Provider:     p.Name(),
		Artist:       response.ArtistName,
		Track:        response.TrackName,
		Lyrics:       response.PlainLyrics,
		SyncedLyrics: response.SyncedLyrics,
		SourceURL:    "https://lrclib.net",
	}, nil
}

type LyricsOVHProvider struct {
	client *HTTPClient
}

func NewLyricsOVHProvider() *LyricsOVHProvider {
	return &LyricsOVHProvider{client: NewHTTPClient("https://api.lyrics.ovh")}
}

func (p *LyricsOVHProvider) Name() string {
	return "lyricsovh"
}

type lyricsOVHResponse struct {
	Lyrics string `json:"lyrics"`
}

func (p *LyricsOVHProvider) Lyrics(ctx context.Context, artist, track string) (*LyricsResult, error) {
	path := "/v1/" + url.PathEscape(strings.TrimSpace(artist)) + "/" + url.PathEscape(strings.TrimSpace(track))
	var response lyricsOVHResponse
	if err := p.client.JSON(ctx, path, nil, nil, &response); err != nil {
		return nil, err
	}
	return &LyricsResult{
		Provider:  p.Name(),
		Artist:    artist,
		Track:     track,
		Lyrics:    response.Lyrics,
		SourceURL: "https://api.lyrics.ovh",
	}, nil
}
