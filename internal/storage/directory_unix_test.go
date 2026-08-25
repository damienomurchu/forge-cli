//go:build linux || darwin

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenDataDirectoryDoesNotCreateMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forge")
	directory, err := OpenDataDirectory(path, os.Geteuid())
	if directory != nil {
		directory.Close()
		t.Fatalf("OpenDataDirectory() returned a handle, want nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("OpenDataDirectory() error = %v, want not exist", err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("data directory state error = %v, want not exist", err)
	}
}

func TestOpenDataDirectoryOpensAndRestrictsExistingDirectory(t *testing.T) {
	for _, mode := range []os.FileMode{0o700, 0o755} {
		t.Run(mode.String(), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "forge")
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatalf("Mkdir() error = %v", err)
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatalf("Chmod() error = %v", err)
			}

			directory, err := OpenDataDirectory(path, os.Geteuid())
			if err != nil {
				t.Fatalf("OpenDataDirectory() error = %v", err)
			}
			t.Cleanup(func() { directory.Close() })
			assertDirectoryMode(t, directory, 0o700)
		})
	}
}

func TestOpenDataDirectoryRejectsUnsafeExistingPaths(t *testing.T) {
	parent := t.TempDir()
	realDirectory := filepath.Join(parent, "real")
	if err := os.Mkdir(realDirectory, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	symlink := filepath.Join(parent, "link")
	if err := os.Symlink(realDirectory, symlink); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	regularFile := filepath.Join(parent, "file")
	if err := os.WriteFile(regularFile, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{name: "symbolic link", path: symlink, wantErr: "must not be a symbolic link"},
		{name: "regular file", path: regularFile, wantErr: "is not a directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directory, err := OpenDataDirectory(tt.path, os.Geteuid())
			if directory != nil {
				directory.Close()
				t.Fatalf("OpenDataDirectory() returned a handle, want nil")
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("OpenDataDirectory() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestOpenDataDirectoryChecksOwnershipBeforePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forge")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	directory, err := OpenDataDirectory(path, os.Geteuid()+1)
	if directory != nil {
		directory.Close()
		t.Fatalf("OpenDataDirectory() returned a handle, want nil")
	}
	if err == nil || !strings.Contains(err.Error(), "is owned by user ID") {
		t.Fatalf("OpenDataDirectory() error = %v, want ownership error", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("directory mode after rejected ownership = %04o, want 0755", got)
	}
}

func TestPrepareDataDirectoryCreatesPrivateDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forge")

	directory, err := PrepareDataDirectory(path, os.Geteuid())
	if err != nil {
		t.Fatalf("PrepareDataDirectory() error = %v", err)
	}
	t.Cleanup(func() { directory.Close() })

	assertDirectoryMode(t, directory, 0o700)
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("created path mode = %v, want directory", info.Mode())
	}
}

func TestPrepareDataDirectoryOpensExistingPrivateDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forge")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	directory, err := PrepareDataDirectory(path, os.Geteuid())
	if err != nil {
		t.Fatalf("PrepareDataDirectory() error = %v", err)
	}
	t.Cleanup(func() { directory.Close() })
	assertDirectoryMode(t, directory, 0o700)
}

func TestPrepareDataDirectoryRestrictsOwnedDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forge")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	directory, err := PrepareDataDirectory(path, os.Geteuid())
	if err != nil {
		t.Fatalf("PrepareDataDirectory() error = %v", err)
	}
	t.Cleanup(func() { directory.Close() })
	assertDirectoryMode(t, directory, 0o700)
}

func TestPrepareDataDirectoryRejectsUnsafePaths(t *testing.T) {
	parent := t.TempDir()
	realDirectory := filepath.Join(parent, "real")
	if err := os.Mkdir(realDirectory, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	symlink := filepath.Join(parent, "link")
	if err := os.Symlink(realDirectory, symlink); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	regularFile := filepath.Join(parent, "file")
	if err := os.WriteFile(regularFile, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{name: "relative", path: "forge", wantErr: "must be absolute"},
		{name: "root", path: string(filepath.Separator), wantErr: "must name a directory"},
		{name: "missing parent", path: filepath.Join(parent, "missing", "forge"), wantErr: "open data directory parent"},
		{name: "symbolic link", path: symlink, wantErr: "must not be a symbolic link"},
		{name: "regular file", path: regularFile, wantErr: "is not a directory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directory, err := PrepareDataDirectory(tt.path, os.Geteuid())
			if directory != nil {
				directory.Close()
				t.Fatalf("PrepareDataDirectory() returned a handle, want nil")
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("PrepareDataDirectory() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestPrepareDataDirectoryChecksOwnershipBeforePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forge")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	directory, err := PrepareDataDirectory(path, os.Geteuid()+1)
	if directory != nil {
		directory.Close()
		t.Fatalf("PrepareDataDirectory() returned a handle, want nil")
	}
	if err == nil || !strings.Contains(err.Error(), "is owned by user ID") {
		t.Fatalf("PrepareDataDirectory() error = %v, want ownership error", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("directory mode after rejected ownership = %04o, want 0755", got)
	}
}

func TestPrepareDataDirectoryRejectsNegativeEffectiveUID(t *testing.T) {
	directory, err := PrepareDataDirectory(filepath.Join(t.TempDir(), "forge"), -1)
	if directory != nil {
		directory.Close()
		t.Fatalf("PrepareDataDirectory() returned a handle, want nil")
	}
	if err == nil || err.Error() != "effective user ID must not be negative" {
		t.Fatalf("PrepareDataDirectory() error = %v", err)
	}
}

func assertDirectoryMode(t *testing.T, directory *os.File, want os.FileMode) {
	t.Helper()
	info, err := directory.Stat()
	if err != nil {
		t.Fatalf("directory.Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("handle mode = %v, want directory", info.Mode())
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("directory mode = %04o, want %04o", got, want)
	}
}
