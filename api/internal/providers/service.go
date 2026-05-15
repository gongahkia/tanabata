package providers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/observability"
	"github.com/gongahkia/tanabata/api/internal/search"
)

type EnrichResult struct {
	ArtistID string
	Target   string
	Status   string
	Details  string
}

type Service struct {
	store       *catalog.Store
	musicBrainz *MusicBrainzProvider
	wikidata    *WikidataProvider
	wikiquote   *WikiquoteProvider
	lastfm      *LastFMProvider
}

func NewService(store *catalog.Store, telemetry *observability.Telemetry) *Service {
	return &Service{
		store:       store,
		musicBrainz: NewMusicBrainzProviderWithTelemetry(telemetry),
		wikidata:    NewWikidataProviderWithTelemetry(telemetry),
		wikiquote:   NewWikiquoteProviderWithTelemetry(telemetry),
		lastfm:      NewLastFMProviderWithTelemetry(telemetry),
	}
}

func (s *Service) EnrichExistingArtists(ctx context.Context) ([]EnrichResult, error) {
	artists, err := s.store.ListArtists(ctx, models.ArtistFilters{Limit: 1000})
	if err != nil {
		return nil, err
	}
	results := make([]EnrichResult, 0, len(artists.Data))
	for _, artist := range artists.Data {
		result, err := s.EnrichArtist(ctx, artist.Name)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *Service) EnrichArtist(ctx context.Context, query string) (EnrichResult, error) {
	result := EnrichResult{
		Target: strings.TrimSpace(query),
		Status: "succeeded",
	}

	artist, err := s.loadOrCreateArtist(ctx, query)
	if err != nil {
		return result, err
	}
	if artist == nil {
		result.Status = "failed"
		result.Details = "artist could not be loaded"
		return result, nil
	}
	result.ArtistID = artist.ArtistID

	statuses := []string{}
	details := []string{}
	for _, apply := range []func(context.Context, *models.Artist) (string, string, error){
		s.applyMusicBrainz,
		s.applyWikidata,
		s.applyWikiquote,
		s.applyLastFM,
	} {
		status, detail, err := apply(ctx, artist)
		if err != nil {
			return result, err
		}
		if status != "" {
			statuses = append(statuses, status)
		}
		if detail != "" {
			details = append(details, detail)
		}
	}

	if err := s.store.UpsertArtist(ctx, *artist); err != nil {
		return result, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.store.SetMeta(ctx, "snapshot_version", now); err != nil {
		return result, err
	}
	if err := s.store.UpdateActiveProviders(ctx); err != nil {
		return result, err
	}

	result.Status = overallStatus(statuses)
	result.Details = strings.Join(details, "; ")
	return result, nil
}

func (s *Service) loadOrCreateArtist(ctx context.Context, query string) (*models.Artist, error) {
	artistID, err := s.store.ResolveArtistID(ctx, query)
	if err != nil {
		return nil, err
	}
	if artistID != "" {
		return s.store.ArtistByID(ctx, artistID)
	}
	artist := &models.Artist{
		ArtistID: search.ArtistID(query, ""),
		Name:     strings.TrimSpace(query),
		Aliases:  []string{strings.TrimSpace(query)},
		Genres:   []string{},
		Links:    []models.ArtistLink{},
		ProviderStatus: map[string]string{
			"legacy": "seeded",
		},
	}
	if err := s.store.UpsertArtist(ctx, *artist); err != nil {
		return nil, err
	}
	return artist, nil
}

func (s *Service) applyMusicBrainz(ctx context.Context, artist *models.Artist) (string, string, error) {
	startedAt := time.Now().UTC()
	runID := search.StableHash("musicbrainz", artist.ArtistID, startedAt.Format(time.RFC3339Nano))
	recordRun := func(status, details string) error {
		return s.store.RecordProviderRun(ctx, catalog.ProviderRun{
			RunID:      runID,
			Provider:   s.musicBrainz.Name(),
			Status:     status,
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
			Details:    details,
		})
	}

	candidate, err := s.musicBrainz.SearchArtist(ctx, artist.Name)
	if err != nil {
		if recordErr := s.recordProviderFailure(ctx, s.musicBrainz.Name(), artist.ArtistID, artist.Name, startedAt, err); recordErr != nil {
			return "", "", recordErr
		}
		return "partial", err.Error(), recordRun("failed", err.Error())
	}
	if candidate == nil {
		return "succeeded", "musicbrainz:no match", recordRun("success", "no match")
	}
	if artist.ArtistID != candidate.ArtistID {
		if err := s.store.RekeyArtist(ctx, artist.ArtistID, candidate.ArtistID, candidate.Name); err != nil {
			return "", "", err
		}
		artist.ArtistID = candidate.ArtistID
	}
	artist.Name = candidate.Name
	artist.Aliases = append(artist.Aliases, candidate.Aliases...)
	artist.MBID = candidate.MBID
	if artist.Country == "" {
		artist.Country = candidate.Country
	}
	if artist.LifeSpan.Begin == "" {
		artist.LifeSpan.Begin = candidate.LifeSpan.Begin
	}
	if artist.LifeSpan.End == "" {
		artist.LifeSpan.End = candidate.LifeSpan.End
	}
	artist.Links = append(artist.Links, candidate.Links...)
	artist.ProviderStatus["musicbrainz"] = "fetched"

	releases, releasesErr := s.musicBrainz.Releases(ctx, artist.MBID)
	if releasesErr != nil {
		if recordErr := s.recordProviderFailure(ctx, s.musicBrainz.Name(), artist.ArtistID, artist.Name, startedAt, releasesErr); recordErr != nil {
			return "", "", recordErr
		}
		return "partial", releasesErr.Error(), recordRun("failed", releasesErr.Error())
	}
	if err := s.store.ReplaceReleases(ctx, artist.ArtistID, releases); err != nil {
		return "", "", err
	}
	return "succeeded", fmt.Sprintf("musicbrainz:releases=%d", len(releases)), recordRun("success", fmt.Sprintf("artist=%s releases=%d", artist.Name, len(releases)))
}

func (s *Service) applyWikidata(ctx context.Context, artist *models.Artist) (string, string, error) {
	startedAt := time.Now().UTC()
	runID := search.StableHash("wikidata", artist.ArtistID, startedAt.Format(time.RFC3339Nano))
	recordRun := func(status, details string) error {
		return s.store.RecordProviderRun(ctx, catalog.ProviderRun{
			RunID:      runID,
			Provider:   s.wikidata.Name(),
			Status:     status,
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
			Details:    details,
		})
	}
	data, err := s.wikidata.SearchArtist(ctx, artist.Name)
	if err != nil {
		if recordErr := s.recordProviderFailure(ctx, s.wikidata.Name(), artist.ArtistID, artist.Name, startedAt, err); recordErr != nil {
			return "", "", recordErr
		}
		return "partial", err.Error(), recordRun("failed", err.Error())
	}
	if data == nil {
		return "succeeded", "wikidata:no match", recordRun("success", "no match")
	}
	artist.WikidataID = data.EntityID
	if artist.Description == "" {
		artist.Description = data.Description
	}
	if artist.WikiquoteTitle == "" {
		artist.WikiquoteTitle = data.WikiquoteTitle
	}
	artist.Links = append(artist.Links, data.Links...)
	artist.ProviderStatus["wikidata"] = "fetched"
	return "succeeded", fmt.Sprintf("wikidata:%s", data.EntityID), recordRun("success", fmt.Sprintf("entity=%s", data.EntityID))
}

func (s *Service) applyWikiquote(ctx context.Context, artist *models.Artist) (string, string, error) {
	startedAt := time.Now().UTC()
	runID := search.StableHash("wikiquote", artist.ArtistID, startedAt.Format(time.RFC3339Nano))
	recordRun := func(status, details string) error {
		return s.store.RecordProviderRun(ctx, catalog.ProviderRun{
			RunID:      runID,
			Provider:   s.wikiquote.Name(),
			Status:     status,
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
			Details:    details,
		})
	}

	title := artist.WikiquoteTitle
	if title == "" {
		found, err := s.wikiquote.SearchPage(ctx, artist.Name)
		if err != nil {
			if recordErr := s.recordProviderFailure(ctx, s.wikiquote.Name(), artist.ArtistID, artist.Name, startedAt, err); recordErr != nil {
				return "", "", recordErr
			}
			return "partial", err.Error(), recordRun("failed", err.Error())
		}
		title = found
	}
	if title == "" {
		return "succeeded", "wikiquote:no page", recordRun("success", "no page")
	}
	artist.WikiquoteTitle = title
	quotes, err := s.wikiquote.Quotes(ctx, title)
	if err != nil {
		if recordErr := s.recordProviderFailure(ctx, s.wikiquote.Name(), artist.ArtistID, title, startedAt, err); recordErr != nil {
			return "", "", recordErr
		}
		return "partial", err.Error(), recordRun("failed", err.Error())
	}
	for _, item := range quotes {
		source := item.Source
		if source.SourceID == "" {
			source.SourceID = search.SourceID("wikiquote", source.URL)
		}
		if err := s.store.UpsertSource(ctx, source); err != nil {
			return "", "", err
		}
		quote := models.Quote{
			QuoteID:          search.QuoteID(artist.ArtistID, search.NormalizeText(item.Text), source.URL),
			Text:             item.Text,
			ArtistID:         artist.ArtistID,
			ArtistName:       artist.Name,
			SourceID:         source.SourceID,
			SourceType:       item.SourceType,
			WorkTitle:        item.Section,
			Tags:             sectionTags(item.Section),
			ProvenanceStatus: "source_attributed",
			ConfidenceScore:  0.9,
			ProviderOrigin:   s.wikiquote.Name(),
			Evidence: []string{
				"Matched Wikiquote page: " + title,
				"Section: " + item.Section,
				"Source URL: " + source.URL,
			},
			License:        source.License,
			FirstSeenAt:    source.RetrievedAt,
			LastVerifiedAt: source.RetrievedAt,
			Source:         &source,
		}
		if err := s.store.UpsertQuote(ctx, quote); err != nil {
			return "", "", err
		}
	}
	artist.ProviderStatus["wikiquote"] = "fetched"
	return "succeeded", fmt.Sprintf("wikiquote:quotes=%d", len(quotes)), recordRun("success", fmt.Sprintf("page=%s quotes=%d", title, len(quotes)))
}

func (s *Service) applyLastFM(ctx context.Context, artist *models.Artist) (string, string, error) {
	if !s.lastfm.Enabled() {
		return "skipped", "lastfm:disabled", nil
	}
	startedAt := time.Now().UTC()
	runID := search.StableHash("lastfm", artist.ArtistID, startedAt.Format(time.RFC3339Nano))
	recordRun := func(status, details string) error {
		return s.store.RecordProviderRun(ctx, catalog.ProviderRun{
			RunID:      runID,
			Provider:   s.lastfm.Name(),
			Status:     status,
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
			Details:    details,
		})
	}
	data, err := s.lastfm.ArtistInfo(ctx, *artist)
	if err != nil {
		if recordErr := s.recordProviderFailure(ctx, s.lastfm.Name(), artist.ArtistID, artist.Name, startedAt, err); recordErr != nil {
			return "", "", recordErr
		}
		return "partial", err.Error(), recordRun("failed", err.Error())
	}
	if data == nil {
		return "skipped", "lastfm:disabled", recordRun("success", "disabled")
	}
	if artist.BioSummary == "" {
		artist.BioSummary = data.Summary
	}
	artist.Genres = append(artist.Genres, data.Tags...)
	artist.Links = append(artist.Links, data.Links...)
	artist.ProviderStatus["lastfm"] = "fetched"
	for _, related := range data.Related {
		relatedArtist := models.Artist{
			ArtistID: related.ArtistID,
			Name:     related.Name,
			Aliases:  []string{related.Name},
			Genres:   []string{},
			Links:    []models.ArtistLink{},
			ProviderStatus: map[string]string{
				"lastfm": "related_only",
			},
		}
		if err := s.store.UpsertArtist(ctx, relatedArtist); err != nil {
			return "", "", err
		}
	}
	if err := s.store.ReplaceArtistRelations(ctx, artist.ArtistID, data.Related); err != nil {
		return "", "", err
	}
	return "succeeded", fmt.Sprintf("lastfm:related=%d", len(data.Related)), recordRun("success", fmt.Sprintf("tags=%d related=%d", len(data.Tags), len(data.Related)))
}

func (s *Service) recordProviderFailure(ctx context.Context, provider, artistID, contextValue string, startedAt time.Time, err error) error {
	return s.store.RecordProviderError(ctx, catalog.ProviderError{
		ErrorID:    search.StableHash(provider, artistID, err.Error(), startedAt.Format(time.RFC3339Nano)),
		Provider:   provider,
		OccurredAt: time.Now().UTC(),
		Context:    contextValue,
		Message:    err.Error(),
	})
}

func overallStatus(statuses []string) string {
	result := "succeeded"
	for _, status := range statuses {
		switch status {
		case "partial":
			return "partial"
		case "failed":
			result = "failed"
		}
	}
	return result
}

func sectionTags(section string) []string {
	normalized := search.NormalizeText(section)
	if normalized == "" || normalized == "quotes" {
		return nil
	}
	return []string{normalized}
}
