package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
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
	var response models.APIResponse[any]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v body=%s", err, recorder.Body.String())
	}
	if response.Error == nil || response.Error.Code != "contract_request_invalid" {
		t.Fatalf("error = %#v, want contract_request_invalid", response.Error)
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
