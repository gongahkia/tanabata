package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gongahkia/tanabata/api/internal/models"
)

// listWorks GET /v1/works
func (s *Server) listWorks(c *gin.Context) {
	response, err := s.store.ListWorks(c.Request.Context(), models.WorkFilters{
		Query:    c.Query("q"),
		ArtistID: c.Query("artist_id"),
		Limit:    parseInt(c.Query("limit")),
		Offset:   parseInt(c.Query("offset")),
	})
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "work_list_failed", "failed to list works", nil, err)
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

// workByID GET /v1/works/{work_id}
func (s *Server) workByID(c *gin.Context) {
	work, err := s.store.WorkByID(c.Request.Context(), c.Param("work_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "work_lookup_failed", "failed to load work", nil, err)
		return
	}
	if work == nil {
		errorResponse(c, http.StatusNotFound, "work_not_found", "work not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, work, nil)
}

// workRecordings GET /v1/works/{work_id}/recordings
// Returns every recording of this work — the cover-lineage feature.
func (s *Server) workRecordings(c *gin.Context) {
	recordings, err := s.store.WorkRecordings(c.Request.Context(), c.Param("work_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "work_recordings_failed", "failed to load recordings", nil, err)
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "work_recordings_failed", "failed to load service metadata", nil, err)
		return
	}
	listResponse(c, http.StatusOK, recordings, meta, models.Pagination{Limit: len(recordings), Offset: 0, Total: len(recordings)})
}

// workCredits GET /v1/works/{work_id}/credits
func (s *Server) workCredits(c *gin.Context) {
	credits, err := s.store.WorkCredits(c.Request.Context(), c.Param("work_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "work_credits_failed", "failed to load credits", nil, err)
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "work_credits_failed", "failed to load service metadata", nil, err)
		return
	}
	listResponse(c, http.StatusOK, credits, meta, models.Pagination{Limit: len(credits), Offset: 0, Total: len(credits)})
}

// workPerformances GET /v1/works/{work_id}/performances
func (s *Server) workPerformances(c *gin.Context) {
	sortOrder, apiErr := parseEnum("sort", c.Query("sort"), performanceSorts)
	if apiErr != nil {
		apiErr.write(c)
		return
	}
	response, err := s.store.ListPerformances(c.Request.Context(), models.PerformanceFilters{
		WorkID: c.Param("work_id"),
		Year:   c.Query("year"),
		Sort:   sortOrder,
		Limit:  parseInt(c.Query("limit")),
		Offset: parseInt(c.Query("offset")),
	})
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "work_performances_failed", "failed to load performances", nil, err)
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

// listRecordings GET /v1/recordings
func (s *Server) listRecordings(c *gin.Context) {
	response, err := s.store.ListRecordings(c.Request.Context(), models.RecordingFilters{
		ArtistID: c.Query("artist_id"),
		WorkID:   c.Query("work_id"),
		Query:    c.Query("q"),
		Limit:    parseInt(c.Query("limit")),
		Offset:   parseInt(c.Query("offset")),
	})
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "recording_list_failed", "failed to list recordings", nil, err)
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

// recordingByID GET /v1/recordings/{recording_id}
func (s *Server) recordingByID(c *gin.Context) {
	recording, err := s.store.RecordingByID(c.Request.Context(), c.Param("recording_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "recording_lookup_failed", "failed to load recording", nil, err)
		return
	}
	if recording == nil {
		errorResponse(c, http.StatusNotFound, "recording_not_found", "recording not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, recording, nil)
}

// recordingOutgoingSamples GET /v1/recordings/{recording_id}/samples
// Returns every recording that this recording samples (its ancestors).
func (s *Server) recordingOutgoingSamples(c *gin.Context) {
	edges, err := s.store.OutgoingSamples(c.Request.Context(), c.Param("recording_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "recording_samples_failed", "failed to load samples", nil, err)
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "recording_samples_failed", "failed to load service metadata", nil, err)
		return
	}
	listResponse(c, http.StatusOK, edges, meta, models.Pagination{Limit: len(edges), Offset: 0, Total: len(edges)})
}

// recordingIncomingSamples GET /v1/recordings/{recording_id}/sampled_by
// Returns every recording that sampled this one (its descendants).
func (s *Server) recordingIncomingSamples(c *gin.Context) {
	edges, err := s.store.IncomingSamples(c.Request.Context(), c.Param("recording_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "recording_sampled_by_failed", "failed to load samples", nil, err)
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "recording_sampled_by_failed", "failed to load service metadata", nil, err)
		return
	}
	listResponse(c, http.StatusOK, edges, meta, models.Pagination{Limit: len(edges), Offset: 0, Total: len(edges)})
}

// sampleByID GET /v1/samples/{sample_id}
func (s *Server) sampleByID(c *gin.Context) {
	edge, err := s.store.SampleEdgeByID(c.Request.Context(), c.Param("sample_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "sample_lookup_failed", "failed to load sample", nil, err)
		return
	}
	if edge == nil {
		errorResponse(c, http.StatusNotFound, "sample_not_found", "sample not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, edge, nil)
}

// artistRecordings GET /v1/artists/{artist_id}/recordings
func (s *Server) artistRecordings(c *gin.Context) {
	response, err := s.store.ListRecordings(c.Request.Context(), models.RecordingFilters{
		ArtistID: c.Param("artist_id"),
		Limit:    parseInt(c.Query("limit")),
		Offset:   parseInt(c.Query("offset")),
	})
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "artist_recordings_failed", "failed to load recordings", nil, err)
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

// artistPerformances GET /v1/artists/{artist_id}/performances
func (s *Server) artistPerformances(c *gin.Context) {
	sortOrder, apiErr := parseEnum("sort", c.Query("sort"), performanceSorts)
	if apiErr != nil {
		apiErr.write(c)
		return
	}
	response, err := s.store.ListPerformances(c.Request.Context(), models.PerformanceFilters{
		ArtistID: c.Param("artist_id"),
		WorkID:   c.Query("work_id"),
		Year:     c.Query("year"),
		Sort:     sortOrder,
		Limit:    parseInt(c.Query("limit")),
		Offset:   parseInt(c.Query("offset")),
	})
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "artist_performances_failed", "failed to load performances", nil, err)
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

// artistPerformanceStats GET /v1/artists/{artist_id}/performances/stats
func (s *Server) artistPerformanceStats(c *gin.Context) {
	stats, err := s.store.PerformanceStats(c.Request.Context(), c.Param("artist_id"), strings.TrimSpace(c.Query("work_id")))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "artist_performance_stats_failed", "failed to compute performance stats", nil, err)
		return
	}
	dataResponse(c, http.StatusOK, stats, nil)
}

// performanceByID GET /v1/performances/{performance_id}
func (s *Server) performanceByID(c *gin.Context) {
	perf, err := s.store.PerformanceByID(c.Request.Context(), c.Param("performance_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "performance_lookup_failed", "failed to load performance", nil, err)
		return
	}
	if perf == nil {
		errorResponse(c, http.StatusNotFound, "performance_not_found", "performance not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, perf, nil)
}

// claimByID GET /v1/claims/{claim_id}
func (s *Server) claimByID(c *gin.Context) {
	claim, err := s.store.ClaimByID(c.Request.Context(), c.Param("claim_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "claim_lookup_failed", "failed to load claim", nil, err)
		return
	}
	if claim == nil {
		errorResponse(c, http.StatusNotFound, "claim_not_found", "claim not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, claim, nil)
}

// listClaims GET /v1/claims?kind=&status=
func (s *Server) listClaims(c *gin.Context) {
	response, err := s.store.ListClaims(c.Request.Context(), models.ClaimFilters{
		Kind:   c.Query("kind"),
		Status: c.Query("status"),
		Limit:  parseInt(c.Query("limit")),
		Offset: parseInt(c.Query("offset")),
	})
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "claim_list_failed", "failed to list claims", nil, err)
		return
	}
	listResponse(c, http.StatusOK, response.Data, response.Meta, response.Pagination)
}

// disputes GET /v1/disputes — feed of contested claims across every kind.
func (s *Server) disputes(c *gin.Context) {
	disputes, err := s.store.Disputes(c.Request.Context(), parseLimit(c.Query("limit"), 20))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "disputes_failed", "failed to load disputes", nil, err)
		return
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "disputes_failed", "failed to load service metadata", nil, err)
		return
	}
	listResponse(c, http.StatusOK, disputes, meta, models.Pagination{Limit: len(disputes), Offset: 0, Total: len(disputes)})
}

// quoteLineage GET /v1/quotes/{quote_id}/lineage — chronological supporting + refuting trail.
func (s *Server) quoteLineage(c *gin.Context) {
	lineage, err := s.store.QuoteLineage(c.Request.Context(), c.Param("quote_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "quote_lineage_failed", "failed to load lineage", nil, err)
		return
	}
	if lineage == nil {
		errorResponse(c, http.StatusNotFound, "quote_not_found", "quote not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, lineage, nil)
}
