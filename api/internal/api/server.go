package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gongahkia/tanabata/api/internal/buildinfo"
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

type apiError struct {
	status  int
	code    string
	message string
	details map[string]any
}

func (e *apiError) write(c *gin.Context) {
	errorResponse(c, e.status, e.code, e.message, e.details)
}

var (
	quoteProvenanceStatuses       = []string{"verified", "source_attributed", "provider_attributed", "ambiguous", "needs_review"}
	reviewQueueProvenanceStatuses = []string{"provider_attributed", "ambiguous", "needs_review"}
	freshnessStatuses             = []string{"fresh", "aging", "stale", "unknown"}
	quoteSorts                    = []string{"random"}
	performanceSorts              = []string{"asc", "desc"}
)

func NewServer(store *catalog.Store, telemetry *observability.Telemetry) (*Server, error) {
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
			return nil, err
		}
	}
	return server, nil
}

func (s *Server) Router() *gin.Engine {
	router := gin.New()
	for _, middleware := range s.middlewareChain() {
		router.Use(middleware.handler)
	}
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
		v1.GET("/artists/:artist_id/recordings", s.artistRecordings)
		v1.GET("/artists/:artist_id/performances", s.artistPerformances)
		v1.GET("/artists/:artist_id/performances/stats", s.artistPerformanceStats)

		v1.GET("/quotes", s.listQuotes)
		v1.GET("/quotes/random", s.randomQuote)
		v1.GET("/quotes/:quote_id", s.quoteByID)
		v1.GET("/quotes/:quote_id/provenance", s.quoteProvenance)
		v1.GET("/quotes/:quote_id/lineage", s.quoteLineage)

		v1.GET("/works", s.listWorks)
		v1.GET("/works/:work_id", s.workByID)
		v1.GET("/works/:work_id/recordings", s.workRecordings)
		v1.GET("/works/:work_id/credits", s.workCredits)
		v1.GET("/works/:work_id/performances", s.workPerformances)

		v1.GET("/recordings", s.listRecordings)
		v1.GET("/recordings/:recording_id", s.recordingByID)
		v1.GET("/recordings/:recording_id/samples", s.recordingOutgoingSamples)
		v1.GET("/recordings/:recording_id/sampled_by", s.recordingIncomingSamples)
		v1.GET("/samples/:sample_id", s.sampleByID)

		v1.GET("/performances/:performance_id", s.performanceByID)

		v1.GET("/claims", s.listClaims)
		v1.GET("/claims/:claim_id", s.claimByID)
		v1.GET("/disputes", s.disputes)

		v1.GET("/sources/:source_id", s.sourceByID)
		v1.GET("/providers", s.providers)
		v1.GET("/providers/:provider/runs", s.providerRuns)
		v1.GET("/providers/:provider/errors", s.providerErrors)
		v1.GET("/jobs", s.jobs)
		v1.GET("/jobs/:job_id", s.jobByID)
		v1.GET("/jobs/:job_id/snapshots", s.jobSnapshots)
		v1.GET("/jobs/:job_id/audit", s.jobAuditEvents)
		v1.GET("/timeline", s.timeline)
		v1.GET("/review/queue", s.reviewQueue)
		v1.GET("/review/stale", s.staleQuotes)
		v1.GET("/search", s.search)
		v1.GET("/stats", s.stats)
		v1.GET("/integrity", s.integrity)
		v1.GET("/lyrics", s.lyrics)
		v1.GET("/version", s.version)
	}

	return router
}

func (s *Server) livez(c *gin.Context) {
	dataResponse(c, http.StatusOK, gin.H{"status": "alive"}, nil)
}

func (s *Server) readyz(c *gin.Context) {
	if err := s.store.Ping(c.Request.Context()); err != nil {
		s.logHandlerError(c, http.StatusServiceUnavailable, "readiness_failed", err)
		dataResponse(c, http.StatusServiceUnavailable, gin.H{"status": "not_ready", "checks": gin.H{"db": "unavailable"}}, nil)
		return
	}
	dataResponse(c, http.StatusOK, gin.H{"status": "ready"}, nil)
}

func (s *Server) health(c *gin.Context) {
	stats, err := s.store.Stats(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "health_failed", "failed to load service metadata", nil, err)
		return
	}
	dataResponse(c, http.StatusOK, gin.H{"status": "ok"}, stats)
}

func (s *Server) version(c *gin.Context) {
	c.JSON(http.StatusOK, struct {
		Data buildinfo.Metadata `json:"data"`
		Meta any                `json:"meta"`
	}{
		Data: buildinfo.Current(),
		Meta: nil,
	})
}

func (s *Server) legacyQuotes(c *gin.Context) {
	quotes, err := s.store.LegacyQuotes(c.Request.Context(), "")
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "legacy_quotes_failed", "failed to list legacy quotes", nil, err)
		return
	}
	c.JSON(http.StatusOK, quotes)
}

func (s *Server) legacyRandomQuote(c *gin.Context) {
	quote, err := s.store.RandomLegacyQuote(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "legacy_random_quote_failed", "failed to select legacy quote", nil, err)
		return
	}
	if quote == nil {
		errorResponse(c, http.StatusNotFound, "quote_not_found", "quote not found", nil)
		return
	}
	c.JSON(http.StatusOK, quote)
}

func (s *Server) legacyAuthorQuotes(c *gin.Context) {
	quotes, err := s.store.LegacyQuotes(c.Request.Context(), c.Param("author"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "legacy_author_quotes_failed", "failed to list legacy author quotes", nil, err)
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
		s.loggedErrorResponse(c, http.StatusInternalServerError, "artist_list_failed", "failed to list artists", nil, err)
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

func (s *Server) artistByID(c *gin.Context) {
	artist, err := s.store.ArtistByID(c.Request.Context(), c.Param("artist_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "artist_lookup_failed", "failed to load artist", nil, err)
		return
	}
	if artist == nil {
		errorResponse(c, http.StatusNotFound, "artist_not_found", "artist not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, artist, nil)
}

func (s *Server) artistQuotes(c *gin.Context) {
	provenanceStatus, apiErr := parseEnum("provenance_status", c.Query("provenance_status"), quoteProvenanceStatuses)
	if apiErr != nil {
		apiErr.write(c)
		return
	}
	freshnessStatus, apiErr := parseEnum("freshness_status", c.Query("freshness_status"), freshnessStatuses)
	if apiErr != nil {
		apiErr.write(c)
		return
	}
	sortOrder, apiErr := parseEnum("sort", c.Query("sort"), quoteSorts)
	if apiErr != nil {
		apiErr.write(c)
		return
	}
	response, err := s.store.ArtistQuotes(c.Request.Context(), c.Param("artist_id"), models.QuoteFilters{
		Query:            c.Query("q"),
		Tag:              c.Query("tag"),
		Source:           c.Query("source"),
		ProvenanceStatus: provenanceStatus,
		FreshnessStatus:  freshnessStatus,
		Limit:            parseInt(c.Query("limit")),
		Offset:           parseInt(c.Query("offset")),
		Sort:             sortOrder,
	})
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "artist_quotes_failed", "failed to list artist quotes", nil, err)
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

func (s *Server) artistRelated(c *gin.Context) {
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "artist_related_failed", "failed to load service metadata", nil, err)
		return
	}
	related, err := s.store.RelatedArtists(c.Request.Context(), c.Param("artist_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "artist_related_failed", "failed to load related artists", nil, err)
		return
	}
	listResponse(c, http.StatusOK, related, meta, models.Pagination{Limit: len(related), Offset: 0, Total: len(related)})
}

func (s *Server) artistReleases(c *gin.Context) {
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "artist_releases_failed", "failed to load service metadata", nil, err)
		return
	}
	releases, err := s.store.Releases(c.Request.Context(), c.Param("artist_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "artist_releases_failed", "failed to load releases", nil, err)
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
		s.loggedErrorResponse(c, http.StatusInternalServerError, "artist_lookup_failed", "failed to load artist", nil, err)
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
		s.loggedErrorResponse(c, http.StatusInternalServerError, "provider_cache_failed", "failed to access provider cache", nil, err)
		return
	}

	setlists, err := s.setlistFM.ArtistSetlists(c.Request.Context(), artist.MBID)
	if err != nil {
		s.loggedErrorResponse(c, http.StatusBadGateway, "provider_request_failed", "failed to fetch setlists", map[string]any{"provider": "setlistfm"}, err)
		return
	}
	body, _ := json.Marshal(setlists)
	if err := s.store.SetProviderCache(c.Request.Context(), "setlistfm", "setlists", cacheKey, string(body), 6*time.Hour); err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "provider_cache_failed", "failed to cache setlists", nil, err)
		return
	}
	dataResponse(c, http.StatusOK, setlists, gin.H{
		"provider": "setlistfm",
		"cached":   false,
	})
}

func (s *Server) listQuotes(c *gin.Context) {
	provenanceStatus, apiErr := parseEnum("provenance_status", c.Query("provenance_status"), quoteProvenanceStatuses)
	if apiErr != nil {
		apiErr.write(c)
		return
	}
	freshnessStatus, apiErr := parseEnum("freshness_status", c.Query("freshness_status"), freshnessStatuses)
	if apiErr != nil {
		apiErr.write(c)
		return
	}
	sortOrder, apiErr := parseEnum("sort", c.Query("sort"), quoteSorts)
	if apiErr != nil {
		apiErr.write(c)
		return
	}
	filters := models.QuoteFilters{
		Artist:           c.Query("artist"),
		ArtistID:         c.Query("artist_id"),
		Query:            c.Query("q"),
		Tag:              c.Query("tag"),
		Source:           c.Query("source"),
		ProvenanceStatus: provenanceStatus,
		FreshnessStatus:  freshnessStatus,
		Limit:            parseInt(c.Query("limit")),
		Offset:           parseInt(c.Query("offset")),
		Sort:             sortOrder,
	}
	if filters.Artist != "" && filters.ArtistID == "" {
		if resolved, err := s.store.ResolveArtistID(c.Request.Context(), filters.Artist); err == nil && resolved != "" {
			filters.ArtistID = resolved
			filters.Artist = ""
		}
	}
	response, err := s.store.ListQuotes(c.Request.Context(), filters)
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "quote_list_failed", "failed to list quotes", nil, err)
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

func (s *Server) randomQuote(c *gin.Context) {
	provenanceStatus, apiErr := parseEnum("provenance_status", c.Query("provenance_status"), quoteProvenanceStatuses)
	if apiErr != nil {
		apiErr.write(c)
		return
	}
	filters := models.QuoteFilters{
		Artist:           c.Query("artist"),
		ArtistID:         c.Query("artist_id"),
		Query:            c.Query("q"),
		Tag:              c.Query("tag"),
		Source:           c.Query("source"),
		ProvenanceStatus: provenanceStatus,
	}
	if filters.Artist != "" && filters.ArtistID == "" {
		if resolved, err := s.store.ResolveArtistID(c.Request.Context(), filters.Artist); err == nil && resolved != "" {
			filters.ArtistID = resolved
			filters.Artist = ""
		}
	}
	quote, err := s.store.RandomQuote(c.Request.Context(), filters)
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "quote_random_failed", "failed to select quote", nil, err)
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
		s.loggedErrorResponse(c, http.StatusInternalServerError, "quote_lookup_failed", "failed to load quote", nil, err)
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
		s.loggedErrorResponse(c, http.StatusInternalServerError, "quote_provenance_failed", "failed to load quote provenance", nil, err)
		return
	}
	if provenance == nil {
		errorResponse(c, http.StatusNotFound, "quote_not_found", "quote not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, provenance, nil)
}

func (s *Server) reviewQueue(c *gin.Context) {
	provenanceStatus, apiErr := parseEnum("provenance_status", c.Query("provenance_status"), reviewQueueProvenanceStatuses)
	if apiErr != nil {
		apiErr.write(c)
		return
	}
	response, err := s.store.ReviewQueue(c.Request.Context(), models.ReviewQueueFilters{
		ProvenanceStatus: provenanceStatus,
		Limit:            parseInt(c.Query("limit")),
		Offset:           parseInt(c.Query("offset")),
	})
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "review_queue_failed", "failed to load review queue", nil, err)
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
		s.loggedErrorResponse(c, http.StatusInternalServerError, "stale_quotes_failed", "failed to load stale quote review set", nil, err)
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

func (s *Server) sourceByID(c *gin.Context) {
	source, err := s.store.SourceByID(c.Request.Context(), c.Param("source_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "source_lookup_failed", "failed to load source", nil, err)
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
		s.loggedErrorResponse(c, http.StatusInternalServerError, "provider_summary_failed", "failed to load provider summaries", nil, err)
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "provider_summary_failed", "failed to load service metadata", nil, err)
		return
	}
	listResponse(c, http.StatusOK, summaries, meta, models.Pagination{Limit: len(summaries), Offset: 0, Total: len(summaries)})
}

func (s *Server) providerRuns(c *gin.Context) {
	runs, err := s.store.ProviderRuns(c.Request.Context(), c.Param("provider"), parseLimit(c.Query("limit"), 20))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "provider_runs_failed", "failed to load provider runs", nil, err)
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "provider_runs_failed", "failed to load service metadata", nil, err)
		return
	}
	listResponse(c, http.StatusOK, runs, meta, models.Pagination{Limit: len(runs), Offset: 0, Total: len(runs)})
}

func (s *Server) providerErrors(c *gin.Context) {
	failures, err := s.store.ProviderErrors(c.Request.Context(), c.Param("provider"), parseLimit(c.Query("limit"), 20))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "provider_errors_failed", "failed to load provider errors", nil, err)
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "provider_errors_failed", "failed to load service metadata", nil, err)
		return
	}
	listResponse(c, http.StatusOK, failures, meta, models.Pagination{Limit: len(failures), Offset: 0, Total: len(failures)})
}

func (s *Server) jobs(c *gin.Context) {
	jobs, err := s.store.ListJobs(c.Request.Context(), parseLimit(c.Query("limit"), 20))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "jobs_failed", "failed to load ingestion jobs", nil, err)
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "jobs_failed", "failed to load service metadata", nil, err)
		return
	}
	listResponse(c, http.StatusOK, jobs, meta, models.Pagination{Limit: len(jobs), Offset: 0, Total: len(jobs)})
}

func (s *Server) jobByID(c *gin.Context) {
	job, err := s.store.JobByID(c.Request.Context(), c.Param("job_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "job_lookup_failed", "failed to load ingestion job", nil, err)
		return
	}
	if job == nil {
		errorResponse(c, http.StatusNotFound, "job_not_found", "job not found", nil)
		return
	}
	includes := parseInclude(c.Query("include"))
	if includes["snapshots"] {
		snapshots, err := s.store.ListIngestionSnapshots(c.Request.Context(), job.JobID, parseLimit(c.Query("snapshot_limit"), 20))
		if err != nil {
			s.loggedErrorResponse(c, http.StatusInternalServerError, "job_lookup_failed", "failed to load ingestion snapshots", nil, err)
			return
		}
		job.Snapshots = snapshots
	}
	if includes["audit"] || includes["audit_events"] {
		events, err := s.store.ListIngestionAuditEvents(c.Request.Context(), job.JobID, parseLimit(c.Query("audit_limit"), 20))
		if err != nil {
			s.loggedErrorResponse(c, http.StatusInternalServerError, "job_lookup_failed", "failed to load ingestion audit events", nil, err)
			return
		}
		job.AuditEvents = events
	}
	dataResponse(c, http.StatusOK, job, nil)
}

func (s *Server) jobSnapshots(c *gin.Context) {
	snapshots, err := s.store.ListIngestionSnapshots(c.Request.Context(), c.Param("job_id"), parseLimit(c.Query("limit"), 20))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "job_snapshots_failed", "failed to load ingestion snapshots", nil, err)
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "job_snapshots_failed", "failed to load service metadata", nil, err)
		return
	}
	listResponse(c, http.StatusOK, snapshots, meta, models.Pagination{Limit: len(snapshots), Offset: 0, Total: len(snapshots)})
}

func (s *Server) jobAuditEvents(c *gin.Context) {
	events, err := s.store.ListIngestionAuditEvents(c.Request.Context(), c.Param("job_id"), parseLimit(c.Query("limit"), 20))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "job_audit_failed", "failed to load ingestion audit events", nil, err)
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "job_audit_failed", "failed to load service metadata", nil, err)
		return
	}
	listResponse(c, http.StatusOK, events, meta, models.Pagination{Limit: len(events), Offset: 0, Total: len(events)})
}

func (s *Server) timeline(c *gin.Context) {
	limit := parseLimit(c.Query("limit"), 50)
	jobs, err := s.store.ListJobs(c.Request.Context(), limit+1)
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "timeline_failed", "failed to load ingestion jobs", nil, err)
		return
	}
	providers, err := s.store.ProviderSummaries(c.Request.Context(), s.providerInventory)
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "timeline_failed", "failed to load provider summaries", nil, err)
		return
	}
	events := make([]models.TimelineEvent, 0, len(jobs)+len(providers))
	for _, job := range jobs {
		events = append(events, models.TimelineEvent{
			EventID: job.JobID,
			Kind:    "job",
			Title:   job.Name,
			Status:  job.Status,
			At:      firstNonEmpty(job.FinishedAt, job.StartedAt),
			Details: job.Details,
			Metadata: map[string]any{
				"scope":       job.Scope,
				"item_count":  len(job.Items),
				"started_at":  job.StartedAt,
				"finished_at": job.FinishedAt,
			},
		})
		for _, item := range job.Items {
			events = append(events, models.TimelineEvent{
				EventID: item.JobItemID,
				Kind:    "job_item",
				Title:   item.Provider,
				Status:  item.Status,
				At:      firstNonEmpty(item.FinishedAt, item.StartedAt),
				Details: item.Details,
				Metadata: map[string]any{
					"job_id": job.JobID,
					"target": item.Target,
				},
			})
		}
	}
	for _, provider := range providers {
		at := firstNonEmpty(provider.CooldownUntil, provider.LastErrorAt, provider.LastSuccessful)
		if at == "" {
			continue
		}
		status := firstNonEmpty(provider.LastStatus, "observed")
		if provider.CooldownUntil != "" {
			status = "cooldown"
		}
		events = append(events, models.TimelineEvent{
			EventID: "provider:" + provider.Provider,
			Kind:    "provider",
			Title:   provider.Provider,
			Status:  status,
			At:      at,
			Details: provider.CooldownReason,
			Metadata: map[string]any{
				"enabled":            provider.Enabled,
				"recent_error_count": provider.RecentErrorCount,
				"last_successful":    provider.LastSuccessful,
				"last_error_at":      provider.LastErrorAt,
			},
		})
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].At > events[j].At
	})
	events, nextCursor := paginateTimelineEvents(events, c.Query("cursor"), limit)
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "timeline_failed", "failed to load service metadata", nil, err)
		return
	}
	listResponse(c, http.StatusOK, events, cursorMeta(meta, nextCursor), models.Pagination{Limit: len(events), Offset: 0, Total: len(events)})
}

func (s *Server) search(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		errorResponse(c, http.StatusBadRequest, "missing_query", "q is required", nil)
		return
	}
	limit := parseLimit(c.Query("limit"), 10)
	response, err := s.store.SearchWithLimit(c.Request.Context(), query, limit+1)
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "search_failed", "failed to search catalog", nil, err)
		return
	}
	quotes, nextCursor := paginateQuotes(response.Data.Quotes, c.Query("cursor"), limit)
	response.Data.Quotes = quotes
	dataResponse(c, http.StatusOK, response.Data, cursorMeta(response.Meta, nextCursor))
}

func (s *Server) stats(c *gin.Context) {
	stats, err := s.store.Stats(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "stats_failed", "failed to load stats", nil, err)
		return
	}
	dataResponse(c, http.StatusOK, stats, nil)
}

func (s *Server) integrity(c *gin.Context) {
	report, err := s.store.IntegrityReport(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "integrity_failed", "failed to run catalog integrity checks", nil, err)
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
				s.loggedErrorResponse(c, http.StatusBadGateway, "provider_request_failed", "failed to fetch lyrics", map[string]any{"provider": providerName}, err)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseInclude(value string) map[string]bool {
	includes := map[string]bool{}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			includes[part] = true
		}
	}
	return includes
}

func parseEnum(name, value string, allowed []string) (string, *apiError) {
	if value == "" {
		return "", nil
	}
	for _, candidate := range allowed {
		if value == candidate {
			return value, nil
		}
	}
	return "", &apiError{
		status:  http.StatusBadRequest,
		code:    "invalid_enum",
		message: "unknown value for " + name,
		details: map[string]any{"allowed": append([]string(nil), allowed...)},
	}
}

func cursorMeta(meta models.ListMeta, nextCursor string) models.CursorMeta {
	return models.CursorMeta{
		SnapshotVersion: meta.SnapshotVersion,
		ActiveProviders: meta.ActiveProviders,
		NextCursor:      nextCursor,
	}
}

func paginateQuotes(quotes []models.Quote, cursor string, limit int) ([]models.Quote, string) {
	start := 0
	if cursor != "" {
		start = len(quotes)
		for i, quote := range quotes {
			if quote.QuoteID == cursor {
				start = i + 1
				break
			}
		}
	}
	if start > len(quotes) {
		start = len(quotes)
	}
	end := start + limit
	if end > len(quotes) {
		end = len(quotes)
	}
	nextCursor := ""
	if end < len(quotes) && end > start {
		nextCursor = quotes[end-1].QuoteID
	}
	return quotes[start:end], nextCursor
}

func paginateTimelineEvents(events []models.TimelineEvent, cursor string, limit int) ([]models.TimelineEvent, string) {
	start := 0
	if cursor != "" {
		start = len(events)
		for i, event := range events {
			if event.EventID == cursor {
				start = i + 1
				break
			}
		}
	}
	if start > len(events) {
		start = len(events)
	}
	end := start + limit
	if end > len(events) {
		end = len(events)
	}
	nextCursor := ""
	if end < len(events) && end > start {
		nextCursor = events[end-1].EventID
	}
	return events[start:end], nextCursor
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

const (
	allowOriginEnv = "ALLOW_ORIGIN"
	corsDevEnv     = "TANABATA_CORS_DEV"
)

type corsPolicy struct {
	devWildcard    bool
	allowedOrigins map[string]struct{}
}

func (s *Server) corsMiddleware() gin.HandlerFunc {
	policy := loadCORSPolicy()
	s.logCORSPolicy(policy)
	return func(c *gin.Context) {
		if allowedOrigin := policy.allowedOrigin(c.GetHeader("Origin")); allowedOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowedOrigin)
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
			if allowedOrigin != "*" {
				c.Header("Vary", "Origin")
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func loadCORSPolicy() corsPolicy {
	if corsDevMode() {
		return corsPolicy{devWildcard: true}
	}
	policy := corsPolicy{allowedOrigins: map[string]struct{}{}}
	for _, value := range strings.Split(os.Getenv(allowOriginEnv), ",") {
		origin := normalizeCORSOrigin(value)
		if origin == "" || origin == "*" {
			continue
		}
		policy.allowedOrigins[origin] = struct{}{}
	}
	return policy
}

func (p corsPolicy) allowedOrigin(origin string) string {
	if p.devWildcard {
		return "*"
	}
	normalized := normalizeCORSOrigin(origin)
	if normalized == "" {
		return ""
	}
	if _, ok := p.allowedOrigins[normalized]; ok {
		return normalized
	}
	return ""
}

func normalizeCORSOrigin(origin string) string {
	return strings.TrimRight(strings.TrimSpace(origin), "/")
}

func corsDevMode() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(corsDevEnv)))
	return value == "1" || value == "true" || value == "yes"
}

func (s *Server) logCORSPolicy(policy corsPolicy) {
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	if policy.devWildcard {
		logger.Warn("cors_policy_resolved", "mode", "wildcard", "env", corsDevEnv)
		return
	}
	if len(policy.allowedOrigins) == 0 {
		logger.Info("cors_policy_resolved", "mode", "deny")
		return
	}
	origins := make([]string, 0, len(policy.allowedOrigins))
	for origin := range policy.allowedOrigins {
		origins = append(origins, origin)
	}
	sort.Strings(origins)
	logger.Info("cors_policy_resolved", "mode", "allowlist", "origins", strings.Join(origins, ","))
}
