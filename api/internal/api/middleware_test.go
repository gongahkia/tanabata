package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/observability"
)

func TestRequestIDGeneratedWhenMissing(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/livez", nil)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("X-Request-ID"); strings.TrimSpace(got) == "" {
		t.Fatalf("expected generated X-Request-ID header")
	}
}

func TestCORSMiddlewareOptionsAndOrigin(t *testing.T) {
	t.Setenv("ALLOW_ORIGIN", "https://tanabata.dev")

	server, store := seededServer(t)
	defer store.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/v1/quotes", nil)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://tanabata.dev" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodOptions) {
		t.Fatalf("Access-Control-Allow-Methods = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "X-Request-ID") {
		t.Fatalf("Access-Control-Allow-Headers = %q", got)
	}
}

func TestMiddlewareChainOrder(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	chain := server.middlewareChain()
	got := make([]string, 0, len(chain))
	for _, middleware := range chain {
		got = append(got, middleware.name)
	}
	want := []string{"request-id", "ratelimit", "cors", "logger", "recovery"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("middleware chain = %v, want %v", got, want)
	}
}

func TestRateLimitReturns429WithRetryAfter(t *testing.T) {
	t.Setenv(rateLimitRPMEnv, "1")
	t.Setenv(rateLimitBurstEnv, "2")

	server, store := seededServer(t)
	defer store.Close()
	router := server.Router()

	for i := range 3 {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/search?q=frank", nil)
		router.ServeHTTP(recorder, request)
		if i < 2 && recorder.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 body=%s", i+1, recorder.Code, recorder.Body.String())
		}
		if i == 2 {
			if recorder.Code != http.StatusTooManyRequests {
				t.Fatalf("request %d status = %d, want 429 body=%s", i+1, recorder.Code, recorder.Body.String())
			}
			if got := recorder.Header().Get("Retry-After"); got == "" {
				t.Fatalf("Retry-After header missing")
			}
			var response models.APIResponse[any]
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Error == nil || response.Error.Code != "rate_limited" {
				t.Fatalf("error = %#v, want rate_limited", response.Error)
			}
		}
	}
}

func TestRateLimitExemptPathsUnderSaturation(t *testing.T) {
	t.Setenv(rateLimitRPMEnv, "1")
	t.Setenv(rateLimitBurstEnv, "1")

	server, store := seededServer(t)
	defer store.Close()
	telemetry, err := observability.New("tanabata-test")
	if err != nil {
		t.Fatalf("telemetry: %v", err)
	}
	defer telemetry.Shutdown(context.Background())
	server.telemetry = telemetry
	router := server.Router()

	for range 2 {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/search?q=frank", nil)
		router.ServeHTTP(recorder, request)
	}

	for _, path := range []string{"/livez", "/readyz", "/metrics"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200 body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestRateLimitZeroDisablesMiddleware(t *testing.T) {
	t.Setenv(rateLimitRPMEnv, "0")
	t.Setenv(rateLimitBurstEnv, "1")

	server, store := seededServer(t)
	defer store.Close()
	router := server.Router()

	for i := range 3 {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/search?q=frank", nil)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 body=%s", i+1, recorder.Code, recorder.Body.String())
		}
	}
}

func TestStructuredLoggerEmitsStableRequestFields(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	var logs bytes.Buffer
	server.logger = slog.New(slog.NewJSONHandler(&logs, nil))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search?q=frank", nil)
	request.Header.Set("X-Request-ID", "log-test-request")
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &payload); err != nil {
		t.Fatalf("decode log payload: %v body=%s", err, logs.String())
	}
	assertLogField(t, payload, "msg", "http_request")
	assertLogField(t, payload, "request_id", "log-test-request")
	assertLogField(t, payload, "method", http.MethodGet)
	assertLogField(t, payload, "path", "/v1/search")
	assertLogField(t, payload, "route", "/v1/search")
	if payload["status"].(float64) != http.StatusOK {
		t.Fatalf("status field = %v, want %d", payload["status"], http.StatusOK)
	}
	if _, ok := payload["duration_ms"].(float64); !ok {
		t.Fatalf("duration_ms field = %T, want number", payload["duration_ms"])
	}
}

func assertLogField(t *testing.T, payload map[string]any, key, want string) {
	t.Helper()
	if got := payload[key]; got != want {
		t.Fatalf("%s = %v, want %q", key, got, want)
	}
}

func TestRecoveryMiddlewareReturnsStructuredError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server, store := seededServer(t)
	defer store.Close()

	router := gin.New()
	router.Use(requestIDMiddleware())
	router.Use(server.rateLimitMiddleware())
	router.Use(server.corsMiddleware())
	router.Use(server.structuredLogger())
	router.Use(server.recoveryMiddleware())
	router.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if got := recorder.Header().Get("X-Request-ID"); strings.TrimSpace(got) == "" {
		t.Fatalf("expected generated request ID on recovered panic")
	}
	var response models.APIResponse[any]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "internal_error" {
		t.Fatalf("unexpected error payload %+v", response.Error)
	}
}

func TestEmptyAndNotFoundStates(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	tests := []struct {
		path       string
		statusCode int
		want       string
	}{
		{path: "/v1/quotes?artist_id=missing&limit=5", statusCode: http.StatusOK, want: `"data":[]`},
		{path: "/v1/artists/missing", statusCode: http.StatusNotFound, want: `"code":"artist_not_found"`},
		{path: "/v1/quotes/missing", statusCode: http.StatusNotFound, want: `"code":"quote_not_found"`},
		{path: "/v1/jobs/missing", statusCode: http.StatusNotFound, want: `"code":"job_not_found"`},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			server.Router().ServeHTTP(recorder, request)
			if recorder.Code != tc.statusCode {
				t.Fatalf("status = %d, want %d body=%s", recorder.Code, tc.statusCode, recorder.Body.String())
			}
			if body := recorder.Body.String(); !strings.Contains(body, tc.want) {
				t.Fatalf("expected %q in %s", tc.want, body)
			}
		})
	}
}
