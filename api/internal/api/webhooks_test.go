package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gongahkia/tanabata/api/internal/models"
)

type receivedWebhook struct {
	body      []byte
	signature string
	eventKind string
}

func TestWebhookSubscriptionDispatchesSignedJobEvent(t *testing.T) {
	t.Setenv(webhookAdminTokenEnv, "admin-token")
	received := make(chan receivedWebhook, 1)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read webhook body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		received <- receivedWebhook{
			body:      body,
			signature: r.Header.Get("X-Tanabata-Signature"),
			eventKind: r.Header.Get("X-Tanabata-Event"),
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer receiver.Close()

	server, store := seededServer(t)
	defer store.Close()
	server.webhooks.async = false

	subscription := createTestWebhook(t, server, receiver.URL, []string{"job.completed"})
	if subscription.Secret == "" {
		t.Fatalf("expected create response to include secret once")
	}
	listed := listTestWebhooks(t, server)
	if len(listed) != 1 || listed[0].Secret != "" {
		t.Fatalf("expected redacted list response, got %+v", listed)
	}

	if err := store.RecordJob(context.Background(), models.JobRun{
		JobID:      "webhook-job-1",
		Name:       "webhook test",
		Status:     "succeeded",
		StartedAt:  "2026-07-11T00:00:00Z",
		FinishedAt: "2026-07-11T00:00:01Z",
	}); err != nil {
		t.Fatalf("RecordJob() error = %v", err)
	}

	var got receivedWebhook
	select {
	case got = <-received:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for webhook")
	}
	if got.signature != signWebhookPayload(subscription.Secret, got.body) {
		t.Fatalf("signature = %q, want valid HMAC", got.signature)
	}
	if got.eventKind != "job.completed" {
		t.Fatalf("event kind header = %q, want job.completed", got.eventKind)
	}
	var event models.WebhookEvent
	if err := json.Unmarshal(got.body, &event); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if event.Kind != "job.completed" || event.EventID == "" {
		t.Fatalf("unexpected event %+v", event)
	}
}

func TestWebhookFailedDeliveriesDisableSubscription(t *testing.T) {
	t.Setenv(webhookAdminTokenEnv, "admin-token")
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer receiver.Close()

	server, store := seededServer(t)
	defer store.Close()
	server.webhooks.async = false
	server.webhooks.backoffs = []time.Duration{0, 0, 0, 0, 0}

	subscription := createTestWebhook(t, server, receiver.URL, []string{"job.completed"})
	if err := store.RecordJob(context.Background(), models.JobRun{
		JobID:      "webhook-job-fail",
		Name:       "webhook failure test",
		Status:     "failed",
		StartedAt:  "2026-07-11T00:00:00Z",
		FinishedAt: "2026-07-11T00:00:01Z",
	}); err != nil {
		t.Fatalf("RecordJob() error = %v", err)
	}

	webhooks, err := store.ListWebhooks(context.Background(), true)
	if err != nil {
		t.Fatalf("ListWebhooks() error = %v", err)
	}
	for _, webhook := range webhooks {
		if webhook.ID != subscription.ID {
			continue
		}
		if webhook.FailureCount != 5 || webhook.DisabledAt == "" {
			t.Fatalf("webhook failure state = %+v, want disabled after 5 failures", webhook)
		}
		return
	}
	t.Fatalf("subscription %s missing from %+v", subscription.ID, webhooks)
}

func createTestWebhook(t *testing.T, server *Server, targetURL string, eventKinds []string) models.WebhookSubscription {
	t.Helper()
	body, err := json.Marshal(webhookCreateRequest{URL: targetURL, EventKinds: eventKinds})
	if err != nil {
		t.Fatalf("marshal webhook request: %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/webhooks", strings.NewReader(string(body)))
	request.Header.Set("Authorization", "Bearer admin-token")
	request.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("create webhook status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	var response models.APIResponse[models.WebhookSubscription]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	return response.Data
}

func listTestWebhooks(t *testing.T, server *Server) []models.WebhookSubscription {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/webhooks", nil)
	request.Header.Set("Authorization", "Bearer admin-token")
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list webhook status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	var response models.APIResponse[[]models.WebhookSubscription]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	return response.Data
}
