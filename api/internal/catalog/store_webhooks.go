package catalog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gongahkia/tanabata/api/internal/models"
)

const webhookMaxFailures = 5

func (s *Store) CreateWebhook(ctx context.Context, rawURL string, eventKinds []string) (models.WebhookSubscription, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	subscription := models.WebhookSubscription{
		ID:         "tanabata:webhook:" + uuid.NewString(),
		URL:        strings.TrimSpace(rawURL),
		Secret:     newWebhookSecret(),
		EventKinds: dedupeStrings(eventKinds),
		CreatedAt:  now,
	}
	kindsJSON, err := json.Marshal(subscription.EventKinds)
	if err != nil {
		return models.WebhookSubscription{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO webhooks(id, url, secret, event_kinds, created_at, disabled_at, last_success_at, failure_count)
		VALUES(?, ?, ?, ?, ?, '', '', 0)
	`, subscription.ID, subscription.URL, subscription.Secret, string(kindsJSON), subscription.CreatedAt)
	if err != nil {
		return models.WebhookSubscription{}, err
	}
	return subscription, nil
}

func (s *Store) ListWebhooks(ctx context.Context, includeDisabled bool) ([]models.WebhookSubscription, error) {
	query := `SELECT id, url, secret, event_kinds, created_at, disabled_at, last_success_at, failure_count FROM webhooks`
	if !includeDisabled {
		query += ` WHERE disabled_at = ''`
	}
	query += ` ORDER BY created_at DESC, id DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	subscriptions := []models.WebhookSubscription{}
	for rows.Next() {
		subscription, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, subscription)
	}
	return subscriptions, rows.Err()
}

func (s *Store) WebhooksForEvent(ctx context.Context, eventKind string) ([]models.WebhookSubscription, error) {
	subscriptions, err := s.ListWebhooks(ctx, false)
	if err != nil {
		return nil, err
	}
	filtered := []models.WebhookSubscription{}
	for _, subscription := range subscriptions {
		if webhookHasEventKind(subscription.EventKinds, eventKind) {
			filtered = append(filtered, subscription)
		}
	}
	return filtered, nil
}

func (s *Store) DeleteWebhook(ctx context.Context, id string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (s *Store) RecordWebhookSuccess(ctx context.Context, id string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE webhooks
		SET last_success_at = ?, failure_count = 0
		WHERE id = ?
	`, at.UTC().Format(time.RFC3339), id)
	return err
}

func (s *Store) RecordWebhookFailure(ctx context.Context, id string, at time.Time) error {
	now := at.UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		UPDATE webhooks
		SET failure_count = failure_count + 1,
			disabled_at = CASE
				WHEN failure_count + 1 >= ? AND disabled_at = '' THEN ?
				ELSE disabled_at
			END
		WHERE id = ?
	`, webhookMaxFailures, now, id)
	return err
}

func scanWebhook(scanner interface{ Scan(dest ...any) error }) (models.WebhookSubscription, error) {
	var subscription models.WebhookSubscription
	var kindsJSON string
	err := scanner.Scan(
		&subscription.ID,
		&subscription.URL,
		&subscription.Secret,
		&kindsJSON,
		&subscription.CreatedAt,
		&subscription.DisabledAt,
		&subscription.LastSuccessAt,
		&subscription.FailureCount,
	)
	if err != nil {
		return subscription, err
	}
	if strings.TrimSpace(kindsJSON) != "" {
		_ = json.Unmarshal([]byte(kindsJSON), &subscription.EventKinds)
	}
	if subscription.EventKinds == nil {
		subscription.EventKinds = []string{}
	}
	return subscription, nil
}

func newWebhookSecret() string {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "whsec_" + uuid.NewString()
	}
	return "whsec_" + hex.EncodeToString(token)
}

func webhookHasEventKind(kinds []string, eventKind string) bool {
	for _, kind := range kinds {
		if kind == eventKind {
			return true
		}
	}
	return false
}

func (s *Store) emitWebhookEvent(ctx context.Context, event models.WebhookEvent) {
	if s.webhookEmitter == nil {
		return
	}
	s.webhookEmitter.EmitWebhookEvent(ctx, event)
}

func isTerminalJobStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "partial":
		return true
	default:
		return false
	}
}

func isDisputeStatus(status string) bool {
	switch status {
	case "disputed", "ambiguous", "refuted":
		return true
	default:
		return false
	}
}

func webhookTimestamp(candidate string) string {
	if strings.TrimSpace(candidate) != "" {
		return candidate
	}
	return time.Now().UTC().Format(time.RFC3339)
}
