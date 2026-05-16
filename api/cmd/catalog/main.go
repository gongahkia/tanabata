package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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

func backupCatalog(sourcePath, destinationPath string) error {
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer destination.Close()

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
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destinationPath, append(content, '\n'), 0o644)
}

func usage() string {
	return fmt.Sprintf("catalog -catalog data/catalog.sqlite [-backup path] [-export path]")
}
