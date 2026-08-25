//go:build linux

package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenExistingClosesResourcesAfterSchemaFailure(t *testing.T) {
	path := createEmptyTestDatabase(t)
	directoryPath := filepath.Dir(path)
	if got := countOpenDescriptorsBelow(t, directoryPath); got != 0 {
		t.Fatalf("open descriptors before test = %d, want 0", got)
	}

	session, err := OpenExisting(context.Background(), path, os.Geteuid(), DatabaseReadOnly)
	if session != nil {
		session.Close()
		t.Fatalf("OpenExisting() returned a session, want nil")
	}
	if err == nil {
		t.Fatalf("OpenExisting() error = nil, want schema failure")
	}
	if got := countOpenDescriptorsBelow(t, directoryPath); got != 0 {
		t.Fatalf("open descriptors after failure = %d, want 0", got)
	}
}

func TestOpenForCreationClosesResourcesAfterMigrationFailure(t *testing.T) {
	path := createEmptyTestDatabase(t)
	directory, database, db := openRawTestDatabase(t, path, DatabaseReadWrite)
	if _, err := db.Exec(`CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		applied_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create conflicting migration table error = %v", err)
	}
	closeRawTestDatabase(t, directory, database, db)
	directoryPath := filepath.Dir(path)

	session, err := OpenForCreation(context.Background(), path, os.Geteuid())
	if session != nil {
		session.Close()
		t.Fatalf("OpenForCreation() returned a session, want nil")
	}
	if err == nil || !strings.Contains(err.Error(), "migrate database for creation") {
		t.Fatalf("OpenForCreation() error = %v, want migration error", err)
	}
	if got := countOpenDescriptorsBelow(t, directoryPath); got != 0 {
		t.Fatalf("open descriptors after migration failure = %d, want 0", got)
	}
}

func countOpenDescriptorsBelow(t *testing.T, directory string) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatalf("ReadDir(/proc/self/fd) error = %v", err)
	}
	directory = filepath.Clean(directory)
	count := 0
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err != nil {
			continue
		}
		target = strings.TrimSuffix(target, " (deleted)")
		if target == directory || strings.HasPrefix(target, directory+string(filepath.Separator)) {
			count++
		}
	}
	return count
}
