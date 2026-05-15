package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	minInterval time.Duration
	lastRequest time.Time
	mu          sync.Mutex
	telemetry   *observability.Telemetry
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL:   strings.TrimRight(baseURL, "/"),
		userAgent: defaultUserAgent,
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
		provider: "external",
		attempts: 3,
		backoff:  200 * time.Millisecond,
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

func (c *HTTPClient) JSON(ctx context.Context, path string, query url.Values, headers map[string]string, target any) error {
	body, err := c.do(ctx, path, query, headers)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
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

		res, err := c.client.Do(req)
		if err != nil {
			lastErr = err
			done(err)
			if attempt == c.attempts {
				break
			}
			time.Sleep(c.backoff * time.Duration(attempt))
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 512<<10))
		_ = res.Body.Close()
		if readErr != nil {
			lastErr = readErr
			done(readErr)
			if attempt == c.attempts {
				break
			}
			time.Sleep(c.backoff * time.Duration(attempt))
			continue
		}
		if res.StatusCode >= 500 || res.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("provider request failed: %s %s", res.Status, strings.TrimSpace(string(body)))
			done(lastErr)
			if attempt == c.attempts {
				break
			}
			time.Sleep(c.backoff * time.Duration(attempt))
			continue
		}
		if res.StatusCode >= 400 {
			err := fmt.Errorf("provider request failed: %s %s", res.Status, strings.TrimSpace(string(body)))
			done(err)
			return nil, err
		}
		done(nil)
		return body, nil
	}
	return nil, lastErr
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
