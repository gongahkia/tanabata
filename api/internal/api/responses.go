package api

import (
	"net/http"

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
	paginationCopy := pagination
	c.JSON(status, models.APIResponse[[]T]{
		Data:       data,
		Meta:       meta,
		Pagination: &paginationCopy,
	})
}

func errorResponse(c *gin.Context, status int, code, message string, details map[string]any) {
	c.JSON(status, models.APIResponse[any]{
		Error: &models.APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func ok(c *gin.Context) {
	c.Status(http.StatusOK)
}
