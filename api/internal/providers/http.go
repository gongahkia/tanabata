package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/gongahkia/tanabata/api/internal/observability"
)

const defaultUserAgent = "tanabata/2.0 (+https://github.com/gongahkia/tanabata)"

type HTTPClient struct {
	baseURL     string
	userAgent   string
	client      *http.Client
	provider    string
	attempts    int
	backoff     time.Duration
	retryBudget int
	minInterval time.Duration
	lastRequest time.Time
	mu          sync.Mutex
	sem         chan struct{}
	telemetry   *observability.Telemetry
}

type FailureKind string

const (
	FailureTimeout     FailureKind = "timeout"
	FailureRateLimit   FailureKind = "rate_limit"
	FailureParseError  FailureKind = "parse_error"
	FailureNotFound    FailureKind = "not_found"
	FailureBadUpstream FailureKind = "bad_upstream"
	FailureNetwork     FailureKind = "network"
)

type ProviderFailure struct {
	Kind       FailureKind
	StatusCode int
	Message    string
	Err        error
}

func (e *ProviderFailure) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Kind)
}

func (e *ProviderFailure) Unwrap() error {
	return e.Err
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL:   strings.TrimRight(baseURL, "/"),
		userAgent: defaultUserAgent,
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
		provider:    "external",
		attempts:    3,
		backoff:     200 * time.Millisecond,
		retryBudget: 2,
		sem:         make(chan struct{}, 2),
	}
}

func (c *HTTPClient) ConfigureProvider(name string, telemetry *observability.Telemetry) *HTTPClient {
	c.provider = name
	c.telemetry = telemetry
	switch name {
	case "musicbrainz":
		c.minInterval = 1100 * time.Millisecond
		c.client.Timeout = 15 * time.Second
	case "wikidata", "wikiquote":
		c.minInterval = 350 * time.Millisecond
		c.client.Timeout = 15 * time.Second
	case "setlistfm":
		c.minInterval = 500 * time.Millisecond
		c.client.Timeout = 12 * time.Second
	default:
		c.minInterval = 200 * time.Millisecond
		c.client.Timeout = 10 * time.Second
	}
	return c
}

func (c *HTTPClient) SetRetryBudget(attempts int, backoff time.Duration) *HTTPClient {
	if attempts < 1 {
		attempts = 1
	}
	c.attempts = attempts
	c.retryBudget = attempts - 1
	if backoff > 0 {
		c.backoff = backoff
	}
	return c
}

func (c *HTTPClient) SetMaxConcurrent(maxConcurrent int) *HTTPClient {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	c.sem = make(chan struct{}, maxConcurrent)
	return c
}

func (c *HTTPClient) JSON(ctx context.Context, path string, query url.Values, headers map[string]string, target any) error {
	body, err := c.do(ctx, path, query, headers)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, target); err != nil {
		return &ProviderFailure{Kind: FailureParseError, Message: "provider parse error: " + err.Error(), Err: err}
	}
	return nil
}

func (c *HTTPClient) Text(ctx context.Context, path string, query url.Values, headers map[string]string) (string, error) {
	body, err := c.do(ctx, path, query, headers)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *HTTPClient) do(ctx context.Context, path string, query url.Values, headers map[string]string) ([]byte, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	var lastErr error
	for attempt := 1; attempt <= c.attempts; attempt++ {
		if attempt > 1 && attempt-1 > c.retryBudget {
			break
		}
		if err := c.waitTurn(); err != nil {
			return nil, err
		}
		startedAt := time.Now()
		spanCtx := ctx
		var span trace.Span
		var done func(error)
		if c.telemetry != nil {
			spanCtx, span = c.telemetry.StartSpan(
				ctx,
				"provider."+c.provider,
				attribute.String("provider.name", c.provider),
				attribute.String("provider.path", path),
				attribute.Int("provider.attempt", attempt),
			)
			done = func(err error) {
				status := "success"
				if err != nil {
					status = "error"
				}
				c.telemetry.ObserveProviderCall(c.provider, path, status, time.Since(startedAt))
				span.End()
			}
		} else {
			done = func(error) {}
		}

		req, err := http.NewRequestWithContext(spanCtx, http.MethodGet, endpoint, nil)
		if err != nil {
			done(err)
			return nil, err
		}
		req.Header.Set("User-Agent", c.userAgent)
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		release, err := c.acquireSlot(spanCtx)
		if err != nil {
			done(err)
			return nil, err
		}
		res, err := c.client.Do(req)
		release()
		if err != nil {
			lastErr = classifyTransportFailure(err)
			done(lastErr)
			if attempt == c.attempts {
				break
			}
			time.Sleep(c.backoff * time.Duration(attempt))
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 512<<10))
		_ = res.Body.Close()
		if readErr != nil {
			lastErr = &ProviderFailure{Kind: FailureNetwork, Message: "provider response read failed: " + readErr.Error(), Err: readErr}
			done(lastErr)
			if attempt == c.attempts {
				break
			}
			time.Sleep(c.backoff * time.Duration(attempt))
			continue
		}
		if res.StatusCode >= 500 || res.StatusCode == http.StatusTooManyRequests {
			lastErr = failureFromStatus(res.StatusCode, res.Status, body)
			done(lastErr)
			if attempt == c.attempts {
				break
			}
			time.Sleep(c.backoff * time.Duration(attempt))
			continue
		}
		if res.StatusCode >= 400 {
			err := failureFromStatus(res.StatusCode, res.Status, body)
			done(err)
			return nil, err
		}
		done(nil)
		return body, nil
	}
	return nil, lastErr
}

func (c *HTTPClient) acquireSlot(ctx context.Context) (func(), error) {
	if c.sem == nil {
		return func() {}, nil
	}
	select {
	case c.sem <- struct{}{}:
		return func() { <-c.sem }, nil
	case <-ctx.Done():
		return func() {}, classifyTransportFailure(ctx.Err())
	}
}

func (c *HTTPClient) waitTurn() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.minInterval <= 0 {
		return nil
	}
	wait := c.minInterval - time.Since(c.lastRequest)
	if wait > 0 {
		time.Sleep(wait)
	}
	c.lastRequest = time.Now()
	return nil
}

func failureFromStatus(statusCode int, status string, body []byte) *ProviderFailure {
	kind := FailureBadUpstream
	switch statusCode {
	case http.StatusTooManyRequests:
		kind = FailureRateLimit
	case http.StatusNotFound:
		kind = FailureNotFound
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = status
	}
	return &ProviderFailure{
		Kind:       kind,
		StatusCode: statusCode,
		Message:    fmt.Sprintf("provider request failed: %s %s", status, message),
	}
}

func classifyTransportFailure(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &ProviderFailure{Kind: FailureTimeout, Message: err.Error(), Err: err}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &ProviderFailure{Kind: FailureTimeout, Message: err.Error(), Err: err}
	}
	return &ProviderFailure{Kind: FailureNetwork, Message: err.Error(), Err: err}
}

func ClassifyFailure(err error) string {
	var failure *ProviderFailure
	if errors.As(err, &failure) {
		return string(failure.Kind)
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return string(FailureTimeout)
	}
	return string(FailureBadUpstream)
}
