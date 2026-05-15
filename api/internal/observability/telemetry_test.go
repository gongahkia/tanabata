package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTelemetryMiddlewareAndMetricsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	telemetry, err := New("tanabata-test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer telemetry.Shutdown(context.Background())

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("request_id", "req-123")
		c.Next()
	})
	router.Use(telemetry.Middleware())
	router.GET("/hello", func(c *gin.Context) {
		telemetry.ObserveProviderCall("wikiquote", "/hello", "success", 0)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	router.GET("/metrics", telemetry.MetricsHandler())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/hello", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	metricsRecorder := httptest.NewRecorder()
	metricsRequest := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	router.ServeHTTP(metricsRecorder, metricsRequest)
	if metricsRecorder.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want 200", metricsRecorder.Code)
	}
	body := metricsRecorder.Body.String()
	if !strings.Contains(body, "tanabata_http_requests_total") || !strings.Contains(body, "tanabata_provider_requests_total") {
		t.Fatalf("expected metrics output, got %s", body)
	}
}
