package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
	"github.com/gongahkia/tanabata/api/internal/providers"
)

type options struct {
	bootstrap        bool
	allArtists       bool
	artistName       string
	catalogPath      string
	legacyPath       string
	curatedPath      string
	samplesPath      string
	worksPath        string
	performancesPath string
	misquotesPath    string
	jobName          string
}

type enrichmentService interface {
	EnrichArtist(ctx context.Context, query string) (providers.EnrichResult, error)
}

func main() {
	var (
		bootstrap        = flag.Bool("bootstrap", false, "seed the catalog from the legacy quotes.json file")
		allArtists       = flag.Bool("all", false, "enrich all artists currently in the catalog")
		artistName       = flag.String("artist", "", "single artist to enrich")
		catalogPath      = flag.String("catalog", filepath.Join("data", "catalog.sqlite"), "path to sqlite catalog")
		legacyPath       = flag.String("legacy", filepath.Join("data", "quotes.json"), "path to legacy quotes json")
		curatedPath      = flag.String("curated", filepath.Join("data", "curated_quotes.json"), "path to curated quotes json")
		samplesPath      = flag.String("samples", filepath.Join("data", "curated_samples.json"), "path to curated samples json")
		worksPath        = flag.String("works", filepath.Join("data", "curated_works.json"), "path to curated works/credits/covers json")
		performancesPath = flag.String("performances", filepath.Join("data", "curated_performances.json"), "path to curated performances json")
		misquotesPath    = flag.String("misquotes", filepath.Join("data", "curated_misquotes.json"), "path to curated misquotes json")
		jobName          = flag.String("name", "catalog-ingestion", "job name")
	)
	flag.Parse()

	ctx := context.Background()
	opts := options{
		bootstrap:        *bootstrap,
		allArtists:       *allArtists,
		artistName:       *artistName,
		catalogPath:      *catalogPath,
		legacyPath:       *legacyPath,
		curatedPath:      *curatedPath,
		samplesPath:      *samplesPath,
		worksPath:        *worksPath,
		performancesPath: *performancesPath,
		misquotesPath:    *misquotesPath,
		jobName:          *jobName,
	}
	store, err := catalog.Open(opts.catalogPath)
	if err != nil {
		log.Fatalf("open catalog: %v", err)
	}
	defer store.Close()

	if err := run(ctx, opts, store, providers.NewService(store, nil)); err != nil {
		log.Fatalf("ingest catalog: %v", err)
	}
}

func run(ctx context.Context, opts options, store *catalog.Store, service enrichmentService) error {
	jobID := uuid.NewString()
	startedAt := time.Now().UTC()
	job := models.JobRun{
		JobID:     jobID,
		Name:      opts.jobName,
		Scope:     jobScope(opts.bootstrap, opts.allArtists, opts.artistName),
		Status:    "running",
		StartedAt: startedAt.Format(time.RFC3339),
	}
	if err := store.RecordJob(ctx, job); err != nil {
		return fmt.Errorf("record job: %w", err)
	}

	statuses := []string{}

	if opts.bootstrap {
		item := newJobItem(jobID, "quotefancy", "bootstrap:"+opts.legacyPath)
		if err := store.RecordJobItem(ctx, item); err != nil {
			return fmt.Errorf("record bootstrap item: %w", err)
		}
		if err := store.SeedFromLegacyJSON(ctx, opts.legacyPath); err != nil {
			item.Status = "failed"
			item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			item.ErrorMessage = err.Error()
			_ = store.RecordJobItem(ctx, item)
			finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
			return fmt.Errorf("seed legacy catalog: %w", err)
		}
		item.Status = "succeeded"
		item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		item.Details = "bootstrapped legacy catalog"
		if err := store.RecordJobItem(ctx, item); err != nil {
			return fmt.Errorf("update bootstrap item: %w", err)
		}
		statuses = append(statuses, item.Status)

		if strings.TrimSpace(opts.curatedPath) != "" {
			if _, err := os.Stat(opts.curatedPath); err == nil {
				if err := importCuratedBundle(ctx, store, jobID, opts.curatedPath, &statuses); err != nil {
					finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
					return err
				}
			} else if !os.IsNotExist(err) {
				finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
				return fmt.Errorf("stat curated quotes: %w", err)
			}
		}

		lineageBundles := []lineageBundle{
			{name: "samples", path: opts.samplesPath, provider: "tanabata_curated", seed: func(ctx context.Context) (int, error) {
				return store.SeedCuratedSamples(ctx, opts.samplesPath, jobID)
			}},
			{name: "works", path: opts.worksPath, provider: "tanabata_curated", seed: func(ctx context.Context) (int, error) {
				return store.SeedCuratedWorks(ctx, opts.worksPath, jobID)
			}},
			{name: "performances", path: opts.performancesPath, provider: "tanabata_curated", seed: func(ctx context.Context) (int, error) {
				return store.SeedCuratedPerformances(ctx, opts.performancesPath, jobID)
			}},
			{name: "misquotes", path: opts.misquotesPath, provider: "tanabata_curated", seed: func(ctx context.Context) (int, error) {
				return store.SeedCuratedMisquotes(ctx, opts.misquotesPath, jobID)
			}},
		}
		for _, bundle := range lineageBundles {
			if err := importLineageBundle(ctx, store, jobID, bundle, &statuses); err != nil {
				finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
				return err
			}
		}
	}

	if strings.TrimSpace(opts.artistName) != "" {
		result, err := service.EnrichArtist(ctx, opts.artistName)
		if err != nil {
			finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
			return fmt.Errorf("enrich artist: %w", err)
		}
		item := newJobItem(jobID, "artist_enrichment", result.Target)
		item.Status = result.Status
		item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		item.Details = result.Details
		if err := store.RecordJobItem(ctx, item); err != nil {
			return fmt.Errorf("record artist item: %w", err)
		}
		statuses = append(statuses, item.Status)
	}

	if opts.allArtists {
		artists, err := store.ListArtists(ctx, models.ArtistFilters{Limit: 1000})
		if err != nil {
			finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
			return fmt.Errorf("list artists: %w", err)
		}
		for _, artist := range artists.Data {
			result, err := service.EnrichArtist(ctx, artist.Name)
			if err != nil {
				finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
				return fmt.Errorf("enrich artist %s: %w", artist.Name, err)
			}
			item := newJobItem(jobID, "artist_enrichment", result.Target)
			item.Status = result.Status
			item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			item.Details = result.Details
			if err := store.RecordJobItem(ctx, item); err != nil {
				return fmt.Errorf("record artist item: %w", err)
			}
			statuses = append(statuses, item.Status)
		}
	}

	if err := store.RefreshSearchIndices(ctx); err != nil {
		finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
		return fmt.Errorf("refresh search indices: %w", err)
	}
	if err := store.UpdateActiveProviders(ctx); err != nil {
		finalizeJob(ctx, store, job, "failed", err.Error(), statuses)
		return fmt.Errorf("update active providers: %w", err)
	}

	finalizeJob(ctx, store, job, overallJobStatus(statuses), strings.Join(statuses, ","), statuses)
	return nil
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

type lineageBundle struct {
	name     string
	path     string
	provider string
	seed     func(ctx context.Context) (int, error)
}

func importLineageBundle(ctx context.Context, store *catalog.Store, jobID string, bundle lineageBundle, statuses *[]string) error {
	if strings.TrimSpace(bundle.path) == "" {
		return nil
	}
	if _, err := os.Stat(bundle.path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", bundle.name, err)
	}
	item := newJobItem(jobID, bundle.provider, "bootstrap:"+bundle.path)
	if err := store.RecordJobItem(ctx, item); err != nil {
		return fmt.Errorf("record %s item: %w", bundle.name, err)
	}
	startedAt := time.Now().UTC().Add(-time.Second)
	imported, err := bundle.seed(ctx)
	if err != nil {
		item.Status = "failed"
		item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		item.ErrorMessage = err.Error()
		_ = store.RecordJobItem(ctx, item)
		_ = store.RecordProviderRun(ctx, catalog.ProviderRun{
			RunID:      uuid.NewString(),
			Provider:   bundle.provider,
			Status:     "failed",
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
			Details:    err.Error(),
		})
		return fmt.Errorf("import %s: %w", bundle.name, err)
	}
	item.Status = "succeeded"
	item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	item.Details = fmt.Sprintf("imported=%d %s records", imported, bundle.name)
	if err := store.RecordJobItem(ctx, item); err != nil {
		return fmt.Errorf("update %s item: %w", bundle.name, err)
	}
	if err := store.RecordProviderRun(ctx, catalog.ProviderRun{
		RunID:      uuid.NewString(),
		Provider:   bundle.provider,
		Status:     "success",
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
		Details:    fmt.Sprintf("imported=%d %s records", imported, bundle.name),
	}); err != nil {
		return fmt.Errorf("record %s provider run: %w", bundle.name, err)
	}
	*statuses = append(*statuses, item.Status)
	return nil
}

func importCuratedBundle(ctx context.Context, store *catalog.Store, jobID, bundlePath string, statuses *[]string) error {
	item := newJobItem(jobID, "tanabata_curated", "bootstrap:"+bundlePath)
	if err := store.RecordJobItem(ctx, item); err != nil {
		return fmt.Errorf("record curated bootstrap item: %w", err)
	}
	startedAt := time.Now().UTC().Add(-time.Second)
	imported, err := store.ImportCuratedQuotes(ctx, bundlePath)
	if err != nil {
		item.Status = "failed"
		item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		item.ErrorMessage = err.Error()
		_ = store.RecordJobItem(ctx, item)
		_ = store.RecordProviderRun(ctx, catalog.ProviderRun{
			RunID:      uuid.NewString(),
			Provider:   "tanabata_curated",
			Status:     "failed",
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
			Details:    err.Error(),
		})
		return fmt.Errorf("import curated quotes: %w", err)
	}
	item.Status = "succeeded"
	item.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	item.Details = fmt.Sprintf("imported=%d curated quotes", imported)
	if err := store.RecordJobItem(ctx, item); err != nil {
		return fmt.Errorf("update curated bootstrap item: %w", err)
	}
	if err := store.RecordProviderRun(ctx, catalog.ProviderRun{
		RunID:      uuid.NewString(),
		Provider:   "tanabata_curated",
		Status:     "success",
		StartedAt:  startedAt,
		FinishedAt: time.Now().UTC(),
		Details:    fmt.Sprintf("imported=%d curated quotes", imported),
	}); err != nil {
		return fmt.Errorf("record curated provider run: %w", err)
	}
	*statuses = append(*statuses, item.Status)
	return nil
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
