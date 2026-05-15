package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gongahkia/tanabata/api/internal/catalog"
	"github.com/gongahkia/tanabata/api/internal/models"
)

func TestJobHelpers(t *testing.T) {
	item := newJobItem("job-1", "wikiquote", "Frank Ocean")
	if item.JobID != "job-1" || item.Provider != "wikiquote" || item.Status != "running" || item.JobItemID == "" || item.StartedAt == "" {
		t.Fatalf("unexpected job item %+v", item)
	}

	if got := overallJobStatus([]string{"succeeded", "partial"}); got != "partial" {
		t.Fatalf("overallJobStatus() = %q, want partial", got)
	}
	if got := overallJobStatus([]string{"succeeded", "failed"}); got != "failed" {
		t.Fatalf("overallJobStatus() = %q, want failed", got)
	}
	if got := overallJobStatus(nil); got != "succeeded" {
		t.Fatalf("overallJobStatus() = %q, want succeeded", got)
	}

	if got := jobScope(true, true, " Frank Ocean "); got != "bootstrap,all,artist:Frank Ocean" {
		t.Fatalf("jobScope() = %q", got)
	}
}

func TestFinalizeJobPersistsState(t *testing.T) {
	tempDir := t.TempDir()
	store, err := catalog.Open(filepath.Join(tempDir, "catalog.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	job := models.JobRun{
		JobID:     "job-1",
		Name:      "catalog-refresh",
		Status:    "running",
		StartedAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
	}
	if err := store.RecordJob(ctx, job); err != nil {
		t.Fatalf("RecordJob() error = %v", err)
	}

	finalizeJob(ctx, store, job, "partial", "bootstrap,partial", nil)

	stored, err := store.JobByID(ctx, "job-1")
	if err != nil || stored == nil {
		t.Fatalf("JobByID() err=%v job=%+v", err, stored)
	}
	if stored.Status != "partial" || stored.Details != "bootstrap,partial" || stored.FinishedAt == "" {
		t.Fatalf("unexpected stored job %+v", stored)
	}
}
