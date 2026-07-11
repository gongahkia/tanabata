package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/gongahkia/tanabata/api/internal/models"
)

func dataResponse[T any](c *gin.Context, status int, data T, meta any) {
	c.JSON(status, models.APIResponse[T]{
		Data: data,
		Meta: meta,
	})
}

func listResponse[T any](c *gin.Context, status int, data []T, meta any, pagination models.Pagination) {
	if data == nil {
		data = []T{}
	}
	paginationCopy := pagination
	c.JSON(status, struct {
		Data       []T                `json:"data"`
		Meta       any                `json:"meta,omitempty"`
		Pagination *models.Pagination `json:"pagination,omitempty"`
	}{
		Data:       data,
		Meta:       meta,
		Pagination: &paginationCopy,
	})
}

func errorResponse(c *gin.Context, status int, code, message string, details map[string]any) {
	c.Header("Content-Type", "application/problem+json")
	c.JSON(status, models.ProblemDetails{
		Type:     "https://tanabata.dev/errors/" + code,
		Title:    message,
		Status:   status,
		Detail:   problemDetail(message, details),
		Instance: c.GetString("request_id"),
		Code:     code,
		Details:  details,
	})
}

func problemDetail(fallback string, details map[string]any) string {
	if details == nil {
		return fallback
	}
	if detail, ok := details["message"].(string); ok && detail != "" {
		return detail
	}
	return fallback
}

func (s *Server) loggedErrorResponse(c *gin.Context, status int, code, message string, details map[string]any, err error) {
	s.logHandlerError(c, status, code, err)
	errorResponse(c, status, code, message, details)
}

func (s *Server) logHandlerError(c *gin.Context, status int, code string, err error) {
	if err == nil {
		return
	}
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Error(
		"handler_error",
		"request_id", c.GetString("request_id"),
		"path", c.Request.URL.Path,
		"status", status,
		"code", code,
		"err", err,
	)
}
