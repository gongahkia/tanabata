package api

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func (s *Server) structuredLogger() gin.HandlerFunc {
	logger := s.logger
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()
		logger.Info(
			"http_request",
			"request_id", c.GetString("request_id"),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"route", c.FullPath(),
			"status", c.Writer.Status(),
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"remote_addr", c.ClientIP(),
		)
	}
}

func (s *Server) recoveryMiddleware() gin.HandlerFunc {
	logger := s.logger
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logger.Error("request panic", "request_id", c.GetString("request_id"), "error", recovered)
		errorResponse(c, http.StatusInternalServerError, "internal_error", "unexpected server error", nil)
	})
}
