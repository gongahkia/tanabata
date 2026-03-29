package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/providers"
)

func main() {
	var (
		allArtists  = flag.Bool("all", true, "enrich all artists currently in the catalog")
		artistName  = flag.String("artist", "", "single artist to enrich")
		catalogPath = flag.String("catalog", filepath.Join("data", "catalog.sqlite"), "path to sqlite catalog")
		legacyPath  = flag.String("legacy", filepath.Join("data", "quotes.json"), "path to legacy quotes json")
	)
	flag.Parse()

	ctx := context.Background()
	store, err := catalog.Open(*catalogPath)
	if err != nil {
		log.Fatalf("open catalog: %v", err)
	}
	defer store.Close()
	if err := store.SeedFromLegacyJSON(ctx, *legacyPath); err != nil {
		log.Fatalf("seed catalog: %v", err)
	}

	service := providers.NewService(store)
	switch {
	case *artistName != "":
		if err := service.EnrichArtist(ctx, *artistName); err != nil {
			log.Fatalf("enrich artist: %v", err)
		}
	case *allArtists:
		if err := service.EnrichExistingArtists(ctx); err != nil {
			log.Fatalf("enrich artists: %v", err)
		}
	}
}
