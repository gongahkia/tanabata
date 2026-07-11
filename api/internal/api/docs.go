package api

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed docs/index.html
var renderedOpenAPIDocs []byte

func (s *Server) docs(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", renderedOpenAPIDocs)
}
