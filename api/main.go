package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	httpapi "github.com/gongahkia/tanabata/api/internal/api"
	"github.com/gongahkia/tanabata/api/internal/catalog"
)

func main() {
	ctx := context.Background()
	catalogPath := envOrDefault("CATALOG_PATH", filepath.Join("data", "catalog.sqlite"))
	legacyQuotesPath := envOrDefault("LEGACY_QUOTES_PATH", filepath.Join("data", "quotes.json"))
	port := envOrDefault("PORT", "8080")

	store, err := catalog.Open(catalogPath)
	if err != nil {
		log.Fatalf("open catalog: %v", err)
	}
	defer store.Close()

	if err := store.SeedFromLegacyJSON(ctx, legacyQuotesPath); err != nil {
		log.Fatalf("seed legacy data: %v", err)
	}

	server := httpapi.NewServer(store)
	if err := server.Router().Run(":" + port); err != nil {
		log.Fatalf("start server: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
