package providers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

type Service struct {
	store       *catalog.Store
	musicBrainz *MusicBrainzProvider
	wikidata    *WikidataProvider
	wikiquote   *WikiquoteProvider
	lastfm      *LastFMProvider
}

func NewService(store *catalog.Store) *Service {
	return &Service{
		store:       store,
		musicBrainz: NewMusicBrainzProvider(),
		wikidata:    NewWikidataProvider(),
		wikiquote:   NewWikiquoteProvider(),
		lastfm:      NewLastFMProvider(),
	}
}

func (s *Service) EnrichExistingArtists(ctx context.Context) error {
	artists, err := s.store.ListArtists(ctx, models.ArtistFilters{Limit: 1000})
	if err != nil {
		return err
	}
	for _, artist := range artists.Data {
		if err := s.EnrichArtist(ctx, artist.Name); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) EnrichArtist(ctx context.Context, query string) error {
	artist, err := s.loadOrCreateArtist(ctx, query)
	if err != nil {
		return err
	}
	if artist == nil {
		return nil
	}

	if err := s.applyMusicBrainz(ctx, artist); err != nil {
		return err
	}
	if err := s.applyWikidata(ctx, artist); err != nil {
		return err
	}
	if err := s.applyWikiquote(ctx, artist); err != nil {
		return err
	}
	if err := s.applyLastFM(ctx, artist); err != nil {
		return err
	}

	if err := s.store.UpsertArtist(ctx, *artist); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.store.SetMeta(ctx, "snapshot_version", now); err != nil {
		return err
	}
	return s.store.UpdateActiveProviders(ctx)
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

func (s *Service) applyMusicBrainz(ctx context.Context, artist *models.Artist) error {
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
		_ = s.store.RecordProviderError(ctx, catalog.ProviderError{
			ErrorID:    search.StableHash("musicbrainz", artist.ArtistID, err.Error(), startedAt.Format(time.RFC3339Nano)),
			Provider:   s.musicBrainz.Name(),
			OccurredAt: time.Now().UTC(),
			Context:    artist.Name,
			Message:    err.Error(),
		})
		return recordRun("failed", err.Error())
	}
	if candidate == nil {
		return recordRun("success", "no match")
	}
	if artist.ArtistID != candidate.ArtistID {
		if err := s.store.RekeyArtist(ctx, artist.ArtistID, candidate.ArtistID, candidate.Name); err != nil {
			return err
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

	releases, err := s.musicBrainz.Releases(ctx, artist.MBID)
	if err == nil {
		if err := s.store.ReplaceReleases(ctx, artist.ArtistID, releases); err != nil {
			return err
		}
	}
	return recordRun("success", fmt.Sprintf("artist=%s releases=%d", artist.Name, len(releases)))
}

func (s *Service) applyWikidata(ctx context.Context, artist *models.Artist) error {
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
		_ = s.store.RecordProviderError(ctx, catalog.ProviderError{
			ErrorID:    search.StableHash("wikidata", artist.ArtistID, err.Error(), startedAt.Format(time.RFC3339Nano)),
			Provider:   s.wikidata.Name(),
			OccurredAt: time.Now().UTC(),
			Context:    artist.Name,
			Message:    err.Error(),
		})
		return recordRun("failed", err.Error())
	}
	if data == nil {
		return recordRun("success", "no match")
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
	return recordRun("success", fmt.Sprintf("entity=%s", data.EntityID))
}

func (s *Service) applyWikiquote(ctx context.Context, artist *models.Artist) error {
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
			return recordRun("failed", err.Error())
		}
		title = found
	}
	if title == "" {
		return recordRun("success", "no page")
	}
	artist.WikiquoteTitle = title
	quotes, err := s.wikiquote.Quotes(ctx, title)
	if err != nil {
		_ = s.store.RecordProviderError(ctx, catalog.ProviderError{
			ErrorID:    search.StableHash("wikiquote", artist.ArtistID, err.Error(), startedAt.Format(time.RFC3339Nano)),
			Provider:   s.wikiquote.Name(),
			OccurredAt: time.Now().UTC(),
			Context:    title,
			Message:    err.Error(),
		})
		return recordRun("failed", err.Error())
	}
	for _, item := range quotes {
		source := item.Source
		if source.SourceID == "" {
			source.SourceID = search.SourceID("wikiquote", source.URL)
		}
		if err := s.store.UpsertSource(ctx, source); err != nil {
			return err
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
			License:          source.License,
			FirstSeenAt:      source.RetrievedAt,
			LastVerifiedAt:   source.RetrievedAt,
			Source:           &source,
		}
		if err := s.store.UpsertQuote(ctx, quote); err != nil {
			return err
		}
	}
	artist.ProviderStatus["wikiquote"] = "fetched"
	return recordRun("success", fmt.Sprintf("page=%s quotes=%d", title, len(quotes)))
}

func (s *Service) applyLastFM(ctx context.Context, artist *models.Artist) error {
	if !s.lastfm.Enabled() {
		return nil
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
		_ = s.store.RecordProviderError(ctx, catalog.ProviderError{
			ErrorID:    search.StableHash("lastfm", artist.ArtistID, err.Error(), startedAt.Format(time.RFC3339Nano)),
			Provider:   s.lastfm.Name(),
			OccurredAt: time.Now().UTC(),
			Context:    artist.Name,
			Message:    err.Error(),
		})
		return recordRun("failed", err.Error())
	}
	if data == nil {
		return recordRun("success", "disabled")
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
			return err
		}
	}
	if err := s.store.ReplaceArtistRelations(ctx, artist.ArtistID, data.Related); err != nil {
		return err
	}
	return recordRun("success", fmt.Sprintf("tags=%d related=%d", len(data.Tags), len(data.Related)))
}

func sectionTags(section string) []string {
	normalized := search.NormalizeText(section)
	if normalized == "" || normalized == "quotes" {
		return nil
	}
	return []string{normalized}
}
