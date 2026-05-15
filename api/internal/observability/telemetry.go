package observability

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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
}

func New(serviceName string) (*Telemetry, error) {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
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
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.2))),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
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
		[]string{"provider", "operation", "status"},
	)
	registry.MustRegister(httpRequests, httpDuration, httpInFlight, providerRequests, providerDuration)

	return &Telemetry{
		tracer:           otel.Tracer(serviceName),
		tracerProvider:   provider,
		registry:         registry,
		httpRequests:     httpRequests,
		httpDuration:     httpDuration,
		httpInFlight:     httpInFlight,
		providerRequests: providerRequests,
		providerDuration: providerDuration,
	}, nil
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
	t.providerDuration.WithLabelValues(provider, operation, status).Observe(duration.Seconds())
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
