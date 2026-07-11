package main

import (
	"context"
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
