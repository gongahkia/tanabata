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

// RecordPerformance saves a live performance event and binds a claim.
func (s *Store) RecordPerformance(ctx context.Context, perf models.Performance, claim models.Claim, evidence []models.ClaimEvidence) (string, error) {
	if strings.TrimSpace(perf.ArtistID) == "" {
		return "", errors.New("artist_id is required for a performance")
	}
	if strings.TrimSpace(perf.PerformedAt) == "" {
		return "", errors.New("performed_at is required")
	}
	performanceID := strings.TrimSpace(perf.PerformanceID)
	if performanceID == "" {
		performanceID = "tanabata:perf:" + search.StableHash(perf.ArtistID, perf.WorkID, perf.PerformedAt, perf.Venue)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO performances(performance_id, artist_id, work_id, recording_id, event_name, venue, city, country, performed_at, setlistfm_id, position_in_set, notes)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(performance_id) DO UPDATE SET
			event_name = COALESCE(NULLIF(excluded.event_name, ''), performances.event_name),
			venue = COALESCE(NULLIF(excluded.venue, ''), performances.venue),
			city = COALESCE(NULLIF(excluded.city, ''), performances.city),
			country = COALESCE(NULLIF(excluded.country, ''), performances.country),
			setlistfm_id = COALESCE(NULLIF(excluded.setlistfm_id, ''), performances.setlistfm_id),
			position_in_set = CASE WHEN excluded.position_in_set > 0 THEN excluded.position_in_set ELSE performances.position_in_set END,
			notes = COALESCE(NULLIF(excluded.notes, ''), performances.notes)
	`, performanceID, perf.ArtistID, perf.WorkID, perf.RecordingID, perf.EventName, perf.Venue,
		perf.City, perf.Country, perf.PerformedAt, perf.SetlistFMID, perf.PositionInSet, perf.Notes)
	if err != nil {
		return "", err
	}
	if err := s.syncPerformanceSearch(ctx, performanceID); err != nil {
		return "", err
	}

	claim.Kind = "performance"
	claim.SubjectType = "performance"
	claim.SubjectID = performanceID
	claim.ObjectType = "work"
	claim.ObjectID = perf.WorkID
	claim.Relation = "performed"
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
	return performanceID, nil
}

// PerformanceByID returns a hydrated performance row, nil if not found.
func (s *Store) PerformanceByID(ctx context.Context, performanceID string) (*models.Performance, error) {
	row := s.db.QueryRowContext(ctx, performanceSelectSQL+` WHERE performances.performance_id = ?`, performanceID)
	perf := &models.Performance{}
	if err := scanPerformance(row, perf); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	view, err := s.lookupClaimView(ctx, "performance", "performance", perf.PerformanceID, "work", perf.WorkID)
	if err != nil {
		return nil, err
	}
	perf.Claim = view
	return perf, nil
}

// ListPerformances returns performances with filters and pagination.
func (s *Store) ListPerformances(ctx context.Context, filters models.PerformanceFilters) (models.ListResponse[models.Performance], error) {
	response := models.ListResponse[models.Performance]{Data: []models.Performance{}}
	meta, err := s.Meta(ctx)
	if err != nil {
		return response, err
	}
	response.Meta = meta
	clauses := []string{}
	args := []any{}
	if strings.TrimSpace(filters.ArtistID) != "" {
		clauses = append(clauses, `performances.artist_id = ?`)
		args = append(args, filters.ArtistID)
	}
	if strings.TrimSpace(filters.WorkID) != "" {
		clauses = append(clauses, `performances.work_id = ?`)
		args = append(args, filters.WorkID)
	}
	if strings.TrimSpace(filters.Year) != "" {
		clauses = append(clauses, `substr(performances.performed_at, 1, 4) = ?`)
		args = append(args, filters.Year)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	sortOrder := "DESC"
	if strings.EqualFold(filters.Sort, "asc") {
		sortOrder = "ASC"
	}
	orderClause := ` ORDER BY performances.performed_at DESC LIMIT ? OFFSET ?`
	if sortOrder == "ASC" {
		orderClause = ` ORDER BY performances.performed_at ASC LIMIT ? OFFSET ?`
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM performances`+where, args...).Scan(&total); err != nil {
		return response, err
	}

	limit := normalizeLimit(filters.Limit)
	offset := normalizeOffset(filters.Offset)
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit, offset)
	rows, err := s.db.QueryContext(ctx, performanceSelectSQL+where+orderClause, queryArgs...)
	if err != nil {
		return response, err
	}
	performances := []models.Performance{}
	for rows.Next() {
		perf := models.Performance{}
		if err := scanPerformance(rows, &perf); err != nil {
			_ = rows.Close()
			return response, err
		}
		performances = append(performances, perf)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return response, err
	}
	if err := rows.Close(); err != nil {
		return response, err
	}
	for idx := range performances {
		view, err := s.lookupClaimView(ctx, "performance", "performance", performances[idx].PerformanceID, "work", performances[idx].WorkID)
		if err != nil {
			return response, err
		}
		performances[idx].Claim = view
	}
	response.Data = performances
	response.Pagination = models.Pagination{Limit: limit, Offset: offset, Total: total}
	return response, nil
}

// PerformanceStats computes first/last/gap metrics for an artist (optionally scoped to a work).
func (s *Store) PerformanceStats(ctx context.Context, artistID, workID string) (models.PerformanceStats, error) {
	stats := models.PerformanceStats{ArtistID: artistID, WorkID: workID}
	clauses := []string{`artist_id = ?`}
	args := []any{artistID}
	if strings.TrimSpace(workID) != "" {
		clauses = append(clauses, `work_id = ?`)
		args = append(args, workID)
		var workTitle string
		if err := s.db.QueryRowContext(ctx, `SELECT title FROM works WHERE work_id = ?`, workID).Scan(&workTitle); err == nil {
			stats.WorkTitle = workTitle
		} else if !errors.Is(err, sql.ErrNoRows) {
			return stats, err
		}
	}
	where := " WHERE " + strings.Join(clauses, " AND ")

	var first, last sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT MIN(performed_at), MAX(performed_at) FROM performances`+where, args...).Scan(&first, &last); err != nil {
		return stats, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM performances`+where, args...).Scan(&stats.TotalPerformed); err != nil {
		return stats, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT venue) FROM performances`+where+` AND venue <> ''`, args...).Scan(&stats.Venues); err != nil {
		return stats, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT country) FROM performances`+where+` AND country <> ''`, args...).Scan(&stats.Countries); err != nil {
		return stats, err
	}
	if first.Valid {
		stats.FirstPerformedAt = first.String
	}
	if last.Valid {
		stats.LastPerformedAt = last.String
	}
	if stats.FirstPerformedAt != "" && stats.LastPerformedAt != "" {
		if firstTime, err1 := time.Parse(time.RFC3339, stats.FirstPerformedAt); err1 == nil {
			if lastTime, err2 := time.Parse(time.RFC3339, stats.LastPerformedAt); err2 == nil {
				stats.GapDays = int(lastTime.Sub(firstTime).Hours() / 24)
				if stats.TotalPerformed > 1 {
					stats.AverageGapDays = float64(stats.GapDays) / float64(stats.TotalPerformed-1)
				}
			}
		}
	}
	return stats, nil
}

func (s *Store) syncPerformanceSearch(ctx context.Context, performanceID string) error {
	perf, err := s.PerformanceByID(ctx, performanceID)
	if err != nil || perf == nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM performance_search WHERE performance_id = ?`, performanceID); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO performance_search(performance_id, artist_id, artist_name, work_title, event_name, venue, city, country, performed_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, perf.PerformanceID, perf.ArtistID, perf.ArtistName, perf.WorkTitle, perf.EventName, perf.Venue, perf.City, perf.Country, perf.PerformedAt)
	return err
}

func (s *Store) rebuildPerformanceSearch(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM performance_search`); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT performance_id FROM performances`)
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
		if err := s.syncPerformanceSearch(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

const performanceSelectSQL = `
SELECT performances.performance_id, performances.artist_id, COALESCE(artists.name, ''),
	performances.work_id, COALESCE(works.title, ''), performances.recording_id,
	performances.event_name, performances.venue, performances.city, performances.country,
	performances.performed_at, performances.setlistfm_id, performances.position_in_set, performances.notes
FROM performances
LEFT JOIN artists ON artists.artist_id = performances.artist_id
LEFT JOIN works ON works.work_id = performances.work_id
`

func scanPerformance(scanner rowScanner, perf *models.Performance) error {
	return scanner.Scan(
		&perf.PerformanceID,
		&perf.ArtistID,
		&perf.ArtistName,
		&perf.WorkID,
		&perf.WorkTitle,
		&perf.RecordingID,
		&perf.EventName,
		&perf.Venue,
		&perf.City,
		&perf.Country,
		&perf.PerformedAt,
		&perf.SetlistFMID,
		&perf.PositionInSet,
		&perf.Notes,
	)
}

// SeedCuratedPerformances loads curated performance events with evidence + audit.
func (s *Store) SeedCuratedPerformances(ctx context.Context, bundlePath, jobID string) (int, error) {
	records, meta, err := decodeCuratedBundle[models.CuratedPerformanceRecord](bundlePath)
	if err != nil {
		return 0, err
	}
	_, sourceMeta, err := s.upsertCuratedFixtureSource(ctx, bundlePath, meta)
	if err != nil {
		return 0, err
	}
	imported := 0
	for _, record := range records {
		artistID, err := s.resolveOrCreateArtist(ctx, record.ArtistName)
		if err != nil {
			return imported, err
		}
		workID := ""
		if strings.TrimSpace(record.WorkTitle) != "" {
			workID, err = s.ResolveOrCreateWork(ctx, record.WorkTitle, artistID, "")
			if err != nil {
				return imported, err
			}
		}
		performedAt := record.PerformedAt
		if !strings.Contains(performedAt, "T") {
			performedAt += "T00:00:00Z"
		}
		perf := models.Performance{
			ArtistID:      artistID,
			WorkID:        workID,
			EventName:     record.EventName,
			Venue:         record.Venue,
			City:          record.City,
			Country:       record.Country,
			PerformedAt:   performedAt,
			SetlistFMID:   record.SetlistFMID,
			PositionInSet: record.PositionInSet,
			Notes:         record.Notes,
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
		performanceID, err := s.RecordPerformance(ctx, perf, claim, evidence)
		if err != nil {
			return imported, err
		}
		if err := s.RecordIngestionAuditEvent(ctx, models.IngestionAuditEvent{
			EventID:    uuid.NewString(),
			JobID:      jobID,
			Provider:   "tanabata_curated",
			Target:     "performance:" + performanceID,
			Action:     "record_performance",
			Status:     "succeeded",
			OccurredAt: time.Now().UTC().Format(time.RFC3339),
			Details:    "work=" + workID + " evidence=" + strconv.Itoa(len(record.Evidence)),
			SourceMeta: sourceMeta,
		}); err != nil {
			return imported, err
		}
		imported++
	}
	return imported, nil
}
