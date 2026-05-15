package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gongahkia/tanabata/api/internal/models"
)

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
