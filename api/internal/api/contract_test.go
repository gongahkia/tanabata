package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/providers"
)

func TestContractValidationEnabledByDefault(t *testing.T) {
	t.Setenv(contractValidationEnv, "")
	t.Setenv(contractSpecPathEnv, "")

	server, store := seededServer(t)
	defer store.Close()
	if server.contractValidator == nil {
		t.Fatalf("contract validator is nil")
	}
}

func TestNewServerFailsWhenOpenAPISpecMissing(t *testing.T) {
	t.Setenv(contractValidationEnv, "")
	t.Setenv(contractSpecPathEnv, filepath.Join(t.TempDir(), "missing-openapi.json"))

	store := newContractTestStore(t)
	defer store.Close()
	if _, err := NewServer(store, nil); err == nil {
		t.Fatalf("NewServer() error = nil, want missing spec error")
	}
}

func TestNewServerAllowsMissingOpenAPISpecWhenOff(t *testing.T) {
	t.Setenv(contractValidationEnv, "off")
	t.Setenv(contractSpecPathEnv, filepath.Join(t.TempDir(), "missing-openapi.json"))

	store := newContractTestStore(t)
	defer store.Close()
	server, err := NewServer(store, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if server.contractValidator != nil {
		t.Fatalf("contract validator = %#v, want nil", server.contractValidator)
	}
}

func TestRuntimeContractValidationRejectsInvalidRequest(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	specPath := filepath.Join("..", "..", "..", "openapi", "openapi.json")
	if err := server.enableContractValidation(specPath); err != nil {
		t.Fatalf("enableContractValidation() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/quotes?limit=not-a-number", nil)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", recorder.Code, recorder.Body.String())
	}
	var problem models.ProblemDetails
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatalf("Unmarshal() error = %v body=%s", err, recorder.Body.String())
	}
	if problem.Code != "contract_request_invalid" {
		t.Fatalf("error = %#v, want contract_request_invalid", problem)
	}
}

func TestRuntimeContractValidationAllowsValidResponse(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	specPath := filepath.Join("..", "..", "..", "openapi", "openapi.json")
	if err := server.enableContractValidation(specPath); err != nil {
		t.Fatalf("enableContractValidation() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/quotes?limit=2", nil)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("X-OpenAPI-Contract-Error"); got != "" {
		t.Fatalf("X-OpenAPI-Contract-Error = %q, want empty", got)
	}
}

func TestRuntimeContractValidationAllows400Response(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/search", nil)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("X-OpenAPI-Contract-Error"); got != "" {
		t.Fatalf("X-OpenAPI-Contract-Error = %q, want empty", got)
	}
}

func TestRuntimeContractValidationAllows500Response(t *testing.T) {
	server, store := seededServer(t)
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/stats", nil)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("X-OpenAPI-Contract-Error"); got != "" {
		t.Fatalf("X-OpenAPI-Contract-Error = %q, want empty", got)
	}
}

func TestOpenAPINegativeResponsesValidate(t *testing.T) {
	validator := newOpenAPIContractValidator(t)
	tests := []struct {
		name      string
		method    string
		path      string
		body      func() *bytes.Reader
		status    int
		configure func(t *testing.T, server *Server, store *catalog.Store) func()
	}{
		{name: "missing query", method: http.MethodGet, path: "/v1/search", status: http.StatusBadRequest},
		{name: "unknown enum", method: http.MethodGet, path: "/v1/quotes?provenance_status=invalid", status: http.StatusBadRequest},
		{name: "unknown cursor", method: http.MethodGet, path: "/v1/timeline?cursor=missing", status: http.StatusBadRequest},
		{name: "unknown path id", method: http.MethodGet, path: "/v1/artists/missing", status: http.StatusNotFound},
		{
			name:   "oversized body",
			method: http.MethodPost,
			path:   "/v1/webhooks",
			body: func() *bytes.Reader {
				return bytes.NewReader(bytes.Repeat([]byte("x"), int(maxRequestBodyBytes)+1))
			},
			status: http.StatusRequestEntityTooLarge,
		},
		{
			name:   "provider failure",
			method: http.MethodGet,
			path:   "/v1/lyrics?artist=Coldplay&track=Yellow&provider=lrclib",
			status: http.StatusBadGateway,
			configure: func(_ *testing.T, server *Server, _ *catalog.Store) func() {
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					http.Error(w, "broken", http.StatusBadGateway)
				}))
				server.lrclib.SetHTTPClient(providers.NewHTTPClient(upstream.URL))
				return upstream.Close
			},
		},
		{
			name:   "integrity store failure",
			method: http.MethodGet,
			path:   "/v1/integrity",
			status: http.StatusInternalServerError,
			configure: func(t *testing.T, _ *Server, store *catalog.Store) func() {
				if err := store.Close(); err != nil {
					t.Fatalf("Close() error = %v", err)
				}
				return func() {}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, store := seededServer(t)
			defer store.Close()
			cleanup := func() {}
			if tc.configure != nil {
				cleanup = tc.configure(t, server, store)
			}
			defer cleanup()

			var body *bytes.Reader
			if tc.body != nil {
				body = tc.body()
			} else {
				body = bytes.NewReader(nil)
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, body)
			if tc.method == http.MethodPost {
				request.Header.Set("Content-Type", "application/json")
			}
			server.Router().ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d body=%s", recorder.Code, tc.status, recorder.Body.String())
			}
			validator.validateErrorResponse(t, request, recorder)
		})
	}
}

func newContractTestStore(t *testing.T) *catalog.Store {
	t.Helper()
	store, err := catalog.Open(filepath.Join(t.TempDir(), "catalog.sqlite"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("ping store: %v", err)
	}
	return store
}
