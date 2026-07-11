package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
)

const webhookAdminTokenEnv = "TANABATA_WEBHOOK_ADMIN_TOKEN" // #nosec G101 -- env var name, not a credential
const webhookMaxDeliveryAttempts = 5

var webhookEventKinds = []string{"claim.state_changed", "job.completed", "dispute.raised"}
var webhookDefaultBackoffs = []time.Duration{time.Second, 5 * time.Second, 30 * time.Second, 5 * time.Minute, time.Hour}

type webhookCreateRequest struct {
	URL        string   `json:"url"`
	EventKinds []string `json:"event_kinds"`
}

type webhookDispatcher struct {
	store    *catalog.Store
	client   *http.Client
	backoffs []time.Duration
	async    bool
	now      func() time.Time
}

func newWebhookDispatcher(store *catalog.Store) *webhookDispatcher {
	return &webhookDispatcher{
		store:    store,
		client:   &http.Client{Timeout: 5 * time.Second},
		backoffs: append([]time.Duration(nil), webhookDefaultBackoffs...),
		async:    true,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Server) webhookAdminAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimSpace(os.Getenv(webhookAdminTokenEnv))
		if token == "" {
			errorResponse(c, http.StatusServiceUnavailable, "webhook_admin_disabled", "webhook admin token is not configured", nil)
			c.Abort()
			return
		}
		if !constantTimeBearerToken(c.GetHeader("Authorization"), token) {
			errorResponse(c, http.StatusUnauthorized, "unauthorized", "valid bearer token required", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *Server) createWebhook(c *gin.Context) {
	var request webhookCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid_request", "invalid webhook request", nil)
		return
	}
	eventKinds, apiErr := validateWebhookRequest(request)
	if apiErr != nil {
		apiErr.write(c)
		return
	}
	subscription, err := s.store.CreateWebhook(c.Request.Context(), request.URL, eventKinds)
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "webhook_create_failed", "failed to create webhook", nil, err)
		return
	}
	dataResponse(c, http.StatusOK, subscription, nil)
}

func (s *Server) listWebhooks(c *gin.Context) {
	subscriptions, err := s.store.ListWebhooks(c.Request.Context(), true)
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "webhook_list_failed", "failed to list webhooks", nil, err)
		return
	}
	for idx := range subscriptions {
		subscriptions[idx].Secret = ""
	}
	meta, err := s.store.Meta(c.Request.Context())
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "webhook_list_failed", "failed to load service metadata", nil, err)
		return
	}
	listResponse(c, http.StatusOK, subscriptions, meta, models.Pagination{Limit: len(subscriptions), Offset: 0, Total: len(subscriptions)})
}

func (s *Server) deleteWebhook(c *gin.Context) {
	deleted, err := s.store.DeleteWebhook(c.Request.Context(), c.Param("webhook_id"))
	if err != nil {
		s.loggedErrorResponse(c, http.StatusInternalServerError, "webhook_delete_failed", "failed to delete webhook", nil, err)
		return
	}
	if !deleted {
		errorResponse(c, http.StatusNotFound, "webhook_not_found", "webhook not found", nil)
		return
	}
	dataResponse(c, http.StatusOK, gin.H{"deleted": true}, nil)
}

func validateWebhookRequest(request webhookCreateRequest) ([]string, *apiError) {
	if !validWebhookURL(request.URL) {
		return nil, &apiError{status: http.StatusBadRequest, code: "invalid_webhook_url", message: "url must be http or https"}
	}
	if len(request.EventKinds) == 0 {
		return nil, &apiError{status: http.StatusBadRequest, code: "invalid_event_kinds", message: "event_kinds is required"}
	}
	kinds := []string{}
	for _, kind := range request.EventKinds {
		kind = strings.TrimSpace(kind)
		if !allowedWebhookEventKind(kind) {
			return nil, &apiError{status: http.StatusBadRequest, code: "invalid_event_kind", message: "unsupported webhook event kind"}
		}
		if !stringInSlice(kinds, kind) {
			kinds = append(kinds, kind)
		}
	}
	return kinds, nil
}

func validWebhookURL(rawURL string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func allowedWebhookEventKind(kind string) bool {
	return stringInSlice(webhookEventKinds, kind)
}

func stringInSlice(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func constantTimeBearerToken(header, token string) bool {
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	provided := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if len(provided) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1
}

func (d *webhookDispatcher) EmitWebhookEvent(ctx context.Context, event models.WebhookEvent) {
	if d == nil || d.store == nil {
		return
	}
	event = d.normalizeEvent(event)
	if d.async {
		dispatchCtx := context.WithoutCancel(ctx)
		go func() {
			_ = d.dispatch(dispatchCtx, event)
		}()
		return
	}
	_ = d.dispatch(ctx, event)
}

func (d *webhookDispatcher) normalizeEvent(event models.WebhookEvent) models.WebhookEvent {
	now := d.now()
	if event.EventID == "" {
		event.EventID = "evt_" + hex.EncodeToString([]byte(event.Kind+"|"+now.Format(time.RFC3339Nano)))
	}
	if event.OccurredAt == "" {
		event.OccurredAt = now.Format(time.RFC3339)
	}
	return event
}

func (d *webhookDispatcher) dispatch(ctx context.Context, event models.WebhookEvent) error {
	subscriptions, err := d.store.WebhooksForEvent(ctx, event.Kind)
	if err != nil {
		return err
	}
	var deliveryErr error
	for _, subscription := range subscriptions {
		if err := d.deliver(ctx, subscription, event); err != nil {
			deliveryErr = errors.Join(deliveryErr, err)
		}
	}
	return deliveryErr
}

func (d *webhookDispatcher) deliver(ctx context.Context, subscription models.WebhookSubscription, event models.WebhookEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < webhookMaxDeliveryAttempts; attempt++ {
		lastErr = d.post(ctx, subscription, event.Kind, body)
		if lastErr == nil {
			return d.store.RecordWebhookSuccess(ctx, subscription.ID, d.now())
		}
		if err := d.store.RecordWebhookFailure(ctx, subscription.ID, d.now()); err != nil {
			return err
		}
		if attempt < webhookMaxDeliveryAttempts-1 {
			d.sleep(ctx, attempt)
		}
	}
	return lastErr
}

func (d *webhookDispatcher) post(ctx context.Context, subscription models.WebhookSubscription, eventKind string, body []byte) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, subscription.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Tanabata-Event", eventKind)
	request.Header.Set("X-Tanabata-Signature", signWebhookPayload(subscription.Secret, body))
	response, err := d.client.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return errors.New("webhook returned non-2xx status")
	}
	return nil
}

func (d *webhookDispatcher) sleep(ctx context.Context, attempt int) {
	if attempt >= len(d.backoffs) || d.backoffs[attempt] <= 0 {
		return
	}
	timer := time.NewTimer(d.backoffs[attempt])
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func signWebhookPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
