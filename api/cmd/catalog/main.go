package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gongahkia/tanabata/api/internal/catalog"
)

func main() {
	var (
		catalogPath = flag.String("catalog", filepath.Join("data", "catalog.sqlite"), "path to sqlite catalog")
		backupPath  = flag.String("backup", "", "write a byte-for-byte SQLite backup to this path")
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
	source, err := os.Open(sourcePath) // #nosec G304 -- caller-provided backup source path
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := source.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) // #nosec G304 -- caller-provided backup target path
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := destination.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	if _, err := io.Copy(destination, source); err != nil {
		return err
	}
	return destination.Sync()
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
