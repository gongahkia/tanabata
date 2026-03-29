package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/providers"
)

type Server struct {
	store     *catalog.Store
	setlistFM *providers.SetlistFMProvider
	lrclib    *providers.LRCLIBProvider
	lyricsOVH *providers.LyricsOVHProvider
}

func NewServer(store *catalog.Store) *Server {
	return &Server{
		store:     store,
		setlistFM: providers.NewSetlistFMProvider(),
		lrclib:    providers.NewLRCLIBProvider(),
		lyricsOVH: providers.NewLyricsOVHProvider(),
	}
}

func (s *Server) Router() *gin.Engine {
	router := gin.Default()
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

		v1.GET("/sources/:source_id", s.sourceByID)
		v1.GET("/search", s.search)
		v1.GET("/stats", s.stats)
		v1.GET("/lyrics", s.lyrics)
	}

	return router
}

func (s *Server) health(c *gin.Context) {
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":           "ok",
		"snapshot_version": meta.SnapshotVersion,
		"active_providers": meta.ActiveProviders,
	})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) artistByID(c *gin.Context) {
	artist, err := s.store.ArtistByID(c.Request.Context(), c.Param("artist_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if artist == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Artist not found"})
		return
	}
	c.JSON(http.StatusOK, artist)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) artistRelated(c *gin.Context) {
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	related, err := s.store.RelatedArtists(c.Request.Context(), c.Param("artist_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": related,
		"pagination": models.Pagination{
			Limit:  len(related),
			Offset: 0,
			Total:  len(related),
		},
		"meta": meta,
	})
}

func (s *Server) artistReleases(c *gin.Context) {
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	releases, err := s.store.Releases(c.Request.Context(), c.Param("artist_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": releases,
		"pagination": models.Pagination{
			Limit:  len(releases),
			Offset: 0,
			Total:  len(releases),
		},
		"meta": meta,
	})
}

func (s *Server) artistSetlists(c *gin.Context) {
	if !s.setlistFM.Enabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "setlist.fm is disabled; configure SETLISTFM_API_KEY"})
		return
	}
	artist, err := s.store.ArtistByID(c.Request.Context(), c.Param("artist_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if artist == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Artist not found"})
		return
	}
	if artist.MBID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Artist does not have a MusicBrainz ID"})
		return
	}
	setlists, err := s.setlistFM.ArtistSetlists(c.Request.Context(), artist.MBID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": setlists,
		"pagination": models.Pagination{
			Limit:  len(setlists),
			Offset: 0,
			Total:  len(setlists),
		},
		"meta": meta,
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if quote == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Quote not found"})
		return
	}
	c.JSON(http.StatusOK, quote)
}

func (s *Server) quoteByID(c *gin.Context) {
	quote, err := s.store.QuoteByID(c.Request.Context(), c.Param("quote_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if quote == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Quote not found"})
		return
	}
	c.JSON(http.StatusOK, quote)
}

func (s *Server) sourceByID(c *gin.Context) {
	source, err := s.store.SourceByID(c.Request.Context(), c.Param("source_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if source == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Source not found"})
		return
	}
	c.JSON(http.StatusOK, source)
}

func (s *Server) search(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}
	response, err := s.store.Search(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) stats(c *gin.Context) {
	stats, err := s.store.Stats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (s *Server) lyrics(c *gin.Context) {
	artist := strings.TrimSpace(c.Query("artist"))
	track := strings.TrimSpace(c.Query("track"))
	if artist == "" || track == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "artist and track are required"})
		return
	}
	providerName := strings.ToLower(strings.TrimSpace(c.DefaultQuery("provider", "auto")))
	switch providerName {
	case "lrclib":
		result, err := s.lrclib.Lyrics(c.Request.Context(), artist, track)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	case "lyricsovh":
		result, err := s.lyricsOVH.Lyrics(c.Request.Context(), artist, track)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	default:
		result, err := s.lrclib.Lyrics(c.Request.Context(), artist, track)
		if err == nil && result != nil && result.Lyrics != "" {
			c.JSON(http.StatusOK, result)
			return
		}
		result, err = s.lyricsOVH.Lyrics(c.Request.Context(), artist, track)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func parseInt(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}
