package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

// UpsertWork inserts or updates a Work composition by stable work_id.
// If WorkID is empty, a stable hash is generated from (title, primary_artist_id).
func (s *Store) UpsertWork(ctx context.Context, work models.Work) (string, error) {
	title := strings.TrimSpace(work.Title)
	if title == "" {
		return "", errors.New("work title is required")
	}
	normalized := search.NormalizeText(title)
	workID := strings.TrimSpace(work.WorkID)
	if workID == "" {
		workID = "tanabata:work:" + search.StableHash(work.PrimaryArtistID, normalized)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO works(work_id, mbid, title, normalized_title, iswc, language, created_year, primary_artist_id, notes)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(work_id) DO UPDATE SET
			mbid = COALESCE(NULLIF(excluded.mbid, ''), works.mbid),
			title = excluded.title,
			normalized_title = excluded.normalized_title,
			iswc = COALESCE(NULLIF(excluded.iswc, ''), works.iswc),
			language = COALESCE(NULLIF(excluded.language, ''), works.language),
			created_year = COALESCE(NULLIF(excluded.created_year, ''), works.created_year),
			primary_artist_id = COALESCE(NULLIF(excluded.primary_artist_id, ''), works.primary_artist_id),
			notes = COALESCE(NULLIF(excluded.notes, ''), works.notes)
	`, workID, work.MBID, title, normalized, work.ISWC, work.Language, work.CreatedYear, work.PrimaryArtistID, work.Notes)
	if err != nil {
		return "", err
	}
	if err := s.syncWorkSearch(ctx, workID); err != nil {
		return "", err
	}
	return workID, nil
}

// WorkByID returns the work with hydrated counts. nil if not found.
func (s *Store) WorkByID(ctx context.Context, workID string) (*models.Work, error) {
	work, err := s.scanWorkByID(ctx, workID)
	if err != nil {
		return nil, err
	}
	return work, nil
}

func (s *Store) scanWorkByID(ctx context.Context, workID string) (*models.Work, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT works.work_id, works.mbid, works.title, works.iswc, works.language,
			works.created_year, works.primary_artist_id, COALESCE(artists.name, ''), works.notes
		FROM works
		LEFT JOIN artists ON artists.artist_id = works.primary_artist_id
		WHERE works.work_id = ?
	`, workID)
	work := &models.Work{}
	if err := row.Scan(&work.WorkID, &work.MBID, &work.Title, &work.ISWC, &work.Language,
		&work.CreatedYear, &work.PrimaryArtistID, &work.PrimaryArtistName, &work.Notes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if err := s.hydrateWorkCounts(ctx, work); err != nil {
		return nil, err
	}
	return work, nil
}

func (s *Store) hydrateWorkCounts(ctx context.Context, work *models.Work) error {
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM recordings WHERE work_id = ?`, work.WorkID).Scan(&work.RecordingCount); err != nil {
		return err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_credits WHERE work_id = ?`, work.WorkID).Scan(&work.CreditCount); err != nil {
		return err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_credits WHERE work_id = ? AND is_disputed = 1`, work.WorkID).Scan(&work.DisputedCredits); err != nil {
		return err
	}
	return nil
}

// ListWorks returns paginated works with optional fts/artist filters.
func (s *Store) ListWorks(ctx context.Context, filters models.WorkFilters) (models.ListResponse[models.Work], error) {
	response := models.ListResponse[models.Work]{
		Data: []models.Work{},
	}
	meta, err := s.Meta(ctx)
	if err != nil {
		return response, err
	}
	response.Meta = meta

	clauses := []string{}
	args := []any{}
	if strings.TrimSpace(filters.ArtistID) != "" {
		clauses = append(clauses, `works.primary_artist_id = ?`)
		args = append(args, filters.ArtistID)
	}
	if strings.TrimSpace(filters.Query) != "" {
		clauses = append(clauses, `works.normalized_title LIKE ?`)
		args = append(args, "%"+search.NormalizeText(filters.Query)+"%")
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM works `+where, args...).Scan(&total); err != nil {
		return response, err
	}

	limit := normalizeLimit(filters.Limit)
	offset := normalizeOffset(filters.Offset)
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit, offset)

	rows, err := s.db.QueryContext(ctx, `
		SELECT works.work_id, works.mbid, works.title, works.iswc, works.language,
			works.created_year, works.primary_artist_id, COALESCE(artists.name, ''), works.notes
		FROM works
		LEFT JOIN artists ON artists.artist_id = works.primary_artist_id
		`+where+`
		ORDER BY works.title ASC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return response, err
	}
	defer rows.Close()
	for rows.Next() {
		var work models.Work
		if err := rows.Scan(&work.WorkID, &work.MBID, &work.Title, &work.ISWC, &work.Language,
			&work.CreatedYear, &work.PrimaryArtistID, &work.PrimaryArtistName, &work.Notes); err != nil {
			return response, err
		}
		if err := s.hydrateWorkCounts(ctx, &work); err != nil {
			return response, err
		}
		response.Data = append(response.Data, work)
	}
	response.Pagination = models.Pagination{Limit: limit, Offset: offset, Total: total}
	return response, rows.Err()
}

// WorkRecordings lists every recording associated with a work, original first.
func (s *Store) WorkRecordings(ctx context.Context, workID string) ([]models.Recording, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT recordings.recording_id, recordings.mbid, recordings.work_id,
			recordings.artist_id, COALESCE(artists.name, ''), recordings.title,
			recordings.duration_ms, recordings.released_year, recordings.release_id,
			recordings.isrc, recordings.is_original, recordings.notes,
			COALESCE(works.title, '')
		FROM recordings
		LEFT JOIN artists ON artists.artist_id = recordings.artist_id
		LEFT JOIN works ON works.work_id = recordings.work_id
		WHERE recordings.work_id = ?
		ORDER BY recordings.is_original DESC, recordings.released_year ASC, recordings.title ASC
	`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	recordings := []models.Recording{}
	for rows.Next() {
		recording, err := scanRecording(rows)
		if err != nil {
			return nil, err
		}
		if err := s.hydrateRecordingDegrees(ctx, &recording); err != nil {
			return nil, err
		}
		recordings = append(recordings, recording)
	}
	return recordings, rows.Err()
}

func (s *Store) syncWorkSearch(ctx context.Context, workID string) error {
	work, err := s.scanWorkByID(ctx, workID)
	if err != nil || work == nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM work_search WHERE work_id = ?`, workID); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO work_search(work_id, title, primary_artist_name)
		VALUES(?, ?, ?)
	`, work.WorkID, work.Title, work.PrimaryArtistName)
	return err
}

// ResolveWorkID looks up a work by exact title under an artist, creating it if absent.
func (s *Store) ResolveOrCreateWork(ctx context.Context, title, primaryArtistID, createdYear string) (string, error) {
	normalized := search.NormalizeText(title)
	var workID string
	err := s.db.QueryRowContext(ctx, `
		SELECT work_id FROM works WHERE normalized_title = ? AND (primary_artist_id = ? OR ? = '')
		ORDER BY CASE WHEN primary_artist_id = ? THEN 0 ELSE 1 END
		LIMIT 1
	`, normalized, primaryArtistID, primaryArtistID, primaryArtistID).Scan(&workID)
	if err == nil {
		return workID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	return s.UpsertWork(ctx, models.Work{
		Title:           title,
		PrimaryArtistID: primaryArtistID,
		CreatedYear:     createdYear,
	})
}

// rebuildWorkSearch reseeds the work_search virtual table.
func (s *Store) rebuildWorkSearch(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM work_search`); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT work_id FROM works`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var workID string
		if err := rows.Scan(&workID); err != nil {
			return err
		}
		if err := s.syncWorkSearch(ctx, workID); err != nil {
			return err
		}
	}
	return rows.Err()
}

// SeedCuratedWorks loads works + nested credits/covers from JSON, recording an audit trail.
func (s *Store) SeedCuratedWorks(ctx context.Context, bundlePath, jobID string) (int, error) {
	records, err := decodeJSONFile[models.CuratedWorkRecord](bundlePath)
	if err != nil {
		return 0, err
	}
	imported := 0
	now := time.Now().UTC().Format(time.RFC3339)
	for _, record := range records {
		artistID, err := s.ResolveArtistID(ctx, record.PrimaryArtistName)
		if err != nil {
			return imported, fmt.Errorf("resolve work artist %q: %w", record.PrimaryArtistName, err)
		}
		if artistID == "" {
			artistID = search.ArtistID(record.PrimaryArtistName, "")
			if err := s.UpsertArtist(ctx, models.Artist{
				ArtistID: artistID,
				Name:     record.PrimaryArtistName,
				Aliases:  []string{record.PrimaryArtistName},
				ProviderStatus: map[string]string{
					"tanabata_curated": "imported",
				},
			}); err != nil {
				return imported, err
			}
		}
		workID, err := s.UpsertWork(ctx, models.Work{
			Title:           record.Title,
			PrimaryArtistID: artistID,
			CreatedYear:     record.CreatedYear,
			ISWC:            record.ISWC,
			Language:        record.Language,
			Notes:           record.Notes,
		})
		if err != nil {
			return imported, err
		}

		for _, cover := range record.Covers {
			coverArtistID, err := s.ResolveArtistID(ctx, cover.ArtistName)
			if err != nil {
				return imported, err
			}
			if coverArtistID == "" {
				coverArtistID = search.ArtistID(cover.ArtistName, "")
				if err := s.UpsertArtist(ctx, models.Artist{
					ArtistID: coverArtistID,
					Name:     cover.ArtistName,
					Aliases:  []string{cover.ArtistName},
					ProviderStatus: map[string]string{
						"tanabata_curated": "imported",
					},
				}); err != nil {
					return imported, err
				}
			}
			recordingTitle := strings.TrimSpace(cover.RecordingTitle)
			if recordingTitle == "" {
				recordingTitle = record.Title
			}
			recordingID, err := s.UpsertRecording(ctx, models.Recording{
				WorkID:       workID,
				ArtistID:     coverArtistID,
				Title:        recordingTitle,
				ReleasedYear: cover.ReleasedYear,
				IsOriginal:   cover.IsOriginal,
				Notes:        cover.Notes,
			})
			if err != nil {
				return imported, err
			}
			claimID, err := s.RecordClaim(ctx, models.Claim{
				Kind:            "cover",
				SubjectType:     "recording",
				SubjectID:       recordingID,
				ObjectType:      "work",
				ObjectID:        workID,
				Relation:        "recording_of",
				Status:          defaultStatus(cover.Status),
				ConfidenceScore: cover.ConfidenceScore,
				ProviderOrigin:  defaultProvider(cover.ProviderOrigin),
				AssertedAt:      now,
				LastVerifiedAt:  now,
				Notes:           cover.Notes,
			})
			if err != nil {
				return imported, err
			}
			if err := s.recordCuratedEvidence(ctx, claimID, cover.Evidence, true); err != nil {
				return imported, err
			}
		}

		for _, credit := range record.Credits {
			creditArtistID, _ := s.ResolveArtistID(ctx, credit.CreditedName)
			creditID, err := s.UpsertWorkCredit(ctx, models.WorkCredit{
				WorkID:           workID,
				CreditedArtistID: creditArtistID,
				CreditedName:     credit.CreditedName,
				Role:             credit.Role,
				IsDisputed:       credit.IsDisputed,
				Notes:            credit.Notes,
			})
			if err != nil {
				return imported, err
			}
			status := defaultStatus(credit.Status)
			if credit.IsDisputed && status == "verified" {
				status = "ambiguous"
			}
			claimID, err := s.RecordClaim(ctx, models.Claim{
				Kind:            "credit",
				SubjectType:     "work_credit",
				SubjectID:       creditID,
				ObjectType:      "work",
				ObjectID:        workID,
				Relation:        credit.Role,
				Status:          status,
				ConfidenceScore: credit.ConfidenceScore,
				ProviderOrigin:  defaultProvider(credit.ProviderOrigin),
				AssertedAt:      now,
				LastVerifiedAt:  now,
				Notes:           credit.Notes,
			})
			if err != nil {
				return imported, err
			}
			if err := s.recordCuratedEvidence(ctx, claimID, credit.Evidence, true); err != nil {
				return imported, err
			}
		}

		if err := s.RecordIngestionAuditEvent(ctx, models.IngestionAuditEvent{
			EventID:    uuid.NewString(),
			JobID:      jobID,
			Provider:   "tanabata_curated",
			Target:     "work:" + workID,
			Action:     "upsert_work",
			Status:     "succeeded",
			OccurredAt: now,
			Details:    fmt.Sprintf("covers=%d credits=%d", len(record.Covers), len(record.Credits)),
		}); err != nil {
			return imported, err
		}
		imported++
	}
	return imported, nil
}
