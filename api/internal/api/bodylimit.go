package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const maxRequestBodyBytes int64 = 64 << 10

func requestBodyLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil || c.Request.Body == http.NoBody {
			c.Next()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
		if c.Request.ContentLength > maxRequestBodyBytes {
			errorResponse(c, http.StatusRequestEntityTooLarge, "payload_too_large", "request body too large", map[string]any{"limit_bytes": maxRequestBodyBytes})
			c.Abort()
			return
		}
		c.Next()
	}
}
