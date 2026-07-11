package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/search"
)

// RecordClaim inserts a new claim (or updates the existing one keyed by subject+object+kind).
func (s *Store) RecordClaim(ctx context.Context, claim models.Claim) (string, error) {
	if strings.TrimSpace(claim.Kind) == "" {
		return "", errors.New("claim kind is required")
	}
	if strings.TrimSpace(claim.SubjectType) == "" || strings.TrimSpace(claim.SubjectID) == "" {
		return "", errors.New("claim subject is required")
	}
	if strings.TrimSpace(claim.ObjectType) == "" || strings.TrimSpace(claim.ObjectID) == "" {
		return "", errors.New("claim object is required")
	}
	if strings.TrimSpace(claim.AssertedAt) == "" {
		claim.AssertedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if strings.TrimSpace(claim.Status) == "" {
		claim.Status = "needs_review"
	}
	claimID := strings.TrimSpace(claim.ClaimID)
	if claimID == "" {
		claimID = "tanabata:claim:" + search.StableHash(claim.Kind, claim.SubjectType, claim.SubjectID, claim.ObjectType, claim.ObjectID, claim.Relation)
	}
	// Resolve the strongest status in Go rather than via a SQL UDF (SQLite has no native rank).
	var existingStatus string
	var existingConfidence float64
	err := s.db.QueryRowContext(ctx, `SELECT status, confidence_score FROM claims WHERE claim_id = ?`, claimID).Scan(&existingStatus, &existingConfidence)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// fresh insert
	case err != nil:
		return "", err
	default:
		if claimStatusRank(existingStatus) >= claimStatusRank(claim.Status) {
			claim.Status = existingStatus
		}
		if existingConfidence > claim.ConfidenceScore {
			claim.ConfidenceScore = existingConfidence
		}
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO claims(claim_id, kind, subject_type, subject_id, object_type, object_id, relation,
			status, confidence_score, provider_origin, source_id, asserted_at, last_verified_at, notes)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(claim_id) DO UPDATE SET
			status = excluded.status,
			confidence_score = excluded.confidence_score,
			provider_origin = COALESCE(NULLIF(excluded.provider_origin, ''), claims.provider_origin),
			source_id = COALESCE(NULLIF(excluded.source_id, ''), claims.source_id),
			last_verified_at = COALESCE(NULLIF(excluded.last_verified_at, ''), claims.last_verified_at),
			notes = COALESCE(NULLIF(excluded.notes, ''), claims.notes)
	`, claimID, claim.Kind, claim.SubjectType, claim.SubjectID, claim.ObjectType, claim.ObjectID, claim.Relation,
		claim.Status, claim.ConfidenceScore, claim.ProviderOrigin, claim.SourceID, claim.AssertedAt, claim.LastVerifiedAt, claim.Notes)
	if err != nil {
		return "", err
	}
	return claimID, nil
}

func claimStatusRank(status string) int {
	switch status {
	case "verified":
		return 6
	case "source_attributed":
		return 5
	case "provider_attributed":
		return 4
	case "ambiguous", "disputed":
		return 3
	case "needs_review":
		return 2
	case "refuted":
		return 1
	default:
		return 0
	}
}

// RecordClaimEvidence appends supporting or refuting evidence to a claim.
func (s *Store) RecordClaimEvidence(ctx context.Context, evidence models.ClaimEvidence) (string, error) {
	if strings.TrimSpace(evidence.ClaimID) == "" {
		return "", errors.New("claim_id is required")
	}
	if strings.TrimSpace(evidence.Excerpt) == "" {
		return "", errors.New("evidence excerpt is required")
	}
	evidenceID := strings.TrimSpace(evidence.EvidenceID)
	if evidenceID == "" {
		evidenceID = "tanabata:ev:" + search.StableHash(evidence.ClaimID, evidence.Excerpt, evidence.SourceURL)
	}
	if evidence.Weight == 0 {
		evidence.Weight = 1.0
	}
	if strings.TrimSpace(evidence.EvidenceKind) == "" {
		evidence.EvidenceKind = "manual_note"
	}
	if strings.TrimSpace(evidence.RecordedAt) == "" {
		evidence.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	}
	supports := 1
	if !evidence.Supports {
		supports = 0
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO claim_evidence(evidence_id, claim_id, supports, source_id, excerpt, source_url, archived_url, evidence_kind, weight, recorded_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(evidence_id) DO UPDATE SET
			supports = excluded.supports,
			excerpt = excluded.excerpt,
			source_url = excluded.source_url,
			archived_url = excluded.archived_url,
			evidence_kind = excluded.evidence_kind,
			weight = excluded.weight,
			recorded_at = excluded.recorded_at
	`, evidenceID, evidence.ClaimID, supports, evidence.SourceID, evidence.Excerpt,
		evidence.SourceURL, evidence.ArchivedURL, evidence.EvidenceKind, evidence.Weight, evidence.RecordedAt)
	return evidenceID, err
}

// ClaimByID returns a full claim with hydrated evidence + counts.
func (s *Store) ClaimByID(ctx context.Context, claimID string) (*models.Claim, error) {
	row := s.db.QueryRowContext(ctx, claimSelectSQL+` WHERE claim_id = ?`, claimID)
	claim := &models.Claim{}
	if err := scanClaim(row, claim); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	supporting, refuting, err := s.evidenceCounts(ctx, claimID)
	if err != nil {
		return nil, err
	}
	claim.SupportingCount = supporting
	claim.RefutingCount = refuting
	claim.Evidence, err = s.ClaimEvidence(ctx, claimID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(claim.SourceID) != "" {
		if src, err := s.SourceByID(ctx, claim.SourceID); err == nil {
			claim.Source = src
		}
	}
	return claim, nil
}

// ClaimEvidence lists the evidence rows for a claim, supporting first.
func (s *Store) ClaimEvidence(ctx context.Context, claimID string) ([]models.ClaimEvidence, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT evidence_id, claim_id, supports, source_id, excerpt, source_url, archived_url, evidence_kind, weight, recorded_at
		FROM claim_evidence
		WHERE claim_id = ?
		ORDER BY supports DESC, recorded_at ASC
	`, claimID)
	if err != nil {
		return nil, err
	}
	evidence := []models.ClaimEvidence{}
	for rows.Next() {
		ev := models.ClaimEvidence{}
		var supports int
		if err := rows.Scan(&ev.EvidenceID, &ev.ClaimID, &supports, &ev.SourceID,
			&ev.Excerpt, &ev.SourceURL, &ev.ArchivedURL, &ev.EvidenceKind, &ev.Weight, &ev.RecordedAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ev.Supports = supports == 1
		evidence = append(evidence, ev)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for idx := range evidence {
		if evidence[idx].SourceID != "" {
			evidence[idx].Source, _ = s.SourceByID(ctx, evidence[idx].SourceID)
		}
	}
	return evidence, nil
}

// ListClaims paginates claims with filters.
func (s *Store) ListClaims(ctx context.Context, filters models.ClaimFilters) (models.ListResponse[models.Claim], error) {
	response := models.ListResponse[models.Claim]{Data: []models.Claim{}}
	meta, err := s.Meta(ctx)
	if err != nil {
		return response, err
	}
	response.Meta = meta

	clauses := []string{}
	args := []any{}
	if strings.TrimSpace(filters.Kind) != "" {
		clauses = append(clauses, `kind = ?`)
		args = append(args, filters.Kind)
	}
	if strings.TrimSpace(filters.Status) != "" {
		clauses = append(clauses, `status = ?`)
		args = append(args, filters.Status)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM claims`+where, args...).Scan(&total); err != nil {
		return response, err
	}
	limit := normalizeLimit(filters.Limit)
	offset := normalizeOffset(filters.Offset)
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit, offset)
	rows, err := s.db.QueryContext(ctx, claimSelectSQL+where+` ORDER BY asserted_at DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return response, err
	}
	claims := []models.Claim{}
	for rows.Next() {
		claim := models.Claim{}
		if err := scanClaim(rows, &claim); err != nil {
			_ = rows.Close()
			return response, err
		}
		claims = append(claims, claim)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return response, err
	}
	if err := rows.Close(); err != nil {
		return response, err
	}
	for idx := range claims {
		claims[idx].SupportingCount, claims[idx].RefutingCount, err = s.evidenceCounts(ctx, claims[idx].ClaimID)
		if err != nil {
			return response, err
		}
	}
	response.Data = claims
	response.Pagination = models.Pagination{Limit: limit, Offset: offset, Total: total}
	return response, nil
}

// Disputes returns claims whose status indicates contested provenance.
func (s *Store) Disputes(ctx context.Context, limit int) ([]models.Dispute, error) {
	limit = normalizeLimit(limit)
	rows, err := s.db.QueryContext(ctx, claimSelectSQL+`
		WHERE status IN ('ambiguous', 'disputed', 'refuted', 'needs_review')
		ORDER BY asserted_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	claims := []models.Claim{}
	for rows.Next() {
		claim := models.Claim{}
		if err := scanClaim(rows, &claim); err != nil {
			_ = rows.Close()
			return nil, err
		}
		claims = append(claims, claim)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	disputes := []models.Dispute{}
	for idx := range claims {
		claims[idx].SupportingCount, claims[idx].RefutingCount, err = s.evidenceCounts(ctx, claims[idx].ClaimID)
		if err != nil {
			return nil, err
		}
		subjectLabel, objectLabel := s.describeClaim(ctx, claims[idx])
		disputes = append(disputes, models.Dispute{
			Claim:            claims[idx],
			SubjectLabel:     subjectLabel,
			ObjectLabel:      objectLabel,
			HumanDescription: humanizeDispute(claims[idx], subjectLabel, objectLabel),
		})
	}
	return disputes, nil
}

// QuoteLineage assembles all evidence and rivals for a quote attribution.
func (s *Store) QuoteLineage(ctx context.Context, quoteID string) (*models.QuoteLineage, error) {
	quote, err := s.QuoteByID(ctx, quoteID)
	if err != nil {
		return nil, err
	}
	if quote == nil {
		return nil, nil
	}
	lineage := &models.QuoteLineage{
		QuoteID:          quoteID,
		Text:             quote.Text,
		AttributedToID:   quote.ArtistID,
		AttributedToName: quote.ArtistName,
		ProvenanceStatus: quote.ProvenanceStatus,
		ConfidenceScore:  quote.ConfidenceScore,
		Supporting:       []models.ClaimEvidence{},
		Refuting:         []models.ClaimEvidence{},
	}
	rows, err := s.db.QueryContext(ctx, claimSelectSQL+`
		WHERE subject_type = 'quote' AND subject_id = ? AND kind = 'attribution'
		ORDER BY asserted_at ASC
	`, quoteID)
	if err != nil {
		return nil, err
	}
	claims := []models.Claim{}
	for rows.Next() {
		claim := models.Claim{}
		if err := scanClaim(rows, &claim); err != nil {
			_ = rows.Close()
			return nil, err
		}
		claims = append(claims, claim)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for idx := range claims {
		claims[idx].Evidence, err = s.ClaimEvidence(ctx, claims[idx].ClaimID)
		if err != nil {
			return nil, err
		}
		for _, ev := range claims[idx].Evidence {
			if ev.Supports {
				lineage.Supporting = append(lineage.Supporting, ev)
			} else {
				lineage.Refuting = append(lineage.Refuting, ev)
			}
		}
		if claims[idx].ObjectType == "artist" && claims[idx].ObjectID != quote.ArtistID {
			lineage.RivalClaims = append(lineage.RivalClaims, claims[idx])
		}
	}
	for _, ev := range lineage.Supporting {
		lineage.EarliestEvidenceAt = earliestTimestamp(lineage.EarliestEvidenceAt, ev.RecordedAt)
		lineage.LatestEvidenceAt = latestTimestamp(lineage.LatestEvidenceAt, ev.RecordedAt)
	}
	for _, ev := range lineage.Refuting {
		lineage.EarliestEvidenceAt = earliestTimestamp(lineage.EarliestEvidenceAt, ev.RecordedAt)
		lineage.LatestEvidenceAt = latestTimestamp(lineage.LatestEvidenceAt, ev.RecordedAt)
	}
	lineage.MergeHistory, err = s.QuoteMergeHistory(ctx, quoteID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(lineage.Supporting, func(i, j int) bool {
		return lineage.Supporting[i].RecordedAt < lineage.Supporting[j].RecordedAt
	})
	sort.SliceStable(lineage.Refuting, func(i, j int) bool {
		return lineage.Refuting[i].RecordedAt < lineage.Refuting[j].RecordedAt
	})
	return lineage, nil
}

// QuoteMergeHistory returns the merge log entries where this quote_id is winner OR loser.
func (s *Store) QuoteMergeHistory(ctx context.Context, quoteID string) ([]models.QuoteMergeLog, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT merge_id, winner_quote_id, loser_quote_id, merge_score, reason, merged_at, job_id
		FROM quote_merge_log
		WHERE winner_quote_id = ? OR loser_quote_id = ?
		ORDER BY merged_at DESC
	`, quoteID, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := []models.QuoteMergeLog{}
	for rows.Next() {
		entry := models.QuoteMergeLog{}
		if err := rows.Scan(&entry.MergeID, &entry.WinnerQuoteID, &entry.LoserQuoteID,
			&entry.MergeScore, &entry.Reason, &entry.MergedAt, &entry.JobID); err != nil {
			return nil, err
		}
		logs = append(logs, entry)
	}
	return logs, rows.Err()
}

// RecordQuoteMerge persists a merge decision (called by ingestion when duplicates are folded).
func (s *Store) RecordQuoteMerge(ctx context.Context, log models.QuoteMergeLog) error {
	return recordQuoteMerge(ctx, s.db, log)
}

type quoteMergeExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func recordQuoteMerge(ctx context.Context, execer quoteMergeExecutor, log models.QuoteMergeLog) error {
	if strings.TrimSpace(log.MergeID) == "" {
		log.MergeID = "tanabata:merge:" + search.StableHash(log.WinnerQuoteID, log.LoserQuoteID, log.Reason)
	}
	if strings.TrimSpace(log.MergedAt) == "" {
		log.MergedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := execer.ExecContext(ctx, `
		INSERT INTO quote_merge_log(merge_id, winner_quote_id, loser_quote_id, merge_score, reason, merged_at, job_id)
		VALUES(?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(merge_id) DO NOTHING
	`, log.MergeID, log.WinnerQuoteID, log.LoserQuoteID, log.MergeScore, log.Reason, log.MergedAt, log.JobID)
	return err
}

const claimSelectSQL = `
SELECT claim_id, kind, subject_type, subject_id, object_type, object_id, relation,
	status, confidence_score, provider_origin, source_id, asserted_at, last_verified_at, notes
FROM claims
`

func scanClaim(scanner rowScanner, claim *models.Claim) error {
	return scanner.Scan(
		&claim.ClaimID,
		&claim.Kind,
		&claim.SubjectType,
		&claim.SubjectID,
		&claim.ObjectType,
		&claim.ObjectID,
		&claim.Relation,
		&claim.Status,
		&claim.ConfidenceScore,
		&claim.ProviderOrigin,
		&claim.SourceID,
		&claim.AssertedAt,
		&claim.LastVerifiedAt,
		&claim.Notes,
	)
}

func (s *Store) evidenceCounts(ctx context.Context, claimID string) (int, int, error) {
	var supporting, refuting int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM claim_evidence WHERE claim_id = ? AND supports = 1`, claimID).Scan(&supporting); err != nil {
		return 0, 0, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM claim_evidence WHERE claim_id = ? AND supports = 0`, claimID).Scan(&refuting); err != nil {
		return 0, 0, err
	}
	return supporting, refuting, nil
}

func (s *Store) lookupClaimView(ctx context.Context, kind, subjectType, subjectID, objectType, objectID string) (*models.ClaimView, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT claim_id, status, confidence_score, provider_origin
		FROM claims
		WHERE kind = ? AND subject_type = ? AND subject_id = ? AND object_type = ? AND object_id = ?
		LIMIT 1
	`, kind, subjectType, subjectID, objectType, objectID)
	view := &models.ClaimView{}
	if err := row.Scan(&view.ClaimID, &view.Status, &view.ConfidenceScore, &view.ProviderOrigin); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	supporting, refuting, err := s.evidenceCounts(ctx, view.ClaimID)
	if err != nil {
		return nil, err
	}
	view.SupportingCount = supporting
	view.RefutingCount = refuting
	return view, nil
}

func (s *Store) describeClaim(ctx context.Context, claim models.Claim) (string, string) {
	subject, object := claim.SubjectID, claim.ObjectID
	switch claim.SubjectType {
	case "quote":
		if quote, _ := s.QuoteByID(ctx, claim.SubjectID); quote != nil {
			subject = "“" + truncate(quote.Text, 64) + "”"
		}
	case "recording":
		if rec, _ := s.RecordingByID(ctx, claim.SubjectID); rec != nil {
			subject = rec.ArtistName + " — " + rec.Title
		}
	case "work_credit":
		if credit, _ := s.CreditByID(ctx, claim.SubjectID); credit != nil {
			subject = credit.CreditedName + " (" + credit.Role + ")"
		}
	case "performance":
		if perf, _ := s.PerformanceByID(ctx, claim.SubjectID); perf != nil {
			subject = perf.ArtistName + " @ " + perf.Venue + " " + perf.PerformedAt
		}
	}
	switch claim.ObjectType {
	case "artist":
		if artist, _ := s.ArtistByID(ctx, claim.ObjectID); artist != nil {
			object = artist.Name
		}
	case "recording":
		if rec, _ := s.RecordingByID(ctx, claim.ObjectID); rec != nil {
			object = rec.ArtistName + " — " + rec.Title
		}
	case "work":
		if work, _ := s.WorkByID(ctx, claim.ObjectID); work != nil {
			object = work.Title
		}
	}
	return subject, object
}

func humanizeDispute(claim models.Claim, subject, object string) string {
	switch claim.Kind {
	case "attribution":
		return "Attribution of " + subject + " to " + object + " is " + claim.Status + "."
	case "sample":
		return "Sample claim from " + object + " to " + subject + " is " + claim.Status + "."
	case "credit":
		return "Credit " + subject + " on " + object + " is " + claim.Status + "."
	case "cover":
		return "Cover claim " + subject + " of " + object + " is " + claim.Status + "."
	case "performance":
		return "Performance claim " + subject + " for " + object + " is " + claim.Status + "."
	}
	return "Claim is " + claim.Status + "."
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return strings.TrimRight(text[:limit], " ") + "…"
}

// SeedCuratedMisquotes loads curated misquote records, generating both supporting and refuting evidence.
func (s *Store) SeedCuratedMisquotes(ctx context.Context, bundlePath, jobID string) (int, error) {
	records, err := decodeJSONFile[models.CuratedMisquoteRecord](bundlePath)
	if err != nil {
		return 0, err
	}
	imported := 0
	for _, record := range records {
		attributedArtistID, err := s.resolveOrCreateArtist(ctx, record.AttributedToName)
		if err != nil {
			return imported, err
		}
		actualArtistID := ""
		if strings.TrimSpace(record.ActuallySaidByName) != "" {
			actualArtistID, err = s.resolveOrCreateArtist(ctx, record.ActuallySaidByName)
			if err != nil {
				return imported, err
			}
		}
		quote := models.Quote{
			Text:             strings.TrimSpace(record.Text),
			ArtistID:         attributedArtistID,
			ArtistName:       record.AttributedToName,
			Tags:             record.Tags,
			ProvenanceStatus: defaultStatus(record.Status),
			ConfidenceScore:  record.ConfidenceScore,
			ProviderOrigin:   defaultProvider(record.ProviderOrigin),
			License:          record.License,
			FirstSeenAt:      time.Now().UTC().Format(time.RFC3339),
			LastVerifiedAt:   time.Now().UTC().Format(time.RFC3339),
		}
		if err := s.UpsertQuote(ctx, quote); err != nil {
			return imported, err
		}
		quoteID := search.QuoteID(attributedArtistID, search.NormalizeText(quote.Text), "")
		now := time.Now().UTC().Format(time.RFC3339)

		// Primary attribution claim (potentially disputed).
		claimID, err := s.RecordClaim(ctx, models.Claim{
			Kind:            "attribution",
			SubjectType:     "quote",
			SubjectID:       quoteID,
			ObjectType:      "artist",
			ObjectID:        attributedArtistID,
			Relation:        "attributed_to",
			Status:          defaultStatus(record.Status),
			ConfidenceScore: record.ConfidenceScore,
			ProviderOrigin:  defaultProvider(record.ProviderOrigin),
			AssertedAt:      now,
			LastVerifiedAt:  now,
			Notes:           record.Notes,
		})
		if err != nil {
			return imported, err
		}
		for _, ev := range record.SupportingEvidence {
			ev := curatedEvidenceToClaim(ev, true)
			ev.ClaimID = claimID
			if _, err := s.RecordClaimEvidence(ctx, ev); err != nil {
				return imported, err
			}
		}
		for _, ev := range record.RefutingEvidence {
			ev := curatedEvidenceToClaim(ev, false)
			ev.ClaimID = claimID
			if _, err := s.RecordClaimEvidence(ctx, ev); err != nil {
				return imported, err
			}
		}

		// Rival claim: actually said by someone else.
		if actualArtistID != "" {
			rivalClaimID, err := s.RecordClaim(ctx, models.Claim{
				Kind:            "attribution",
				SubjectType:     "quote",
				SubjectID:       quoteID,
				ObjectType:      "artist",
				ObjectID:        actualArtistID,
				Relation:        "actually_said_by",
				Status:          "source_attributed",
				ConfidenceScore: 0.9,
				ProviderOrigin:  "tanabata_curated",
				AssertedAt:      now,
				LastVerifiedAt:  now,
				Notes:           "Documented true author per Quote Investigator–style trail.",
			})
			if err != nil {
				return imported, err
			}
			if _, err := s.RecordClaimEvidence(ctx, models.ClaimEvidence{
				ClaimID:      rivalClaimID,
				Supports:     true,
				Excerpt:      "Earliest verifiable citation is attributed to " + record.ActuallySaidByName + ".",
				EvidenceKind: "manual_note",
				Weight:       1.0,
				RecordedAt:   now,
			}); err != nil {
				return imported, err
			}
		}

		if err := s.RecordIngestionAuditEvent(ctx, models.IngestionAuditEvent{
			EventID:    uuid.NewString(),
			JobID:      jobID,
			Provider:   "tanabata_curated",
			Target:     "misquote:" + quoteID,
			Action:     "record_misquote",
			Status:     "succeeded",
			OccurredAt: now,
			Details:    "supporting=" + strconv.Itoa(len(record.SupportingEvidence)) + " refuting=" + strconv.Itoa(len(record.RefutingEvidence)) + " rival=" + strconv.FormatBool(actualArtistID != ""),
		}); err != nil {
			return imported, err
		}
		imported++
	}
	return imported, nil
}

// resolveOrCreateArtist returns the artist_id for a name, creating a minimal record if missing.
func (s *Store) resolveOrCreateArtist(ctx context.Context, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", errors.New("artist name is required")
	}
	if existing, err := s.ResolveArtistID(ctx, name); err == nil && existing != "" {
		return existing, nil
	}
	artistID := search.ArtistID(name, "")
	return artistID, s.UpsertArtist(ctx, models.Artist{
		ArtistID: artistID,
		Name:     name,
		Aliases:  []string{name},
		ProviderStatus: map[string]string{
			"tanabata_curated": "imported",
		},
	})
}

func curatedEvidenceToClaim(ev models.CuratedEvidence, supports bool) models.ClaimEvidence {
	kind := strings.TrimSpace(ev.EvidenceKind)
	if kind == "" {
		kind = "manual_note"
	}
	weight := ev.Weight
	if weight == 0 {
		weight = 1.0
	}
	recordedAt := ev.RecordedAt
	if recordedAt == "" {
		recordedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return models.ClaimEvidence{
		Supports:     supports,
		Excerpt:      ev.Excerpt,
		SourceURL:    ev.SourceURL,
		ArchivedURL:  ev.ArchivedURL,
		EvidenceKind: kind,
		Weight:       weight,
		RecordedAt:   recordedAt,
	}
}

func defaultStatus(value string) string {
	if strings.TrimSpace(value) == "" {
		return "needs_review"
	}
	return value
}

func defaultProvider(value string) string {
	if strings.TrimSpace(value) == "" {
		return "tanabata_curated"
	}
	return value
}

func (s *Store) recordCuratedEvidence(ctx context.Context, claimID string, items []models.CuratedEvidence, supports bool) error {
	for _, ev := range items {
		entry := curatedEvidenceToClaim(ev, supports)
		entry.ClaimID = claimID
		if _, err := s.RecordClaimEvidence(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

// decodeJSONFile is a small helper for reading typed seed bundles.
func decodeJSONFile[T any](path string) ([]T, error) {
	content, err := os.ReadFile(path) // #nosec G304 -- caller-provided seed bundle path
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var records []T
	if err := json.Unmarshal(content, &records); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return records, nil
}
