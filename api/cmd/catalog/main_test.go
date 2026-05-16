package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/testutil"
)

func TestBackupCatalogCopiesSQLiteFile(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := seededCatalogPath(t, tempDir)
	backupPath := filepath.Join(tempDir, "backups", "catalog.sqlite")

	if err := backupCatalog(sourcePath, backupPath); err != nil {
		t.Fatalf("backupCatalog() error = %v", err)
	}
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatalf("stat source: %v", err)
	}
	backupInfo, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if backupInfo.Size() != sourceInfo.Size() || backupInfo.Size() == 0 {
		t.Fatalf("backup size = %d source size = %d", backupInfo.Size(), sourceInfo.Size())
	}
}

func TestExportCatalogWritesStatsAndIntegrity(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := seededCatalogPath(t, tempDir)
	exportPath := filepath.Join(tempDir, "exports", "catalog.json")

	if err := exportCatalog(context.Background(), sourcePath, exportPath); err != nil {
		t.Fatalf("exportCatalog() error = %v", err)
	}
	content, err := os.ReadFile(exportPath)
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
