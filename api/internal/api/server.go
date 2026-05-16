package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/observability"
	"github.com/gongahkia/tanabata/api/internal/providers"
	"github.com/gongahkia/tanabata/api/internal/search"
)

type Server struct {
	store             *catalog.Store
	telemetry         *observability.Telemetry
	logger            *slog.Logger
	setlistFM         *providers.SetlistFMProvider
	lrclib            *providers.LRCLIBProvider
	lyricsOVH         *providers.LyricsOVHProvider
	providerInventory []models.ProviderSummary
	contractValidator *runtimeContractValidator
}

func NewServer(store *catalog.Store, telemetry *observability.Telemetry) *Server {
	server := &Server{
		store:     store,
		telemetry: telemetry,
		logger:    slog.Default(),
		setlistFM: providers.NewSetlistFMProviderWithTelemetry(telemetry),
		lrclib:    providers.NewLRCLIBProviderWithTelemetry(telemetry),
		lyricsOVH: providers.NewLyricsOVHProviderWithTelemetry(telemetry),
		providerInventory: []models.ProviderSummary{
			{Provider: "lastfm", Category: "enrichment", Enabled: providers.NewLastFMProvider().Enabled()},
			{Provider: "lrclib", Category: "runtime", Enabled: true},
			{Provider: "lyricsovh", Category: "runtime", Enabled: true},
			{Provider: "musicbrainz", Category: "enrichment", Enabled: true},
			{Provider: "quotefancy", Category: "bootstrap", Enabled: true},
			{Provider: "setlistfm", Category: "runtime", Enabled: providers.NewSetlistFMProvider().Enabled()},
			{Provider: "wikidata", Category: "enrichment", Enabled: true},
			{Provider: "wikiquote", Category: "enrichment", Enabled: true},
		},
	}
	if contractValidationEnabled() {
		if err := server.enableContractValidation(os.Getenv(contractSpecPathEnv)); err != nil {
			server.logger.Error("openapi_contract_validation_disabled", "error", err)
		}
	}
	return server
}

func (s *Server) Router() *gin.Engine {
	router := gin.New()
	router.Use(requestIDMiddleware())
	router.Use(s.corsMiddleware())
	router.Use(s.structuredLogger())
	router.Use(s.recoveryMiddleware())
	if s.contractValidator != nil {
		router.Use(s.contractValidator.middleware())
	}
	if s.telemetry != nil {
		router.Use(s.telemetry.Middleware())
		router.GET("/metrics", s.telemetry.MetricsHandler())
	}

	router.GET("/livez", s.livez)
	router.GET("/readyz", s.readyz)
	router.GET("/health", s.health)

	router.GET("/quotes", s.legacyQuotes)
	router.GET("/quotes/random", s.legacyRandomQuote)
	router.GET("/quotes/:author", s.legacyAuthorQuotes)

	v1 := router.Group("/v1")
	{
		v1.GET("/artists", s.listArtists)
		v1.GET("/artists/:artist_id", s.artistByID)
		v1.GET("/artists/:artist_id/quotes", s.artistQuotes)
		v1.GET("/artists/:artist_id/related", s.artistRelated)
		v1.GET("/artists/:artist_id/releases", s.artistReleases)
		v1.GET("/artists/:artist_id/setlists", s.artistSetlists)

		v1.GET("/quotes", s.listQuotes)
		v1.GET("/quotes/random", s.randomQuote)
		v1.GET("/quotes/:quote_id", s.quoteByID)
		v1.GET("/quotes/:quote_id/provenance", s.quoteProvenance)

		v1.GET("/sources/:source_id", s.sourceByID)
		v1.GET("/providers", s.providers)
		v1.GET("/providers/:provider/runs", s.providerRuns)
		v1.GET("/providers/:provider/errors", s.providerErrors)
		v1.GET("/jobs", s.jobs)
		v1.GET("/jobs/:job_id", s.jobByID)
		v1.GET("/review/queue", s.reviewQueue)
		v1.GET("/review/stale", s.staleQuotes)
		v1.GET("/search", s.search)
		v1.GET("/stats", s.stats)
		v1.GET("/integrity", s.integrity)
		v1.GET("/lyrics", s.lyrics)
	}

	return router
}

func (s *Server) livez(c *gin.Context) {
	dataResponse(c, http.StatusOK, gin.H{"status": "alive"}, nil)
}

func (s *Server) readyz(c *gin.Context) {
	if err := s.store.Ping(c.Request.Context()); err != nil {
		errorResponse(c, http.StatusServiceUnavailable, "catalog_unavailable", "catalog is not ready", map[string]any{"error": err.Error()})
		return
	}
	stats, err := s.store.Stats(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusServiceUnavailable, "catalog_unavailable", "catalog metadata is unavailable", map[string]any{"error": err.Error()})
		return
	}
	dataResponse(c, http.StatusOK, gin.H{"status": "ready"}, stats)
}

func (s *Server) health(c *gin.Context) {
	stats, err := s.store.Stats(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "health_failed", "failed to load service metadata", map[string]any{"error": err.Error()})
		return
	}
	dataResponse(c, http.StatusOK, gin.H{"status": "ok"}, stats)
}

func (s *Server) legacyQuotes(c *gin.Context) {
	quotes, err := s.store.LegacyQuotes(c.Request.Context(), "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, quotes)
}

func (s *Server) legacyRandomQuote(c *gin.Context) {
	quote, err := s.store.RandomLegacyQuote(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if quote == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No quotes available"})
		return
	}
	c.JSON(http.StatusOK, quote)
}

func (s *Server) legacyAuthorQuotes(c *gin.Context) {
	quotes, err := s.store.LegacyQuotes(c.Request.Context(), c.Param("author"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, quotes)
}

func (s *Server) listArtists(c *gin.Context) {
	response, err := s.store.ListArtists(c.Request.Context(), models.ArtistFilters{
		Query:          c.Query("q"),
		MBID:           c.Query("mbid"),
		WikiquoteTitle: c.Query("wikiquote_title"),
		Tag:            c.Query("tag"),
		Limit:          parseInt(c.Query("limit")),
		Offset:         parseInt(c.Query("offset")),
	})
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "artist_list_failed", "failed to list artists", map[string]any{"error": err.Error()})
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

func (s *Server) artistByID(c *gin.Context) {
	artist, err := s.store.ArtistByID(c.Request.Context(), c.Param("artist_id"))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "artist_lookup_failed", "failed to load artist", map[string]any{"error": err.Error()})
		return
	}
	if artist == nil {
		errorResponse(c, http.StatusNotFound, "artist_not_found", "artist not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, artist, nil)
}

func (s *Server) artistQuotes(c *gin.Context) {
	response, err := s.store.ArtistQuotes(c.Request.Context(), c.Param("artist_id"), models.QuoteFilters{
		Query:            c.Query("q"),
		Tag:              c.Query("tag"),
		Source:           c.Query("source"),
		ProvenanceStatus: c.Query("provenance_status"),
		Limit:            parseInt(c.Query("limit")),
		Offset:           parseInt(c.Query("offset")),
		Sort:             c.Query("sort"),
	})
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "artist_quotes_failed", "failed to list artist quotes", map[string]any{"error": err.Error()})
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

func (s *Server) artistRelated(c *gin.Context) {
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "artist_related_failed", "failed to load service metadata", map[string]any{"error": err.Error()})
		return
	}
	related, err := s.store.RelatedArtists(c.Request.Context(), c.Param("artist_id"))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "artist_related_failed", "failed to load related artists", map[string]any{"error": err.Error()})
		return
	}
	listResponse(c, http.StatusOK, related, meta, models.Pagination{Limit: len(related), Offset: 0, Total: len(related)})
}

func (s *Server) artistReleases(c *gin.Context) {
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "artist_releases_failed", "failed to load service metadata", map[string]any{"error": err.Error()})
		return
	}
	releases, err := s.store.Releases(c.Request.Context(), c.Param("artist_id"))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "artist_releases_failed", "failed to load releases", map[string]any{"error": err.Error()})
		return
	}
	listResponse(c, http.StatusOK, releases, meta, models.Pagination{Limit: len(releases), Offset: 0, Total: len(releases)})
}

func (s *Server) artistSetlists(c *gin.Context) {
	if !s.setlistFM.Enabled() {
		errorResponse(c, http.StatusServiceUnavailable, "provider_disabled", "setlist.fm is disabled", map[string]any{"provider": "setlistfm"})
		return
	}
	artist, err := s.store.ArtistByID(c.Request.Context(), c.Param("artist_id"))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "artist_lookup_failed", "failed to load artist", map[string]any{"error": err.Error()})
		return
	}
	if artist == nil {
		errorResponse(c, http.StatusNotFound, "artist_not_found", "artist not found", nil)
		return
	}
	if artist.MBID == "" {
		errorResponse(c, http.StatusBadRequest, "artist_missing_mbid", "artist does not have a MusicBrainz ID", nil)
		return
	}

	cacheKey := artist.MBID
	if payload, refreshedAt, expiresAt, ok, err := s.store.GetProviderCache(c.Request.Context(), "setlistfm", "setlists", cacheKey); err == nil && ok {
		var setlists []providers.Setlist
		if unmarshalErr := json.Unmarshal([]byte(payload), &setlists); unmarshalErr == nil {
			dataResponse(c, http.StatusOK, setlists, gin.H{
				"provider":     "setlistfm",
				"cached":       true,
				"refreshed_at": refreshedAt,
				"expires_at":   expiresAt,
			})
			return
		}
	} else if err != nil {
		errorResponse(c, http.StatusInternalServerError, "provider_cache_failed", "failed to access provider cache", map[string]any{"error": err.Error()})
		return
	}

	setlists, err := s.setlistFM.ArtistSetlists(c.Request.Context(), artist.MBID)
	if err != nil {
		errorResponse(c, http.StatusBadGateway, "provider_request_failed", "failed to fetch setlists", map[string]any{"provider": "setlistfm", "error": err.Error()})
		return
	}
	body, _ := json.Marshal(setlists)
	if err := s.store.SetProviderCache(c.Request.Context(), "setlistfm", "setlists", cacheKey, string(body), 6*time.Hour); err != nil {
		errorResponse(c, http.StatusInternalServerError, "provider_cache_failed", "failed to cache setlists", map[string]any{"error": err.Error()})
		return
	}
	dataResponse(c, http.StatusOK, setlists, gin.H{
		"provider": "setlistfm",
		"cached":   false,
	})
}

func (s *Server) listQuotes(c *gin.Context) {
	filters := models.QuoteFilters{
		Artist:           c.Query("artist"),
		ArtistID:         c.Query("artist_id"),
		Query:            c.Query("q"),
		Tag:              c.Query("tag"),
		Source:           c.Query("source"),
		ProvenanceStatus: c.Query("provenance_status"),
		Limit:            parseInt(c.Query("limit")),
		Offset:           parseInt(c.Query("offset")),
		Sort:             c.Query("sort"),
	}
	if filters.Artist != "" && filters.ArtistID == "" {
		if resolved, err := s.store.ResolveArtistID(c.Request.Context(), filters.Artist); err == nil && resolved != "" {
			filters.ArtistID = resolved
			filters.Artist = ""
		}
	}
	response, err := s.store.ListQuotes(c.Request.Context(), filters)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "quote_list_failed", "failed to list quotes", map[string]any{"error": err.Error()})
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

func (s *Server) randomQuote(c *gin.Context) {
	filters := models.QuoteFilters{
		Artist:           c.Query("artist"),
		ArtistID:         c.Query("artist_id"),
		Query:            c.Query("q"),
		Tag:              c.Query("tag"),
		Source:           c.Query("source"),
		ProvenanceStatus: c.Query("provenance_status"),
	}
	if filters.Artist != "" && filters.ArtistID == "" {
		if resolved, err := s.store.ResolveArtistID(c.Request.Context(), filters.Artist); err == nil && resolved != "" {
			filters.ArtistID = resolved
			filters.Artist = ""
		}
	}
	quote, err := s.store.RandomQuote(c.Request.Context(), filters)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "quote_random_failed", "failed to select quote", map[string]any{"error": err.Error()})
		return
	}
	if quote == nil {
		errorResponse(c, http.StatusNotFound, "quote_not_found", "quote not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, quote, nil)
}

func (s *Server) quoteByID(c *gin.Context) {
	quote, err := s.store.QuoteByID(c.Request.Context(), c.Param("quote_id"))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "quote_lookup_failed", "failed to load quote", map[string]any{"error": err.Error()})
		return
	}
	if quote == nil {
		errorResponse(c, http.StatusNotFound, "quote_not_found", "quote not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, quote, nil)
}

func (s *Server) quoteProvenance(c *gin.Context) {
	provenance, err := s.store.QuoteProvenance(c.Request.Context(), c.Param("quote_id"))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "quote_provenance_failed", "failed to load quote provenance", map[string]any{"error": err.Error()})
		return
	}
	if provenance == nil {
		errorResponse(c, http.StatusNotFound, "quote_not_found", "quote not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, provenance, nil)
}

func (s *Server) reviewQueue(c *gin.Context) {
	response, err := s.store.ReviewQueue(c.Request.Context(), models.ReviewQueueFilters{
		ProvenanceStatus: c.Query("provenance_status"),
		Limit:            parseInt(c.Query("limit")),
		Offset:           parseInt(c.Query("offset")),
	})
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "review_queue_failed", "failed to load review queue", map[string]any{"error": err.Error()})
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

func (s *Server) staleQuotes(c *gin.Context) {
	response, err := s.store.StaleQuotes(c.Request.Context(), models.ReviewQueueFilters{
		Limit:  parseInt(c.Query("limit")),
		Offset: parseInt(c.Query("offset")),
	})
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "stale_quotes_failed", "failed to load stale quote review set", map[string]any{"error": err.Error()})
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

func (s *Server) sourceByID(c *gin.Context) {
	source, err := s.store.SourceByID(c.Request.Context(), c.Param("source_id"))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "source_lookup_failed", "failed to load source", map[string]any{"error": err.Error()})
		return
	}
	if source == nil {
		errorResponse(c, http.StatusNotFound, "source_not_found", "source not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, source, nil)
}

func (s *Server) providers(c *gin.Context) {
	summaries, err := s.store.ProviderSummaries(c.Request.Context(), s.providerInventory)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "provider_summary_failed", "failed to load provider summaries", map[string]any{"error": err.Error()})
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "provider_summary_failed", "failed to load service metadata", map[string]any{"error": err.Error()})
		return
	}
	listResponse(c, http.StatusOK, summaries, meta, models.Pagination{Limit: len(summaries), Offset: 0, Total: len(summaries)})
}

func (s *Server) providerRuns(c *gin.Context) {
	runs, err := s.store.ProviderRuns(c.Request.Context(), c.Param("provider"), parseLimit(c.Query("limit"), 20))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "provider_runs_failed", "failed to load provider runs", map[string]any{"error": err.Error()})
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "provider_runs_failed", "failed to load service metadata", map[string]any{"error": err.Error()})
		return
	}
	listResponse(c, http.StatusOK, runs, meta, models.Pagination{Limit: len(runs), Offset: 0, Total: len(runs)})
}

func (s *Server) providerErrors(c *gin.Context) {
	failures, err := s.store.ProviderErrors(c.Request.Context(), c.Param("provider"), parseLimit(c.Query("limit"), 20))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "provider_errors_failed", "failed to load provider errors", map[string]any{"error": err.Error()})
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "provider_errors_failed", "failed to load service metadata", map[string]any{"error": err.Error()})
		return
	}
	listResponse(c, http.StatusOK, failures, meta, models.Pagination{Limit: len(failures), Offset: 0, Total: len(failures)})
}

func (s *Server) jobs(c *gin.Context) {
	jobs, err := s.store.ListJobs(c.Request.Context(), parseLimit(c.Query("limit"), 20))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "jobs_failed", "failed to load ingestion jobs", map[string]any{"error": err.Error()})
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "jobs_failed", "failed to load service metadata", map[string]any{"error": err.Error()})
		return
	}
	listResponse(c, http.StatusOK, jobs, meta, models.Pagination{Limit: len(jobs), Offset: 0, Total: len(jobs)})
}

func (s *Server) jobByID(c *gin.Context) {
	job, err := s.store.JobByID(c.Request.Context(), c.Param("job_id"))
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "job_lookup_failed", "failed to load ingestion job", map[string]any{"error": err.Error()})
		return
	}
	if job == nil {
		errorResponse(c, http.StatusNotFound, "job_not_found", "job not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, job, nil)
}

func (s *Server) search(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		errorResponse(c, http.StatusBadRequest, "missing_query", "q is required", nil)
		return
	}
	response, err := s.store.Search(c.Request.Context(), query)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "search_failed", "failed to search catalog", map[string]any{"error": err.Error()})
		return
	}
	dataResponse(c, http.StatusOK, response.Data, response.Meta)
}

func (s *Server) stats(c *gin.Context) {
	stats, err := s.store.Stats(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "stats_failed", "failed to load stats", map[string]any{"error": err.Error()})
		return
	}
	dataResponse(c, http.StatusOK, stats, nil)
}

func (s *Server) integrity(c *gin.Context) {
	report, err := s.store.IntegrityReport(c.Request.Context())
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "integrity_failed", "failed to run catalog integrity checks", map[string]any{"error": err.Error()})
		return
	}
	dataResponse(c, http.StatusOK, report, nil)
}

func (s *Server) lyrics(c *gin.Context) {
	artist := strings.TrimSpace(c.Query("artist"))
	track := strings.TrimSpace(c.Query("track"))
	if artist == "" || track == "" {
		errorResponse(c, http.StatusBadRequest, "missing_lyrics_params", "artist and track are required", nil)
		return
	}
	requestedProvider := strings.ToLower(strings.TrimSpace(c.DefaultQuery("provider", "auto")))
	for _, providerName := range lyricsProviderOrder(requestedProvider) {
		result, cached, refreshedAt, expiresAt, err := s.fetchLyrics(c.Request.Context(), providerName, artist, track)
		if err != nil {
			if requestedProvider != "auto" {
				errorResponse(c, http.StatusBadGateway, "provider_request_failed", "failed to fetch lyrics", map[string]any{"provider": providerName, "error": err.Error()})
				return
			}
			continue
		}
		dataResponse(c, http.StatusOK, result, gin.H{
			"provider":     result.Provider,
			"cached":       cached,
			"refreshed_at": refreshedAt,
			"expires_at":   expiresAt,
		})
		return
	}
	errorResponse(c, http.StatusBadGateway, "provider_request_failed", "failed to fetch lyrics from available providers", nil)
}

func (s *Server) fetchLyrics(ctx context.Context, providerName, artist, track string) (*providers.LyricsResult, bool, string, string, error) {
	cacheKey := search.StableHash(strings.ToLower(strings.TrimSpace(artist)), strings.ToLower(strings.TrimSpace(track)))
	payload, refreshedAt, expiresAt, ok, err := s.store.GetProviderCache(ctx, providerName, "lyrics", cacheKey)
	if err != nil {
		return nil, false, "", "", err
	}
	if ok {
		var result providers.LyricsResult
		if err := json.Unmarshal([]byte(payload), &result); err == nil {
			return &result, true, refreshedAt, expiresAt, nil
		}
	}
	var result *providers.LyricsResult
	switch providerName {
	case "lrclib":
		result, err = s.lrclib.Lyrics(ctx, artist, track)
	case "lyricsovh":
		result, err = s.lyricsOVH.Lyrics(ctx, artist, track)
	default:
		return nil, false, "", "", nil
	}
	if err != nil {
		return nil, false, "", "", err
	}
	body, _ := json.Marshal(result)
	if err := s.store.SetProviderCache(ctx, providerName, "lyrics", cacheKey, string(body), 24*time.Hour); err != nil {
		return nil, false, "", "", err
	}
	return result, false, "", "", nil
}

func lyricsProviderOrder(requested string) []string {
	switch requested {
	case "lrclib", "lyricsovh":
		return []string{requested}
	default:
		return []string{"lrclib", "lyricsovh"}
	}
}

func parseInt(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func parseLimit(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	if parsed > 100 {
		return 100
	}
	return parsed
}

func (s *Server) corsMiddleware() gin.HandlerFunc {
	allowedOrigin := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(getenv("ALLOW_ORIGIN", "*")), "/"))
	if allowedOrigin == "" {
		allowedOrigin = "*"
	}
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
