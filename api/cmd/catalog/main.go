package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gongahkia/tanabata/api/internal/catalog"
)

const sqliteBackupPragmaSuffix = "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"

func main() {
	var (
		catalogPath = flag.String("catalog", filepath.Join("data", "catalog.sqlite"), "path to sqlite catalog")
		backupPath  = flag.String("backup", "", "write a consistent SQLite backup to this path")
		exportPath  = flag.String("export", "", "write catalog metadata JSON to this path")
	)
	flag.Parse()

	if *backupPath == "" && *exportPath == "" {
		log.Fatal("one of -backup or -export is required")
	}
	ctx := context.Background()
	if *backupPath != "" {
		if err := backupCatalog(*catalogPath, *backupPath); err != nil {
			log.Fatalf("backup catalog: %v", err)
		}
	}
	if *exportPath != "" {
		if err := exportCatalog(ctx, *catalogPath, *exportPath); err != nil {
			log.Fatalf("export catalog: %v", err)
		}
	}
}

func backupCatalog(sourcePath, destinationPath string) (err error) {
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o750); err != nil {
		return err
	}
	sourceAbs, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}
	destinationAbs, err := filepath.Abs(destinationPath)
	if err != nil {
		return err
	}
	if sourceAbs == destinationAbs {
		return fmt.Errorf("backup destination must differ from source: %s", destinationPath)
	}
	if err := removeSQLiteBackupFiles(destinationPath); err != nil {
		return err
	}
	ctx := context.Background()
	db, err := sql.Open("sqlite", sqliteBackupDSN(sourcePath))
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	if _, err := db.ExecContext(ctx, `VACUUM main INTO ?`, destinationPath); err != nil {
		return err
	}
	return verifySQLiteIntegrity(ctx, destinationPath)
}

func removeSQLiteBackupFiles(path string) error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(path + suffix); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func sqliteBackupDSN(path string) string {
	if strings.Contains(path, "?") {
		return path + "&" + strings.TrimPrefix(sqliteBackupPragmaSuffix, "?")
	}
	return path + sqliteBackupPragmaSuffix
}

func verifySQLiteIntegrity(ctx context.Context, path string) (err error) {
	db, err := sql.Open("sqlite", sqliteBackupDSN(path))
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	var result string
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("sqlite integrity_check failed for %s: %s", path, result)
	}
	return nil
}

func exportCatalog(ctx context.Context, catalogPath, destinationPath string) error {
	store, err := catalog.Open(catalogPath)
	if err != nil {
		return err
	}
	defer store.Close()

	stats, err := store.Stats(ctx)
	if err != nil {
		return err
	}
	integrity, err := store.IntegrityReport(ctx)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"exported_at": time.Now().UTC().Format(time.RFC3339),
		"stats":       stats,
		"integrity":   integrity,
	}
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o750); err != nil {
		return err
	}
	return os.WriteFile(destinationPath, append(content, '\n'), 0o600)
}
