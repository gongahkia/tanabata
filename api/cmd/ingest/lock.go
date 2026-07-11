package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

type ingestLock struct {
	path string
	file *os.File
	pid  int
}

type ingestLockHeldError struct {
	Path      string
	HolderPID int
}

func (e *ingestLockHeldError) Error() string {
	if e.HolderPID > 0 {
		return fmt.Sprintf("already_running: ingest lock %s held by pid %d", e.Path, e.HolderPID)
	}
	return fmt.Sprintf("already_running: ingest lock %s held by unknown pid", e.Path)
}

func acquireIngestLock(catalogPath string, forceUnlock bool) (*ingestLock, error) {
	lockPath := ingestLockPath(catalogPath)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o750); err != nil {
		return nil, fmt.Errorf("create ingest lock directory: %w", err)
	}
	if forceUnlock {
		if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("force unlock ingest lock: %w", err)
		}
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- lock path is derived from the configured catalog path
	if err != nil {
		return nil, fmt.Errorf("open ingest lock: %w", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		holderPID := readIngestLockPID(file)
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, &ingestLockHeldError{Path: lockPath, HolderPID: holderPID}
		}
		return nil, fmt.Errorf("acquire ingest lock: %w", err)
	}
	pid := os.Getpid()
	if err := writeIngestLockPID(file, pid); err != nil {
		_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
		_ = file.Close()
		return nil, err
	}
	return &ingestLock{path: lockPath, file: file, pid: pid}, nil
}

func ingestLockPath(catalogPath string) string {
	return catalogPath + ".ingest.lock"
}

func readIngestLockPID(file *os.File) int {
	if _, err := file.Seek(0, 0); err != nil {
		return 0
	}
	data, err := io.ReadAll(io.LimitReader(file, 64))
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid
}

func writeIngestLockPID(file *os.File, pid int) error {
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("truncate ingest lock: %w", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("seek ingest lock: %w", err)
	}
	if _, err := fmt.Fprintf(file, "%d\n", pid); err != nil {
		return fmt.Errorf("write ingest lock pid: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync ingest lock: %w", err)
	}
	return nil
}

func (l *ingestLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	closeErr := l.file.Close()
	removeErr := l.removeIfOwned()
	return errors.Join(unlockErr, closeErr, removeErr)
}

func (l *ingestLock) removeIfOwned() error {
	data, err := os.ReadFile(l.path) // #nosec G304 -- lock path is derived from the configured catalog path
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(string(data)) != strconv.Itoa(l.pid) {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
