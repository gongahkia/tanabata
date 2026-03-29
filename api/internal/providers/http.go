package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultUserAgent = "tanabata/2.0 (+https://github.com/gongahkia/tanabata)"

type HTTPClient struct {
	baseURL   string
	userAgent string
	client    *http.Client
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL:   strings.TrimRight(baseURL, "/"),
		userAgent: defaultUserAgent,
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *HTTPClient) JSON(ctx context.Context, path string, query url.Values, headers map[string]string, target any) error {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.userAgent)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
		return fmt.Errorf("provider request failed: %s %s", res.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func (c *HTTPClient) Text(ctx context.Context, path string, query url.Values, headers map[string]string) (string, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.userAgent)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
		return "", fmt.Errorf("provider request failed: %s %s", res.Status, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
