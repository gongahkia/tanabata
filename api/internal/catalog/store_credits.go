package catalog

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

// UpsertWorkCredit inserts or updates a contributor credit for a work.
func (s *Store) UpsertWorkCredit(ctx context.Context, credit models.WorkCredit) (string, error) {
	if strings.TrimSpace(credit.WorkID) == "" {
		return "", errors.New("work_id is required")
	}
	if strings.TrimSpace(credit.CreditedName) == "" {
		return "", errors.New("credited_name is required")
	}
	role := strings.TrimSpace(strings.ToLower(credit.Role))
	if role == "" {
		role = "contributor"
	}
	creditID := strings.TrimSpace(credit.CreditID)
	if creditID == "" {
		creditID = "tanabata:credit:" + search.StableHash(credit.WorkID, credit.CreditedName, role)
	}
	disputed := 0
	if credit.IsDisputed {
		disputed = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO work_credits(credit_id, work_id, credited_artist_id, credited_name, role, is_disputed, notes)
		VALUES(?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(credit_id) DO UPDATE SET
			credited_artist_id = COALESCE(NULLIF(excluded.credited_artist_id, ''), work_credits.credited_artist_id),
			credited_name = excluded.credited_name,
			role = excluded.role,
			is_disputed = excluded.is_disputed,
			notes = COALESCE(NULLIF(excluded.notes, ''), work_credits.notes)
	`, creditID, credit.WorkID, credit.CreditedArtistID, credit.CreditedName, role, disputed, credit.Notes)
	if err != nil {
		return "", err
	}
	return creditID, nil
}

// WorkCredits returns hydrated credits for a work, with the most recent claim view per credit.
func (s *Store) WorkCredits(ctx context.Context, workID string) ([]models.WorkCredit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT credit_id, work_id, credited_artist_id, credited_name, role, is_disputed, notes
		FROM work_credits
		WHERE work_id = ?
		ORDER BY is_disputed DESC, role ASC, credited_name ASC
	`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	credits := []models.WorkCredit{}
	for rows.Next() {
		credit := models.WorkCredit{}
		var disputed int
		if err := rows.Scan(&credit.CreditID, &credit.WorkID, &credit.CreditedArtistID,
			&credit.CreditedName, &credit.Role, &disputed, &credit.Notes); err != nil {
			return nil, err
		}
		credit.IsDisputed = disputed == 1
		credit.Claim, err = s.lookupClaimView(ctx, "credit", "work_credit", credit.CreditID, "work", workID)
		if err != nil {
			return nil, err
		}
		credits = append(credits, credit)
	}
	return credits, rows.Err()
}

// CreditByID returns a single credit row (hydrated). nil if not found.
func (s *Store) CreditByID(ctx context.Context, creditID string) (*models.WorkCredit, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT credit_id, work_id, credited_artist_id, credited_name, role, is_disputed, notes
		FROM work_credits WHERE credit_id = ?
	`, creditID)
	credit := &models.WorkCredit{}
	var disputed int
	if err := row.Scan(&credit.CreditID, &credit.WorkID, &credit.CreditedArtistID,
		&credit.CreditedName, &credit.Role, &disputed, &credit.Notes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	credit.IsDisputed = disputed == 1
	view, err := s.lookupClaimView(ctx, "credit", "work_credit", credit.CreditID, "work", credit.WorkID)
	if err != nil {
		return nil, err
	}
	credit.Claim = view
	return credit, nil
}
