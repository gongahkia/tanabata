package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAcquireIngestLockRejectsConcurrentHolder(t *testing.T) {
	catalogPath := filepath.Join(t.TempDir(), "catalog.sqlite")
	lock, err := acquireIngestLock(catalogPath, false)
	if err != nil {
		t.Fatalf("acquireIngestLock(first) error = %v", err)
	}
	cleanupIngestLock(t, lock)

	_, err = acquireIngestLock(catalogPath, false)
	var heldErr *ingestLockHeldError
	if !errors.As(err, &heldErr) {
		t.Fatalf("acquireIngestLock(second) error = %v, want ingestLockHeldError", err)
	}
	if heldErr.HolderPID != os.Getpid() {
		t.Fatalf("holder pid = %d, want %d", heldErr.HolderPID, os.Getpid())
	}
	if !strings.Contains(err.Error(), "already_running") {
		t.Fatalf("lock error missing already_running: %v", err)
	}
}

func TestIngestLockCloseReleasesAndRemovesLockFile(t *testing.T) {
	catalogPath := filepath.Join(t.TempDir(), "catalog.sqlite")
	lock, err := acquireIngestLock(catalogPath, false)
	if err != nil {
		t.Fatalf("acquireIngestLock() error = %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := os.Stat(ingestLockPath(catalogPath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock file stat error = %v, want not exist", err)
	}
	lock, err = acquireIngestLock(catalogPath, false)
	if err != nil {
		t.Fatalf("acquireIngestLock(after close) error = %v", err)
	}
	cleanupIngestLock(t, lock)
}

func TestAcquireIngestLockForceUnlockRemovesStaleFile(t *testing.T) {
	catalogPath := filepath.Join(t.TempDir(), "catalog.sqlite")
	lockPath := ingestLockPath(catalogPath)
	if err := os.WriteFile(lockPath, []byte("999999\n"), 0o600); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	lock, err := acquireIngestLock(catalogPath, true)
	if err != nil {
		t.Fatalf("acquireIngestLock(force) error = %v", err)
	}
	cleanupIngestLock(t, lock)
	data, err := os.ReadFile(lockPath) // #nosec G304 -- test-owned lock path
	if err != nil {
		t.Fatalf("read lock path: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != strconv.Itoa(os.Getpid()) {
		t.Fatalf("lock pid = %q, want current pid", got)
	}
}

func cleanupIngestLock(t *testing.T, lock *ingestLock) {
	t.Helper()
	t.Cleanup(func() {
		if err := lock.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
}
