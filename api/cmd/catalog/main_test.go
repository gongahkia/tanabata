package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/providers"
	"github.com/gongahkia/tanabata/api/internal/testutil"
)

func TestBackupCatalogCreatesIntegrityCheckedSQLiteBackup(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := seededCatalogPath(t, tempDir)
	backupPath := filepath.Join(tempDir, "backups", "catalog.sqlite")

	if err := backupCatalog(sourcePath, backupPath); err != nil {
		t.Fatalf("backupCatalog() error = %v", err)
	}
	backupInfo, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if backupInfo.Size() == 0 {
		t.Fatalf("backup is empty")
	}
	if err := verifySQLiteIntegrity(context.Background(), backupPath); err != nil {
		t.Fatalf("verify backup integrity: %v", err)
	}
	assertSQLiteCLIIntegrity(t, backupPath)
}

func TestBackupCatalogDuringIngestCommand(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "catalog.sqlite")
	backupPath := filepath.Join(tempDir, "backup.sqlite")
	cmd := exec.Command( // #nosec G204 -- test runs local cmd/ingest with temp paths
		"go", "run", "./cmd/ingest",
		"-bootstrap=true",
		"-catalog", sourcePath,
		"-legacy", writeLargeLegacyQuotes(t, tempDir, 2000),
		"-curated", testutil.WriteCuratedQuotes(t, tempDir),
		"-samples", "",
		"-works", "",
		"-performances", "",
		"-misquotes", "",
		"-name", "backup-race",
	)
	cmd.Dir = filepath.Clean(filepath.Join("..", ".."))
	cmd.Env = append(os.Environ(), providers.MusicBrainzUserAgentEnv+"=TanabataTest/1.0 ( tests@example.com )")
	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatalf("start ingest command: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	deadline := time.After(20 * time.Second)
	ingestDone := false
	var ingestErr error
backupLoop:
	for {
		if _, err := os.Stat(sourcePath); err == nil {
			if err := backupCatalog(sourcePath, backupPath); err == nil {
				break backupLoop
			}
		}
		select {
		case err := <-done:
			ingestDone = true
			ingestErr = err
			if err != nil {
				t.Fatalf("ingest command error: %v\n%s", err, output.String())
			}
			if backupErr := backupCatalog(sourcePath, backupPath); backupErr != nil {
				t.Fatalf("backupCatalog() after fast ingest error = %v", backupErr)
			}
			break backupLoop
		case <-deadline:
			_ = cmd.Process.Kill()
			t.Fatalf("timed out waiting for ingest command\n%s", output.String())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if err := verifySQLiteIntegrity(context.Background(), backupPath); err != nil {
		t.Fatalf("verify concurrent backup integrity: %v", err)
	}
	assertSQLiteCLIIntegrity(t, backupPath)
	if !ingestDone {
		select {
		case ingestErr = <-done:
		case <-time.After(20 * time.Second):
			_ = cmd.Process.Kill()
			t.Fatalf("timed out waiting for ingest command completion\n%s", output.String())
		}
	}
	if ingestErr != nil {
		t.Fatalf("ingest command error: %v\n%s", ingestErr, output.String())
	}
}

func TestExportCatalogWritesStatsAndIntegrity(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := seededCatalogPath(t, tempDir)
	exportPath := filepath.Join(tempDir, "exports", "catalog.json")

	if err := exportCatalog(context.Background(), sourcePath, exportPath); err != nil {
		t.Fatalf("exportCatalog() error = %v", err)
	}
	content, err := os.ReadFile(exportPath) // #nosec G304 -- test temp path
	if err != nil {
		t.Fatalf("ReadFile(export) error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("Unmarshal(export) error = %v", err)
	}
	if payload["exported_at"] == "" || payload["stats"] == nil || payload["integrity"] == nil {
		t.Fatalf("unexpected export payload %+v", payload)
	}
}

func TestRepairOrphansDryRunAndApply(t *testing.T) {
	ctx := context.Background()
	path := orphanedCatalogPath(t)
	expected := expectedOrphanCounts()

	store, err := catalog.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	report, err := store.IntegrityReport(ctx)
	if err != nil {
		t.Fatalf("IntegrityReport() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	for key, want := range expected {
		if got := report.Counts[key]; got != want {
			t.Fatalf("pre-repair count %s = %d, want %d in %+v", key, got, want, report.Counts)
		}
	}

	var dryRun strings.Builder
	if err := repairOrphansCommand(ctx, path, "dry-run", &dryRun); err != nil {
		t.Fatalf("repairOrphansCommand(dry-run) error = %v", err)
	}
	if output := dryRun.String(); !strings.Contains(output, "quotes_missing_artist would_delete=1") || !strings.Contains(output, "quote-missing-artist") {
		t.Fatalf("dry-run output missing target details:\n%s", output)
	}
	store, err = catalog.Open(path)
	if err != nil {
		t.Fatalf("Open(after dry-run) error = %v", err)
	}
	afterDryRun, err := store.IntegrityReport(ctx)
	if err != nil {
		t.Fatalf("IntegrityReport(after dry-run) error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close(after dry-run) error = %v", err)
	}
	for key, want := range expected {
		if got := afterDryRun.Counts[key]; got != want {
			t.Fatalf("dry-run mutated %s = %d, want %d", key, got, want)
		}
	}

	var applyOut strings.Builder
	if err := repairOrphansCommand(ctx, path, "apply", &applyOut); err != nil {
		t.Fatalf("repairOrphansCommand(apply) error = %v", err)
	}
	if output := applyOut.String(); !strings.Contains(output, "quotes_missing_artist deleted=1") {
		t.Fatalf("apply output missing delete details:\n%s", output)
	}
	store, err = catalog.Open(path)
	if err != nil {
		t.Fatalf("Open(after apply) error = %v", err)
	}
	afterApply, err := store.IntegrityReport(ctx)
	if err != nil {
		t.Fatalf("IntegrityReport(after apply) error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close(after apply) error = %v", err)
	}
	for key := range expected {
		if got := afterApply.Counts[key]; got != 0 {
			t.Fatalf("post-repair count %s = %d, want 0 in %+v", key, got, afterApply.Counts)
		}
	}
	db, err := sql.Open("sqlite", sqliteBackupDSN(path))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close(audit db) error = %v", err)
		}
	}()
	var auditEvents int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ingestion_audit_events WHERE action LIKE 'repair_orphan_%'`).Scan(&auditEvents); err != nil {
		t.Fatalf("count repair audit events: %v", err)
	}
	if auditEvents == 0 {
		t.Fatalf("expected repair audit events")
	}
}

func seededCatalogPath(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "catalog.sqlite")
	store, err := catalog.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.SeedFromLegacyJSON(ctx, testutil.WriteLegacyQuotes(t, dir)); err != nil {
		t.Fatalf("SeedFromLegacyJSON() error = %v", err)
	}
	if _, err := store.ImportCuratedQuotes(ctx, testutil.WriteCuratedQuotes(t, dir)); err != nil {
		t.Fatalf("ImportCuratedQuotes() error = %v", err)
	}
	return path
}

func orphanedCatalogPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.sqlite")
	store, err := catalog.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	db, err := sql.Open("sqlite", sqliteBackupDSN(path))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close(orphan fixture db) error = %v", err)
		}
	}()
	now := time.Now().UTC().Format(time.RFC3339)
	statements := []string{
		`PRAGMA foreign_keys = OFF;`,
		`INSERT INTO artists(artist_id, name, slug, description, bio_summary, provider_status) VALUES('artist-valid', 'Artist Valid', 'artist-valid', '', '', '{}');`,
		`INSERT INTO works(work_id, title, normalized_title) VALUES('work-valid', 'Work Valid', 'work valid');`,
		`INSERT INTO recordings(recording_id, artist_id, title, normalized_title) VALUES('recording-valid', 'artist-valid', 'Recording Valid', 'recording valid');`,
		`INSERT INTO quotes(quote_id, text, normalized_text, artist_id, source_id, provenance_status, confidence_score) VALUES('quote-valid', 'valid', 'valid', 'artist-valid', '', 'verified', 1.0);`,
		`INSERT INTO quotes(quote_id, text, normalized_text, artist_id, source_id, provenance_status, confidence_score) VALUES('quote-missing-artist', 'missing artist', 'missing artist', 'artist-missing', '', 'needs_review', 0.1);`,
		`INSERT INTO quotes(quote_id, text, normalized_text, artist_id, source_id, provenance_status, confidence_score) VALUES('quote-missing-source', 'missing source', 'missing source', 'artist-valid', 'source-missing', 'needs_review', 0.1);`,
		`INSERT INTO quote_tags(quote_id, tag) VALUES('quote-tag-missing', 'orphan');`,
		`INSERT INTO quote_evidence(quote_id, evidence, position) VALUES('quote-evidence-missing', 'orphan evidence', 0);`,
		`INSERT INTO job_items(job_item_id, job_id, provider, status, started_at) VALUES('job-item-missing-job', 'job-missing', 'test', 'queued', '` + now + `');`,
		`INSERT INTO recordings(recording_id, artist_id, title, normalized_title) VALUES('recording-missing-artist', 'artist-missing', 'Missing Artist Recording', 'missing artist recording');`,
		`INSERT INTO recordings(recording_id, artist_id, work_id, title, normalized_title) VALUES('recording-missing-work', 'artist-valid', 'work-missing', 'Missing Work Recording', 'missing work recording');`,
		`INSERT INTO samples(sample_id, source_recording_id, derivative_recording_id, kind) VALUES('sample-missing-source', 'recording-missing-source', 'recording-valid', 'direct_sample');`,
		`INSERT INTO samples(sample_id, source_recording_id, derivative_recording_id, kind) VALUES('sample-missing-derivative', 'recording-valid', 'recording-missing-derivative', 'direct_sample');`,
		`INSERT INTO work_credits(credit_id, work_id, credited_name, role) VALUES('credit-missing-work', 'work-missing', 'Missing Credit', 'composer');`,
		`INSERT INTO performances(performance_id, artist_id, performed_at) VALUES('performance-missing-artist', 'artist-missing', '` + now + `');`,
		`INSERT INTO performances(performance_id, artist_id, work_id, performed_at) VALUES('performance-missing-work', 'artist-valid', 'work-missing', '` + now + `');`,
		`INSERT INTO performances(performance_id, artist_id, recording_id, performed_at) VALUES('performance-missing-recording', 'artist-valid', 'recording-missing', '` + now + `');`,
		`INSERT INTO claims(claim_id, kind, subject_type, subject_id, object_type, object_id, relation, status, asserted_at) VALUES('claim-missing-subject', 'attribution', 'quote', 'quote-missing-subject', 'artist', 'artist-valid', 'attributed_to', 'needs_review', '` + now + `');`,
		`INSERT INTO claims(claim_id, kind, subject_type, subject_id, object_type, object_id, relation, status, asserted_at) VALUES('claim-missing-object', 'attribution', 'quote', 'quote-valid', 'artist', 'artist-missing', 'attributed_to', 'needs_review', '` + now + `');`,
		`INSERT INTO claim_evidence(evidence_id, claim_id, excerpt, evidence_kind, recorded_at) VALUES('evidence-missing-claim', 'claim-missing', 'orphan evidence', 'editorial', '` + now + `');`,
		`PRAGMA foreign_keys = ON;`,
	}
	for _, stmt := range statements {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("seed orphan fixture: %v\n%s", err, stmt)
		}
	}
	return path
}

func expectedOrphanCounts() map[string]int {
	return map[string]int{
		"quotes_missing_artist":          1,
		"quotes_missing_source":          1,
		"tags_missing_quote":             1,
		"evidence_missing_quote":         1,
		"job_items_missing_job":          1,
		"recordings_missing_artist":      1,
		"recordings_missing_work":        1,
		"samples_missing_source":         1,
		"samples_missing_derivative":     1,
		"credits_missing_work":           1,
		"performances_missing_artist":    1,
		"performances_missing_work":      1,
		"performances_missing_recording": 1,
		"claims_missing_subject":         1,
		"claims_missing_object":          1,
		"claim_evidence_missing_claim":   1,
	}
}

func writeLargeLegacyQuotes(t *testing.T, dir string, count int) string {
	t.Helper()
	base := testutil.LegacyQuotes()
	quotes := make([]models.LegacyQuote, 0, count)
	for idx := range count {
		quote := base[idx%len(base)]
		quote.Author = quote.Author + " " + strconv.Itoa(idx%32)
		quote.Text = quote.Text + " #" + strconv.Itoa(idx)
		quotes = append(quotes, quote)
	}
	path := filepath.Join(dir, "large_quotes.json")
	content, err := json.Marshal(quotes)
	if err != nil {
		t.Fatalf("marshal large quotes: %v", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write large quotes: %v", err)
	}
	return path
}

func assertSQLiteCLIIntegrity(t *testing.T, path string) {
	t.Helper()
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Logf("sqlite3 CLI unavailable: %v", err)
		return
	}
	output, err := exec.Command("sqlite3", path, "PRAGMA integrity_check;").CombinedOutput() // #nosec G204 -- test opens a temp SQLite file
	if err != nil {
		t.Fatalf("sqlite3 integrity_check error: %v\n%s", err, string(output))
	}
	if got := strings.TrimSpace(string(output)); got != "ok" {
		t.Fatalf("sqlite3 integrity_check = %q, want ok", got)
	}
}
