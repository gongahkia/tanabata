package catalog

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

var ErrSampleCycle = errors.New("sample edge would create a cycle")

// RecordSampleEdge persists that derivativeID samples sourceID and binds a claim with evidence.
func (s *Store) RecordSampleEdge(ctx context.Context, edge models.SampleEdge, claim models.Claim, evidence []models.ClaimEvidence) (string, error) {
	sourceID := strings.TrimSpace(edge.SourceRecording.RecordingID)
	derivativeID := strings.TrimSpace(edge.DerivativeRecording.RecordingID)
	if sourceID == "" || derivativeID == "" {
		return "", errors.New("sample edge requires source and derivative recording ids")
	}
	kind := defaultSampleKind(edge.Kind)
	cycle, err := s.sampleEdgeWouldCycle(ctx, sourceID, derivativeID)
	if err != nil {
		return "", err
	}
	if cycle {
		return "", ErrSampleCycle
	}
	sampleID := "tanabata:sample:" + search.StableHash(sourceID, derivativeID, kind)
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO samples(sample_id, source_recording_id, derivative_recording_id, kind, source_offset_ms, derivative_offset_ms, duration_ms, notes)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_recording_id, derivative_recording_id, kind) DO UPDATE SET
			kind = excluded.kind,
			source_offset_ms = excluded.source_offset_ms,
			derivative_offset_ms = excluded.derivative_offset_ms,
			duration_ms = excluded.duration_ms,
			notes = COALESCE(NULLIF(excluded.notes, ''), samples.notes)
	`, sampleID, sourceID, derivativeID, kind, edge.SourceOffsetMs, edge.DerivativeOffsetMs, edge.DurationMs, edge.Notes); err != nil {
		if sampleCycleConstraintFailed(err) {
			return "", ErrSampleCycle
		}
		return "", err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT sample_id FROM samples
		WHERE source_recording_id = ? AND derivative_recording_id = ? AND kind = ?
	`, sourceID, derivativeID, kind).Scan(&sampleID); err != nil {
		return "", err
	}
	claim.Kind = "sample"
	claim.SubjectType = "recording"
	claim.SubjectID = derivativeID
	claim.ObjectType = "recording"
	claim.ObjectID = sourceID
	claim.Relation = kind
	if claim.Status == "" {
		claim.Status = "provider_attributed"
	}
	if claim.AssertedAt == "" {
		claim.AssertedAt = time.Now().UTC().Format(time.RFC3339)
	}
	claimID, err := s.RecordClaim(ctx, claim)
	if err != nil {
		return "", err
	}
	for _, ev := range evidence {
		ev.ClaimID = claimID
		if _, err := s.RecordClaimEvidence(ctx, ev); err != nil {
			return "", err
		}
	}
	return sampleID, nil
}

func (s *Store) sampleEdgeWouldCycle(ctx context.Context, sourceID, derivativeID string) (bool, error) {
	if sourceID == derivativeID {
		return true, nil
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `
		WITH RECURSIVE reachable(recording_id, depth) AS (
			SELECT derivative_recording_id, 1
			FROM samples
			WHERE source_recording_id = ?
			UNION ALL
			SELECT samples.derivative_recording_id, reachable.depth + 1
			FROM samples
			JOIN reachable ON samples.source_recording_id = reachable.recording_id
			WHERE reachable.depth < 8
		)
		SELECT EXISTS(SELECT 1 FROM reachable WHERE recording_id = ?)
	`, derivativeID, sourceID).Scan(&exists); err != nil {
		return false, err
	}
	return exists == 1, nil
}

func sampleCycleConstraintFailed(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "samples_no_self_loop") || strings.Contains(message, "CHECK constraint failed")
}

// IncomingSamples lists every recording that sampled (or was derived from) the given recording.
func (s *Store) IncomingSamples(ctx context.Context, recordingID string) ([]models.SampleEdge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT samples.sample_id, samples.source_recording_id, samples.derivative_recording_id,
			samples.kind, samples.source_offset_ms, samples.derivative_offset_ms, samples.duration_ms, samples.notes
		FROM samples
		WHERE samples.source_recording_id = ?
		ORDER BY samples.kind ASC
	`, recordingID)
	return s.sampleEdges(ctx, rows, err)
}

// OutgoingSamples lists every recording that the given recording sampled.
func (s *Store) OutgoingSamples(ctx context.Context, recordingID string) ([]models.SampleEdge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT samples.sample_id, samples.source_recording_id, samples.derivative_recording_id,
			samples.kind, samples.source_offset_ms, samples.derivative_offset_ms, samples.duration_ms, samples.notes
		FROM samples
		WHERE samples.derivative_recording_id = ?
		ORDER BY samples.kind ASC
	`, recordingID)
	return s.sampleEdges(ctx, rows, err)
}

func (s *Store) sampleEdges(ctx context.Context, rows *sql.Rows, err error) ([]models.SampleEdge, error) {
	if err != nil {
		return nil, err
	}
	edges := []models.SampleEdge{}
	sourceIDs := []string{}
	derivativeIDs := []string{}
	for rows.Next() {
		edge := models.SampleEdge{}
		var sourceID, derivativeID string
		if err := rows.Scan(&edge.SampleID, &sourceID, &derivativeID, &edge.Kind,
			&edge.SourceOffsetMs, &edge.DerivativeOffsetMs, &edge.DurationMs, &edge.Notes); err != nil {
			_ = rows.Close()
			return nil, err
		}
		edges = append(edges, edge)
		sourceIDs = append(sourceIDs, sourceID)
		derivativeIDs = append(derivativeIDs, derivativeID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for idx := range edges {
		sourceID := sourceIDs[idx]
		derivativeID := derivativeIDs[idx]
		source, err := s.RecordingByID(ctx, sourceID)
		if err != nil {
			return nil, err
		}
		derivative, err := s.RecordingByID(ctx, derivativeID)
		if err != nil {
			return nil, err
		}
		if source != nil {
			edges[idx].SourceRecording = *source
		}
		if derivative != nil {
			edges[idx].DerivativeRecording = *derivative
		}
		edges[idx].Claim, err = s.lookupClaimView(ctx, "sample", "recording", derivativeID, "recording", sourceID)
		if err != nil {
			return nil, err
		}
	}
	return edges, nil
}

// SampleEdgeByID looks up a single sample edge with hydrated recordings + claim view.
func (s *Store) SampleEdgeByID(ctx context.Context, sampleID string) (*models.SampleEdge, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT sample_id, source_recording_id, derivative_recording_id, kind,
			source_offset_ms, derivative_offset_ms, duration_ms, notes
		FROM samples WHERE sample_id = ?
	`, sampleID)
	edge := &models.SampleEdge{}
	var sourceID, derivativeID string
	if err := row.Scan(&edge.SampleID, &sourceID, &derivativeID, &edge.Kind,
		&edge.SourceOffsetMs, &edge.DerivativeOffsetMs, &edge.DurationMs, &edge.Notes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	source, err := s.RecordingByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	derivative, err := s.RecordingByID(ctx, derivativeID)
	if err != nil {
		return nil, err
	}
	if source != nil {
		edge.SourceRecording = *source
	}
	if derivative != nil {
		edge.DerivativeRecording = *derivative
	}
	edge.Claim, err = s.lookupClaimView(ctx, "sample", "recording", derivativeID, "recording", sourceID)
	if err != nil {
		return nil, err
	}
	return edge, nil
}

func defaultSampleKind(kind string) string {
	kind = strings.TrimSpace(strings.ToLower(kind))
	switch kind {
	case "":
		return "direct_sample"
	case "direct_sample", "interpolation", "replay", "cover_interpolation", "lyrics_quote":
		return kind
	default:
		return kind
	}
}

// SeedCuratedSamples loads curated sample relationships, recording an audit trail.
func (s *Store) SeedCuratedSamples(ctx context.Context, bundlePath, jobID string) (int, error) {
	records, err := decodeJSONFile[models.CuratedSampleRecord](bundlePath)
	if err != nil {
		return 0, err
	}
	imported := 0
	for _, record := range records {
		sourceArtistID, err := s.resolveOrCreateArtist(ctx, record.SourceArtistName)
		if err != nil {
			return imported, err
		}
		derivativeArtistID, err := s.resolveOrCreateArtist(ctx, record.DerivativeArtistName)
		if err != nil {
			return imported, err
		}
		sourceRecID, err := s.UpsertRecording(ctx, models.Recording{
			ArtistID:     sourceArtistID,
			Title:        record.SourceTrackTitle,
			ReleasedYear: record.SourceReleasedYear,
		})
		if err != nil {
			return imported, err
		}
		derivativeRecID, err := s.UpsertRecording(ctx, models.Recording{
			ArtistID:     derivativeArtistID,
			Title:        record.DerivativeTrackTitle,
			ReleasedYear: record.DerivativeReleasedYear,
		})
		if err != nil {
			return imported, err
		}
		edge := models.SampleEdge{
			SourceRecording:     models.Recording{RecordingID: sourceRecID},
			DerivativeRecording: models.Recording{RecordingID: derivativeRecID},
			Kind:                record.Kind,
			SourceOffsetMs:      record.SourceOffsetMs,
			DerivativeOffsetMs:  record.DerivativeOffsetMs,
			DurationMs:          record.DurationMs,
			Notes:               record.Notes,
		}
		claim := models.Claim{
			Status:          defaultStatus(record.Status),
			ConfidenceScore: record.ConfidenceScore,
			ProviderOrigin:  defaultProvider(record.ProviderOrigin),
			AssertedAt:      time.Now().UTC().Format(time.RFC3339),
			LastVerifiedAt:  time.Now().UTC().Format(time.RFC3339),
			Notes:           record.Notes,
		}
		evidence := []models.ClaimEvidence{}
		for _, ev := range record.Evidence {
			evidence = append(evidence, curatedEvidenceToClaim(ev, true))
		}
		if _, err := s.RecordSampleEdge(ctx, edge, claim, evidence); err != nil {
			if errors.Is(err, ErrSampleCycle) {
				if err := s.RecordIngestionAuditEvent(ctx, models.IngestionAuditEvent{
					EventID:    uuid.NewString(),
					JobID:      jobID,
					Provider:   "tanabata_curated",
					Target:     "sample:" + sourceRecID + "->" + derivativeRecID,
					Action:     "record_sample_cycle",
					Status:     "rejected",
					OccurredAt: time.Now().UTC().Format(time.RFC3339),
					Details:    "kind=" + defaultSampleKind(record.Kind) + " error=" + ErrSampleCycle.Error(),
				}); err != nil {
					return imported, err
				}
				continue
			}
			return imported, err
		}
		if err := s.RecordIngestionAuditEvent(ctx, models.IngestionAuditEvent{
			EventID:    uuid.NewString(),
			JobID:      jobID,
			Provider:   "tanabata_curated",
			Target:     "sample:" + sourceRecID + "->" + derivativeRecID,
			Action:     "record_sample",
			Status:     "succeeded",
			OccurredAt: time.Now().UTC().Format(time.RFC3339),
			Details:    "kind=" + record.Kind + " evidence=" + strconv.Itoa(len(record.Evidence)),
		}); err != nil {
			return imported, err
		}
		imported++
	}
	return imported, nil
}
