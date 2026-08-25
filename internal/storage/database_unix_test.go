//go:build linux || darwin

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenDatabaseFileDoesNotCreateForExistingModes(t *testing.T) {
	for _, mode := range []DatabaseOpenMode{DatabaseReadOnly, DatabaseReadWrite} {
		t.Run(databaseModeName(mode), func(t *testing.T) {
			directory := openTestDataDirectory(t)
			database, err := OpenDatabaseFile(directory, mode, os.Geteuid())
			if database != nil {
				database.Close()
				t.Fatalf("OpenDatabaseFile() returned a handle, want nil")
			}
			if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("OpenDatabaseFile() error = %v, want not exist", err)
			}
			if _, err := os.Stat(filepath.Join(directory.Name(), databaseFilename)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("database state error = %v, want not exist", err)
			}
		})
	}
}

func TestOpenDatabaseFileCreatesPrivateFile(t *testing.T) {
	directory := openTestDataDirectory(t)
	database, err := OpenDatabaseFile(directory, DatabaseCreate, os.Geteuid())
	if err != nil {
		t.Fatalf("OpenDatabaseFile() error = %v", err)
	}
	t.Cleanup(func() { database.Close() })

	assertDatabaseMode(t, database, 0o600)
	if _, err := database.WriteString("sqlite"); err != nil {
		t.Fatalf("database.WriteString() error = %v", err)
	}
}

func TestOpenDatabaseFileHonorsAccessMode(t *testing.T) {
	directory := openTestDataDirectory(t)
	path := filepath.Join(directory.Name(), databaseFilename)
	if err := os.WriteFile(path, []byte("sqlite"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	readOnly, err := OpenDatabaseFile(directory, DatabaseReadOnly, os.Geteuid())
	if err != nil {
		t.Fatalf("OpenDatabaseFile(read only) error = %v", err)
	}
	if _, err := readOnly.WriteString("write"); err == nil {
		readOnly.Close()
		t.Fatalf("read-only database write succeeded")
	}
	if err := readOnly.Close(); err != nil {
		t.Fatalf("readOnly.Close() error = %v", err)
	}

	readWrite, err := OpenDatabaseFile(directory, DatabaseReadWrite, os.Geteuid())
	if err != nil {
		t.Fatalf("OpenDatabaseFile(read write) error = %v", err)
	}
	t.Cleanup(func() { readWrite.Close() })
	if _, err := readWrite.WriteString("write"); err != nil {
		t.Fatalf("read-write database write error = %v", err)
	}
}

func TestOpenDatabaseFileRestrictsOwnedFile(t *testing.T) {
	directory := openTestDataDirectory(t)
	path := filepath.Join(directory.Name(), databaseFilename)
	if err := os.WriteFile(path, nil, 0o666); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	database, err := OpenDatabaseFile(directory, DatabaseReadOnly, os.Geteuid())
	if err != nil {
		t.Fatalf("OpenDatabaseFile() error = %v", err)
	}
	t.Cleanup(func() { database.Close() })
	assertDatabaseMode(t, database, 0o600)
}

func TestOpenDatabaseFileRejectsUnsafeFiles(t *testing.T) {
	tests := []struct {
		name    string
		create  func(t *testing.T, path string)
		wantErr string
	}{
		{
			name: "symbolic link",
			create: func(t *testing.T, path string) {
				t.Helper()
				target := filepath.Join(filepath.Dir(path), "target")
				if err := os.WriteFile(target, nil, 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatalf("Symlink() error = %v", err)
				}
			},
			wantErr: "must not be a symbolic link",
		},
		{
			name: "directory",
			create: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatalf("Mkdir() error = %v", err)
				}
			},
			wantErr: "is not a regular file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directory := openTestDataDirectory(t)
			tt.create(t, filepath.Join(directory.Name(), databaseFilename))
			database, err := OpenDatabaseFile(directory, DatabaseCreate, os.Geteuid())
			if database != nil {
				database.Close()
				t.Fatalf("OpenDatabaseFile() returned a handle, want nil")
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("OpenDatabaseFile() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestOpenDatabaseFileChecksOwnershipBeforePermissions(t *testing.T) {
	directory := openTestDataDirectory(t)
	path := filepath.Join(directory.Name(), databaseFilename)
	if err := os.WriteFile(path, nil, 0o666); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	database, err := OpenDatabaseFile(directory, DatabaseReadWrite, os.Geteuid()+1)
	if database != nil {
		database.Close()
		t.Fatalf("OpenDatabaseFile() returned a handle, want nil")
	}
	if err == nil || !strings.Contains(err.Error(), "is owned by user ID") {
		t.Fatalf("OpenDatabaseFile() error = %v, want ownership error", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o666 {
		t.Errorf("database mode after rejected ownership = %04o, want 0666", got)
	}
}

func TestOpenDatabaseFileRejectsInvalidArguments(t *testing.T) {
	directory := openTestDataDirectory(t)
	tests := []struct {
		name    string
		dir     *os.File
		mode    DatabaseOpenMode
		uid     int
		wantErr string
	}{
		{name: "nil directory", mode: DatabaseReadOnly, uid: os.Geteuid(), wantErr: "data directory handle is required"},
		{name: "negative user ID", dir: directory, mode: DatabaseReadOnly, uid: -1, wantErr: "effective user ID must not be negative"},
		{name: "invalid mode", dir: directory, mode: DatabaseOpenMode(255), uid: os.Geteuid(), wantErr: "invalid database open mode 255"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database, err := OpenDatabaseFile(tt.dir, tt.mode, tt.uid)
			if database != nil {
				database.Close()
				t.Fatalf("OpenDatabaseFile() returned a handle, want nil")
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("OpenDatabaseFile() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func openTestDataDirectory(t *testing.T) *os.File {
	t.Helper()
	directory, err := PrepareDataDirectory(filepath.Join(t.TempDir(), "forge"), os.Geteuid())
	if err != nil {
		t.Fatalf("PrepareDataDirectory() error = %v", err)
	}
	t.Cleanup(func() { directory.Close() })
	return directory
}

func assertDatabaseMode(t *testing.T, database *os.File, want os.FileMode) {
	t.Helper()
	info, err := database.Stat()
	if err != nil {
		t.Fatalf("database.Stat() error = %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("database mode = %v, want regular file", info.Mode())
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("database mode = %04o, want %04o", got, want)
	}
}

func databaseModeName(mode DatabaseOpenMode) string {
	switch mode {
	case DatabaseReadOnly:
		return "read only"
	case DatabaseReadWrite:
		return "read write"
	default:
		return "unknown"
	}
}
