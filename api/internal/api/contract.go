package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/gin-gonic/gin"
)

const (
	contractValidationEnv = "TANABATA_CONTRACT_VALIDATION"
	contractSpecPathEnv   = "TANABATA_OPENAPI_SPEC"
)

type runtimeContractValidator struct {
	router routers.Router
	logger *slog.Logger
}

func newRuntimeContractValidator(specPath string, logger *slog.Logger) (*runtimeContractValidator, error) {
	if specPath == "" {
		specPath = defaultOpenAPISpecPath()
	}
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("load OpenAPI spec: %w", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validate OpenAPI spec: %w", err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("build OpenAPI router: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &runtimeContractValidator{router: router, logger: logger}, nil
}

func defaultOpenAPISpecPath() string {
	if specPath := os.Getenv(contractSpecPathEnv); specPath != "" {
		return specPath
	}
	candidates := []string{
		filepath.Join("..", "openapi", "openapi.json"),
		filepath.Join("openapi", "openapi.json"),
		filepath.Join("..", "..", "openapi", "openapi.json"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

func (s *Server) enableContractValidation(specPath string) error {
	validator, err := newRuntimeContractValidator(specPath, s.logger)
	if err != nil {
		return err
	}
	s.contractValidator = validator
	return nil
}

func (v *runtimeContractValidator) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/v1/") && c.Request.URL.Path != "/v1" {
			c.Next()
			return
		}

		route, pathParams, err := v.routeFor(c.Request)
		if err != nil {
			v.logger.Warn("openapi_contract_route_miss", "path", c.Request.URL.Path, "error", err)
			c.Next()
			return
		}
		requestInput := &openapi3filter.RequestValidationInput{
			Request:    cloneRequestForValidation(c.Request),
			PathParams: pathParams,
			Route:      route,
		}
		if err := openapi3filter.ValidateRequest(c.Request.Context(), requestInput); err != nil {
			errorResponse(c, http.StatusBadRequest, "contract_request_invalid", "request does not match the OpenAPI contract", map[string]any{"error": err.Error()})
			c.Abort()
			return
		}

		recorder := &contractBodyRecorder{ResponseWriter: c.Writer}
		c.Writer = recorder
		c.Next()

		status := recorder.Status()
		if status < 200 || status >= 300 || len(recorder.body.Bytes()) == 0 {
			return
		}
		responseInput := &openapi3filter.ResponseValidationInput{
			RequestValidationInput: requestInput,
			Status:                 status,
			Header:                 recorder.Header(),
		}
		responseInput.SetBodyBytes(recorder.body.Bytes())
		if err := openapi3filter.ValidateResponse(c.Request.Context(), responseInput); err != nil {
			v.logger.Error(
				"openapi_contract_response_invalid",
				"request_id", c.GetString("request_id"),
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"status", status,
				"error", err,
			)
			recorder.Header().Set("X-OpenAPI-Contract-Error", "response_invalid")
		}
	}
}

func (v *runtimeContractValidator) routeFor(request *http.Request) (*routers.Route, map[string]string, error) {
	validationRequest := cloneRequestForValidation(request)
	route, pathParams, err := v.router.FindRoute(validationRequest)
	if err != nil {
		return nil, nil, err
	}
	return route, pathParams, nil
}

func cloneRequestForValidation(request *http.Request) *http.Request {
	body := request.Body
	if body == nil {
		body = io.NopCloser(bytes.NewReader(nil))
	}
	clone := request.Clone(request.Context())
	clone.Body = body
	if clone.URL.Scheme == "" {
		clone.URL.Scheme = "http"
	}
	if clone.URL.Host == "" {
		clone.URL.Host = "localhost:8080"
	}
	return clone
}

type contractBodyRecorder struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *contractBodyRecorder) Write(data []byte) (int, error) {
	if len(data) > 0 {
		_, _ = w.body.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func (w *contractBodyRecorder) WriteString(data string) (int, error) {
	if data != "" {
		_, _ = w.body.WriteString(data)
	}
	return w.ResponseWriter.WriteString(data)
}

func contractValidationEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(contractValidationEnv)))
	return value == "1" || value == "true" || value == "yes"
}
