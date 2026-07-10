package api

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const (
	rateLimitRPMEnv   = "TANABATA_RATE_LIMIT_RPM"
	rateLimitBurstEnv = "TANABATA_RATE_LIMIT_BURST"
	defaultRateRPM    = 60
	defaultRateBurst  = 20
)

type rateLimitConfig struct {
	rpm   int
	burst int
}

func (c rateLimitConfig) enabled() bool {
	return c.rpm > 0 && c.burst > 0
}

func (c rateLimitConfig) retryAfterSeconds() int {
	seconds := (60 + c.rpm - 1) / c.rpm
	if seconds < 1 {
		return 1
	}
	return seconds
}

func loadRateLimitConfig() rateLimitConfig {
	return rateLimitConfig{
		rpm:   parseRateLimitEnv(rateLimitRPMEnv, defaultRateRPM),
		burst: parseRateLimitEnv(rateLimitBurstEnv, defaultRateBurst),
	}
}

func parseRateLimitEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func (s *Server) rateLimitMiddleware() gin.HandlerFunc {
	config := loadRateLimitConfig()
	if !config.enabled() {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	limit := rate.Limit(float64(config.rpm) / 60.0)
	var clients sync.Map
	return func(c *gin.Context) {
		if rateLimitExemptPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		clientIP := rateLimitClientIP(c)
		value, _ := clients.LoadOrStore(clientIP, rate.NewLimiter(limit, config.burst))
		limiter := value.(*rate.Limiter)
		if limiter.Allow() {
			c.Next()
			return
		}
		retryAfter := strconv.Itoa(config.retryAfterSeconds())
		c.Header("Retry-After", retryAfter)
		logger.Warn(
			"rate_limited",
			"request_id", c.GetString("request_id"),
			"remote_addr", clientIP,
			"path", c.Request.URL.Path,
			"retry_after", retryAfter,
		)
		errorResponse(c, http.StatusTooManyRequests, "rate_limited", "too many requests", nil)
		c.Abort()
	}
}

func rateLimitExemptPath(path string) bool {
	switch path {
	case "/livez", "/readyz", "/metrics":
		return true
	default:
		return false
	}
}

func rateLimitClientIP(c *gin.Context) string {
	if clientIP := c.ClientIP(); clientIP != "" {
		return clientIP
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return c.Request.RemoteAddr
}
