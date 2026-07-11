package observability

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type Telemetry struct {
	tracer           trace.Tracer
	tracerProvider   *sdktrace.TracerProvider
	registry         *prometheus.Registry
	httpRequests     *prometheus.CounterVec
	httpDuration     *prometheus.HistogramVec
	httpInFlight     prometheus.Gauge
	providerRequests *prometheus.CounterVec
	providerDuration *prometheus.HistogramVec
	providerErrors   *prometheus.CounterVec
	ingestDuration   *prometheus.HistogramVec
	claimTransitions *prometheus.CounterVec
	catalogRows      *prometheus.GaugeVec
}

func New(serviceName string) (*Telemetry, error) {
	exporter, exporterName, err := traceExporter(context.Background())
	if err != nil {
		return nil, err
	}
	if configured := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")); configured != "" {
		serviceName = configured
	}
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			attribute.String("deployment.environment", os.Getenv("APP_ENV")),
		),
	)
	if err != nil {
		return nil, err
	}
	options := []sdktrace.TracerProviderOption{sdktrace.WithSampler(configuredSampler()), sdktrace.WithResource(res)}
	if exporter != nil {
		options = append(options, sdktrace.WithBatcher(exporter))
	}
	provider := sdktrace.NewTracerProvider(options...)
	slog.Info("telemetry_exporter_resolved", "exporter", exporterName)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	registry := prometheus.NewRegistry()
	httpRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tanabata_http_requests_total",
			Help: "Total HTTP requests handled by the API.",
		},
		[]string{"method", "route", "status"},
	)
	httpDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tanabata_http_request_duration_seconds",
			Help:    "Duration of HTTP requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route", "status"},
	)
	httpInFlight := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "tanabata_http_in_flight_requests",
			Help: "Current in-flight HTTP requests.",
		},
	)
	providerRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tanabata_provider_requests_total",
			Help: "Total upstream provider requests.",
		},
		[]string{"provider", "operation", "status"},
	)
	providerDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tanabata_provider_request_duration_seconds",
			Help:    "Duration of upstream provider requests.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"provider", "outcome"},
	)
	providerErrors := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tanabata_provider_error_total", Help: "Total upstream provider errors."}, []string{"provider", "kind"})
	ingestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "tanabata_ingest_job_duration_seconds", Help: "Duration of ingestion jobs.", Buckets: prometheus.DefBuckets}, []string{"status"})
	claimTransitions := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tanabata_claim_status_transition_total", Help: "Claim status transitions."}, []string{"from", "to", "kind"})
	catalogRows := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "tanabata_catalog_row_count", Help: "Current catalog row count by table."}, []string{"table"})
	providerDuration.WithLabelValues("unknown", "unknown").Observe(0)
	providerErrors.WithLabelValues("unknown", "unknown").Add(0)
	for _, status := range []string{"succeeded", "failed", "partial"} {
		ingestDuration.WithLabelValues(status).Observe(0)
	}
	claimTransitions.WithLabelValues("unknown", "unknown", "unknown").Add(0)
	for _, table := range []string{"artists", "quotes", "claims", "samples", "works", "recordings", "performances"} {
		catalogRows.WithLabelValues(table).Set(0)
	}
	registry.MustRegister(httpRequests, httpDuration, httpInFlight, providerRequests, providerDuration, providerErrors, ingestDuration, claimTransitions, catalogRows)

	return &Telemetry{
		tracer:           otel.Tracer(serviceName),
		tracerProvider:   provider,
		registry:         registry,
		httpRequests:     httpRequests,
		httpDuration:     httpDuration,
		httpInFlight:     httpInFlight,
		providerRequests: providerRequests,
		providerDuration: providerDuration,
		providerErrors:   providerErrors,
		ingestDuration:   ingestDuration,
		claimTransitions: claimTransitions,
		catalogRows:      catalogRows,
	}, nil
}

func traceExporter(ctx context.Context) (sdktrace.SpanExporter, string, error) {
	if strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) != "" {
		protocol := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")))
		if protocol == "grpc" {
			exporter, err := otlptracegrpc.New(ctx)
			return exporter, "otlp_grpc", err
		}
		exporter, err := otlptracehttp.New(ctx)
		return exporter, "otlp_http", err
	}
	if os.Getenv("TANABATA_TELEMETRY_DEV") == "1" {
		exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		return exporter, "stdout", err
	}
	return nil, "noop", nil
}

func configuredSampler() sdktrace.Sampler {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER")))
	if name == "" {
		name = "parentbased_traceidratio"
	}
	ratio := 0.1
	if value := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG")); value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || parsed < 0 || parsed > 1 {
			slog.Warn("otel_sampler_arg_invalid", "value", value, "error", fmt.Sprint(err))
		} else {
			ratio = parsed
		}
	}
	switch name {
	case "always_on":
		return sdktrace.AlwaysSample()
	case "always_off":
		return sdktrace.NeverSample()
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(ratio)
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	}
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t == nil || t.tracerProvider == nil {
		return nil
	}
	return t.tracerProvider.Shutdown(ctx)
}

func (t *Telemetry) StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if t == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

func (t *Telemetry) ObserveProviderCall(provider, operation, status string, duration time.Duration) {
	if t == nil {
		return
	}
	t.providerRequests.WithLabelValues(provider, operation, status).Inc()
	t.providerDuration.WithLabelValues(provider, status).Observe(duration.Seconds())
}

func (t *Telemetry) ObserveProviderError(provider, kind string) {
	if t != nil {
		t.providerErrors.WithLabelValues(provider, kind).Inc()
	}
}
func (t *Telemetry) ObserveIngestJob(status string, duration time.Duration) {
	if t != nil {
		t.ingestDuration.WithLabelValues(status).Observe(duration.Seconds())
	}
}
func (t *Telemetry) ObserveClaimStatusTransition(from, to, kind string) {
	if t != nil {
		t.claimTransitions.WithLabelValues(from, to, kind).Inc()
	}
}
func (t *Telemetry) SetCatalogRowCount(table string, count float64) {
	if t != nil {
		t.catalogRows.WithLabelValues(table).Set(count)
	}
}

func (t *Telemetry) MetricsHandler() gin.HandlerFunc {
	handler := promhttp.HandlerFor(t.registry, promhttp.HandlerOpts{})
	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

func (t *Telemetry) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		t.httpInFlight.Inc()
		startedAt := time.Now()
		ctx, span := t.StartSpan(
			c.Request.Context(),
			route,
			semconv.HTTPRequestMethodKey.String(c.Request.Method),
			semconv.URLPath(route),
		)
		defer span.End()
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		statusCode := http.StatusText(c.Writer.Status())
		if statusCode == "" {
			statusCode = "UNKNOWN"
		}
		t.httpRequests.WithLabelValues(c.Request.Method, route, statusCode).Inc()
		t.httpDuration.WithLabelValues(c.Request.Method, route, statusCode).Observe(time.Since(startedAt).Seconds())
		t.httpInFlight.Dec()
		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
			attribute.String("request.id", c.GetString("request_id")),
		)
	}
}
