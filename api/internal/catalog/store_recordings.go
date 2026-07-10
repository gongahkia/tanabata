package catalog

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

// UpsertRecording inserts or updates a Recording. Returns the (possibly generated) recording_id.
func (s *Store) UpsertRecording(ctx context.Context, recording models.Recording) (string, error) {
	title := strings.TrimSpace(recording.Title)
	if title == "" {
		return "", errors.New("recording title is required")
	}
	if strings.TrimSpace(recording.ArtistID) == "" {
		return "", errors.New("recording artist_id is required")
	}
	normalized := search.NormalizeText(title)
	recordingID := strings.TrimSpace(recording.RecordingID)
	if recordingID == "" {
		recordingID = "tanabata:rec:" + search.StableHash(recording.ArtistID, normalized, recording.ReleasedYear)
	}
	isOriginal := 0
	if recording.IsOriginal {
		isOriginal = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO recordings(recording_id, mbid, work_id, artist_id, title, normalized_title,
			duration_ms, released_year, release_id, isrc, is_original, notes)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(recording_id) DO UPDATE SET
			mbid = COALESCE(NULLIF(excluded.mbid, ''), recordings.mbid),
			work_id = COALESCE(NULLIF(excluded.work_id, ''), recordings.work_id),
			title = excluded.title,
			normalized_title = excluded.normalized_title,
			duration_ms = CASE WHEN excluded.duration_ms > 0 THEN excluded.duration_ms ELSE recordings.duration_ms END,
			released_year = COALESCE(NULLIF(excluded.released_year, ''), recordings.released_year),
			release_id = COALESCE(NULLIF(excluded.release_id, ''), recordings.release_id),
			isrc = COALESCE(NULLIF(excluded.isrc, ''), recordings.isrc),
			is_original = excluded.is_original,
			notes = COALESCE(NULLIF(excluded.notes, ''), recordings.notes)
	`, recordingID, recording.MBID, recording.WorkID, recording.ArtistID, title, normalized,
		recording.DurationMs, recording.ReleasedYear, recording.ReleaseID, recording.ISRC, isOriginal, recording.Notes)
	if err != nil {
		return "", err
	}
	if err := s.syncRecordingSearch(ctx, recordingID); err != nil {
		return "", err
	}
	return recordingID, nil
}

// RecordingByID returns a hydrated recording (with sample degrees). nil if not found.
func (s *Store) RecordingByID(ctx context.Context, recordingID string) (*models.Recording, error) {
	row := s.db.QueryRowContext(ctx, recordingSelectSQL+` WHERE recordings.recording_id = ?`, recordingID)
	recording, err := scanRecording(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if err := s.hydrateRecordingDegrees(ctx, &recording); err != nil {
		return nil, err
	}
	return &recording, nil
}

// ListRecordings paginates over the recording catalog with simple filters.
func (s *Store) ListRecordings(ctx context.Context, filters models.RecordingFilters) (models.ListResponse[models.Recording], error) {
	response := models.ListResponse[models.Recording]{Data: []models.Recording{}}
	meta, err := s.Meta(ctx)
	if err != nil {
		return response, err
	}
	response.Meta = meta

	clauses := []string{}
	args := []any{}
	if strings.TrimSpace(filters.ArtistID) != "" {
		clauses = append(clauses, `recordings.artist_id = ?`)
		args = append(args, filters.ArtistID)
	}
	if strings.TrimSpace(filters.WorkID) != "" {
		clauses = append(clauses, `recordings.work_id = ?`)
		args = append(args, filters.WorkID)
	}
	if strings.TrimSpace(filters.Query) != "" {
		clauses = append(clauses, `recordings.normalized_title LIKE ?`)
		args = append(args, "%"+search.NormalizeText(filters.Query)+"%")
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM recordings`+where, args...).Scan(&total); err != nil {
		return response, err
	}

	limit := normalizeLimit(filters.Limit)
	offset := normalizeOffset(filters.Offset)
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit, offset)
	rows, err := s.db.QueryContext(ctx, recordingSelectSQL+where+` ORDER BY recordings.released_year ASC, recordings.title ASC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return response, err
	}
	recordings := []models.Recording{}
	for rows.Next() {
		recording, err := scanRecording(rows)
		if err != nil {
			_ = rows.Close()
			return response, err
		}
		recordings = append(recordings, recording)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return response, err
	}
	if err := rows.Close(); err != nil {
		return response, err
	}
	for idx := range recordings {
		if err := s.hydrateRecordingDegrees(ctx, &recordings[idx]); err != nil {
			return response, err
		}
	}
	response.Data = recordings
	response.Pagination = models.Pagination{Limit: limit, Offset: offset, Total: total}
	return response, nil
}

// ArtistRecordings lists every recording for an artist (used by /v1/artists/{id}/recordings).
func (s *Store) ArtistRecordings(ctx context.Context, artistID string) ([]models.Recording, error) {
	response, err := s.ListRecordings(ctx, models.RecordingFilters{ArtistID: artistID, Limit: 100})
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

const recordingSelectSQL = `
SELECT recordings.recording_id, recordings.mbid, recordings.work_id,
	recordings.artist_id, COALESCE(artists.name, ''), recordings.title,
	recordings.duration_ms, recordings.released_year, recordings.release_id,
	recordings.isrc, recordings.is_original, recordings.notes,
	COALESCE(works.title, '')
FROM recordings
LEFT JOIN artists ON artists.artist_id = recordings.artist_id
LEFT JOIN works ON works.work_id = recordings.work_id
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecording(scanner rowScanner) (models.Recording, error) {
	recording := models.Recording{}
	var isOriginal int
	if err := scanner.Scan(
		&recording.RecordingID,
		&recording.MBID,
		&recording.WorkID,
		&recording.ArtistID,
		&recording.ArtistName,
		&recording.Title,
		&recording.DurationMs,
		&recording.ReleasedYear,
		&recording.ReleaseID,
		&recording.ISRC,
		&isOriginal,
		&recording.Notes,
		&recording.WorkTitle,
	); err != nil {
		return recording, err
	}
	recording.IsOriginal = isOriginal == 1
	return recording, nil
}

func (s *Store) hydrateRecordingDegrees(ctx context.Context, recording *models.Recording) error {
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM samples WHERE derivative_recording_id = ?`, recording.RecordingID).Scan(&recording.SampleOutDeg); err != nil {
		return err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM samples WHERE source_recording_id = ?`, recording.RecordingID).Scan(&recording.SampleInDeg); err != nil {
		return err
	}
	return nil
}

func (s *Store) syncRecordingSearch(ctx context.Context, recordingID string) error {
	recording, err := s.RecordingByID(ctx, recordingID)
	if err != nil || recording == nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM recording_search WHERE recording_id = ?`, recordingID); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO recording_search(recording_id, artist_id, artist_name, title, work_title)
		VALUES(?, ?, ?, ?, ?)
	`, recording.RecordingID, recording.ArtistID, recording.ArtistName, recording.Title, recording.WorkTitle)
	return err
}

func (s *Store) rebuildRecordingSearch(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM recording_search`); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT recording_id FROM recordings`)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.syncRecordingSearch(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// ResolveRecordingByTitle finds a recording by artist + title, optionally creating it.
func (s *Store) ResolveOrCreateRecording(ctx context.Context, artistID, title, releasedYear string) (string, error) {
	normalized := search.NormalizeText(title)
	var id string
	err := s.db.QueryRowContext(ctx, `
		SELECT recording_id FROM recordings
		WHERE artist_id = ? AND normalized_title = ?
		ORDER BY released_year ASC LIMIT 1
	`, artistID, normalized).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	return s.UpsertRecording(ctx, models.Recording{
		ArtistID:     artistID,
		Title:        title,
		ReleasedYear: releasedYear,
	})
}
