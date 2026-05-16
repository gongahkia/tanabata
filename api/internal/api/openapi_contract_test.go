package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

func TestOpenAPIContractRuntimeResponses(t *testing.T) {
	server, store := seededServer(t)
	defer store.Close()

	ctx := context.Background()
	if err := store.RecordProviderRun(ctx, catalog.ProviderRun{
		RunID:      "contract-run-1",
		Provider:   "wikiquote",
		Status:     "success",
		StartedAt:  time.Now().UTC().Add(-time.Minute),
		FinishedAt: time.Now().UTC(),
		Details:    "quotes=1",
	}); err != nil {
		t.Fatalf("RecordProviderRun() error = %v", err)
	}
	if err := store.RecordJob(ctx, models.JobRun{
		JobID:      "contract-job-1",
		Name:       "contract-refresh",
		Scope:      "bootstrap",
		Status:     "succeeded",
		StartedAt:  time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Details:    "bootstrap,succeeded",
	}); err != nil {
		t.Fatalf("RecordJob() error = %v", err)
	}
	if err := store.RecordJobItem(ctx, models.JobItem{
		JobItemID:  "contract-item-1",
		JobID:      "contract-job-1",
		Provider:   "tanabata_curated",
		Target:     "bootstrap:data/curated_quotes.json",
		Status:     "succeeded",
		StartedAt:  time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Details:    "imported=4 curated quotes",
	}); err != nil {
		t.Fatalf("RecordJobItem() error = %v", err)
	}

	quotes, err := store.ListQuotes(ctx, models.QuoteFilters{Limit: 10})
	if err != nil || len(quotes.Data) == 0 {
		t.Fatalf("ListQuotes() err=%v count=%d", err, len(quotes.Data))
	}
	artistID := quotes.Data[0].ArtistID
	quoteID := quotes.Data[0].QuoteID
	sourceID := quotes.Data[0].SourceID

	validator := newOpenAPIContractValidator(t)
	tests := []struct {
		name string
		path string
	}{
		{name: "list artists", path: "/v1/artists"},
		{name: "artist detail", path: "/v1/artists/" + artistID},
		{name: "artist quotes", path: "/v1/artists/" + artistID + "/quotes?limit=5"},
		{name: "quote list", path: "/v1/quotes?limit=5"},
		{name: "quote detail", path: "/v1/quotes/" + quoteID},
		{name: "quote provenance", path: "/v1/quotes/" + quoteID + "/provenance"},
		{name: "source detail", path: "/v1/sources/" + sourceID},
		{name: "providers", path: "/v1/providers"},
		{name: "provider runs", path: "/v1/providers/wikiquote/runs?limit=5"},
		{name: "jobs", path: "/v1/jobs?limit=5"},
		{name: "job detail", path: "/v1/jobs/contract-job-1"},
		{name: "review queue", path: "/v1/review/queue?limit=5"},
		{name: "stale quote review", path: "/v1/review/stale?limit=5"},
		{name: "search", path: "/v1/search?q=frank"},
		{name: "stats", path: "/v1/stats"},
		{name: "integrity", path: "/v1/integrity"},
		{name: "lyrics", path: "/v1/lyrics?artist=Coldplay&track=Yellow&provider=lrclib"},
	}

	if err := store.SetProviderCache(ctx, "lrclib", "lyrics", search.StableHash("coldplay", "yellow"), `{"provider":"lrclib","artist":"Coldplay","track":"Yellow","lyrics":"Look at the stars","source_url":"https://lrclib.net"}`, time.Hour); err != nil {
		t.Fatalf("SetProviderCache() error = %v", err)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			server.Router().ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s status = %d, want 200 body=%s", tc.path, recorder.Code, recorder.Body.String())
			}
			validator.validateResponse(t, request, recorder)
		})
	}
}

type openAPIContractValidator struct {
	t      *testing.T
	router routers.Router
}

func newOpenAPIContractValidator(t *testing.T) *openAPIContractValidator {
	t.Helper()

	specPath := filepath.Join("..", "..", "..", "openapi", "openapi.json")
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("LoadFromFile(%s) error = %v", specPath, err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("Validate(%s) error = %v", specPath, err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	return &openAPIContractValidator{t: t, router: router}
}

func (v *openAPIContractValidator) validateResponse(t *testing.T, request *http.Request, recorder *httptest.ResponseRecorder) {
	t.Helper()

	contractRequest, err := http.NewRequestWithContext(
		request.Context(),
		request.Method,
		"http://localhost:8080"+request.URL.RequestURI(),
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	route, pathParams, err := v.router.FindRoute(contractRequest)
	if err != nil {
		t.Fatalf("FindRoute(%s) error = %v", request.URL.RequestURI(), err)
	}
	requestInput := &openapi3filter.RequestValidationInput{
		Request:    contractRequest,
		PathParams: pathParams,
		Route:      route,
	}
	if err := openapi3filter.ValidateRequest(context.Background(), requestInput); err != nil {
		t.Fatalf("ValidateRequest(%s) error = %v", request.URL.RequestURI(), err)
	}
	responseInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestInput,
		Status:                 recorder.Code,
		Header:                 recorder.Header(),
	}
	responseInput.SetBodyBytes(recorder.Body.Bytes())
	if err := openapi3filter.ValidateResponse(context.Background(), responseInput); err != nil {
		t.Fatalf("ValidateResponse(%s) error = %v body=%s", request.URL.RequestURI(), err, recorder.Body.String())
	}
}
