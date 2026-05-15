package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/providers"
)

func main() {
	var (
		bootstrap   = flag.Bool("bootstrap", false, "seed the catalog from the legacy quotes.json file")
		allArtists  = flag.Bool("all", false, "enrich all artists currently in the catalog")
		artistName  = flag.String("artist", "", "single artist to enrich")
		catalogPath = flag.String("catalog", filepath.Join("data", "catalog.sqlite"), "path to sqlite catalog")
		legacyPath  = flag.String("legacy", filepath.Join("data", "quotes.json"), "path to legacy quotes json")
		jobName     = flag.String("name", "catalog-ingestion", "job name")
	)
	flag.Parse()

	ctx := context.Background()
	store, err := catalog.Open(*catalogPath)
	if err != nil {
		log.Fatalf("open catalog: %v", err)
	}
	defer store.Close()

	jobID := uuid.NewString()
	startedAt := time.Now().UTC()
	job := models.JobRun{
		JobID:     jobID,
		Name:      *jobName,
		Scope:     jobScope(*bootstrap, *allArtists, *artistName),
		Status:    "running",
		StartedAt: startedAt.Format(time.RFC3339),
	}
	if err := store.RecordJob(ctx, job); err != nil {
		log.Fatalf("record job: %v", err)
	}

	statuses := []string{}
	service := providers.NewService(store, nil)

	if *bootstrap {
		item := newJobItem(jobID, "quotefancy", "bootstrap:"+*legacyPath)
		if err := store.RecordJobItem(ctx, item); err != nil {
			log.Fatalf("record bootstrap item: %v", err)
		}
		if err := store.SeedFromLegacyJSON(ctx, *legacyPath); err != nil {
			item.Status = "failed"
			item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			item.ErrorMessage = err.Error()
			_ = store.RecordJobItem(ctx, item)
			finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
			log.Fatalf("seed catalog: %v", err)
		}
		item.Status = "succeeded"
		item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		item.Details = "bootstrapped legacy catalog"
		if err := store.RecordJobItem(ctx, item); err != nil {
			log.Fatalf("update bootstrap item: %v", err)
		}
		statuses = append(statuses, item.Status)
	}

	if strings.TrimSpace(*artistName) != "" {
		result, err := service.EnrichArtist(ctx, *artistName)
		if err != nil {
			finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
			log.Fatalf("enrich artist: %v", err)
		}
		item := newJobItem(jobID, "artist_enrichment", result.Target)
		item.Status = result.Status
		item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		item.Details = result.Details
		if err := store.RecordJobItem(ctx, item); err != nil {
			log.Fatalf("record artist item: %v", err)
		}
		statuses = append(statuses, item.Status)
	}

	if *allArtists {
		artists, err := store.ListArtists(ctx, models.ArtistFilters{Limit: 1000})
		if err != nil {
			finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
			log.Fatalf("list artists: %v", err)
		}
		for _, artist := range artists.Data {
			result, err := service.EnrichArtist(ctx, artist.Name)
			if err != nil {
				finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
				log.Fatalf("enrich artist %s: %v", artist.Name, err)
			}
			item := newJobItem(jobID, "artist_enrichment", result.Target)
			item.Status = result.Status
			item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			item.Details = result.Details
			if err := store.RecordJobItem(ctx, item); err != nil {
				log.Fatalf("record artist item: %v", err)
			}
			statuses = append(statuses, item.Status)
		}
	}

	if err := store.RefreshSearchIndices(ctx); err != nil {
		finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
		log.Fatalf("refresh search indices: %v", err)
	}

	finalizeJob(ctx, store, job, overallJobStatus(statuses), strings.Join(statuses, ","), statuses)
}

func newJobItem(jobID, provider, target string) models.JobItem {
	return models.JobItem{
		JobItemID: uuid.NewString(),
		JobID:     jobID,
		Provider:  provider,
		Target:    target,
		Status:    "running",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func finalizeJob(ctx context.Context, store *catalog.Store, job models.JobRun, status, details string, _ []string) {
	job.Status = status
	job.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	job.Details = details
	if err := store.RecordJob(ctx, job); err != nil {
		log.Printf("update job %s: %v", job.JobID, err)
	}
}

func overallJobStatus(statuses []string) string {
	result := "succeeded"
	for _, status := range statuses {
		switch status {
		case "failed":
			return "failed"
		case "partial":
			result = "partial"
		}
	}
	return result
}

func jobScope(bootstrap, all bool, artist string) string {
	parts := []string{}
	if bootstrap {
		parts = append(parts, "bootstrap")
	}
	if all {
		parts = append(parts, "all")
	}
	if strings.TrimSpace(artist) != "" {
		parts = append(parts, "artist:"+strings.TrimSpace(artist))
	}
	return strings.Join(parts, ",")
}
