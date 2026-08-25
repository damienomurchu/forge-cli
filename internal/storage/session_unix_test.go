//go:build linux || darwin

package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenExistingReadOnlySession(t *testing.T) {
	path := createCurrentTestDatabase(t)
	session, err := OpenExisting(context.Background(), path, os.Geteuid(), DatabaseReadOnly)
	if err != nil {
		t.Fatalf("OpenExisting() error = %v", err)
	}
	t.Cleanup(func() { session.Close() })

	assertPragma(t, session.Database(), "foreign_keys", 1)
	assertPragma(t, session.Database(), "busy_timeout", BusyTimeoutMS)
	if _, err := session.Database().Exec(`CREATE TABLE forbidden (id INTEGER)`); err == nil {
		t.Fatalf("read-only session write succeeded")
	}
}

func TestOpenForCreationInitializesFirstRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forge", databaseFilename)
	session, err := OpenForCreation(context.Background(), path, os.Geteuid())
	if err != nil {
		t.Fatalf("OpenForCreation() error = %v", err)
	}
	t.Cleanup(func() { session.Close() })

	state, err := InspectSchema(context.Background(), session.Database())
	if err != nil {
		t.Fatalf("InspectSchema() error = %v", err)
	}
	if state.Version != LatestSchemaVersion || state.NeedsMigration {
		t.Errorf("schema state = %+v, want current", state)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file state error = %v", err)
	}
}

func TestOpenForCreationMigratesExistingEmptyDatabase(t *testing.T) {
	path := createEmptyTestDatabase(t)
	session, err := OpenForCreation(context.Background(), path, os.Geteuid())
	if err != nil {
		t.Fatalf("OpenForCreation() error = %v", err)
	}
	t.Cleanup(func() { session.Close() })
	state, err := InspectSchema(context.Background(), session.Database())
	if err != nil {
		t.Fatalf("InspectSchema() error = %v", err)
	}
	if state.Version != LatestSchemaVersion || state.NeedsMigration {
		t.Errorf("schema state = %+v, want current", state)
	}
}

func TestOpenForCreationReusesCurrentDatabase(t *testing.T) {
	path := createCurrentTestDatabase(t)
	modifyTestDatabase(t, path, func(t *testing.T, session *Session) {
		t.Helper()
		if _, err := session.Database().Exec(`CREATE TABLE preserved (value TEXT)`); err != nil {
			t.Fatalf("create preserved table error = %v", err)
		}
		if _, err := session.Database().Exec(`INSERT INTO preserved(value) VALUES ('kept')`); err != nil {
			t.Fatalf("insert preserved value error = %v", err)
		}
	})

	session, err := OpenForCreation(context.Background(), path, os.Geteuid())
	if err != nil {
		t.Fatalf("OpenForCreation() error = %v", err)
	}
	t.Cleanup(func() { session.Close() })
	var value string
	if err := session.Database().QueryRow(`SELECT value FROM preserved`).Scan(&value); err != nil {
		t.Fatalf("read preserved value error = %v", err)
	}
	if value != "kept" {
		t.Errorf("preserved value = %q, want kept", value)
	}
}

func TestOpenForCreationRejectsNewerSchema(t *testing.T) {
	path := createCurrentTestDatabase(t)
	modifyTestDatabase(t, path, func(t *testing.T, session *Session) {
		t.Helper()
		if _, err := session.Database().Exec(
			`INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)`,
			LatestSchemaVersion+1,
			"future.sql",
			"2026-08-25T00:00:00.000Z",
		); err != nil {
			t.Fatalf("insert future migration error = %v", err)
		}
	})

	session, err := OpenForCreation(context.Background(), path, os.Geteuid())
	if session != nil {
		session.Close()
		t.Fatalf("OpenForCreation() returned a session, want nil")
	}
	if err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("OpenForCreation() error = %v, want newer-schema error", err)
	}
}

func TestOpenForCreationRejectsInvalidPathBeforeFilesystemAccess(t *testing.T) {
	root := t.TempDir()
	wrongNamePath := filepath.Join(root, "missing", "other.db")
	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{name: "relative", path: "forge/forge.db", wantErr: "database path must be absolute"},
		{name: "wrong filename", path: wrongNamePath, wantErr: "database path must end with forge.db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := OpenForCreation(context.Background(), tt.path, os.Geteuid())
			if session != nil {
				session.Close()
				t.Fatalf("OpenForCreation() returned a session, want nil")
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("OpenForCreation() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
	if _, err := os.Stat(filepath.Dir(wrongNamePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("wrong-name directory state error = %v, want not exist", err)
	}
}

func TestOpenForCreationRestrictsExistingStorage(t *testing.T) {
	path := createCurrentTestDatabase(t)
	if err := os.Chmod(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("chmod data directory error = %v", err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatalf("chmod database file error = %v", err)
	}

	session, err := OpenForCreation(context.Background(), path, os.Geteuid())
	if err != nil {
		t.Fatalf("OpenForCreation() error = %v", err)
	}
	t.Cleanup(func() { session.Close() })
	directoryInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat data directory error = %v", err)
	}
	databaseInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat database file error = %v", err)
	}
	if directoryInfo.Mode().Perm() != 0o700 || databaseInfo.Mode().Perm() != 0o600 {
		t.Errorf("storage modes = %04o/%04o, want 0700/0600", directoryInfo.Mode().Perm(), databaseInfo.Mode().Perm())
	}
}

func TestOpenExistingReadWriteSession(t *testing.T) {
	path := createCurrentTestDatabase(t)
	session, err := OpenExisting(context.Background(), path, os.Geteuid(), DatabaseReadWrite)
	if err != nil {
		t.Fatalf("OpenExisting() error = %v", err)
	}
	t.Cleanup(func() { session.Close() })

	if _, err := session.Database().Exec(`CREATE TABLE writable (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("read-write session write error = %v", err)
	}
}

func TestOpenExistingClassifiesMissingStorageWithoutCreation(t *testing.T) {
	t.Run("data directory", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "forge", databaseFilename)
		session, err := OpenExisting(context.Background(), path, os.Geteuid(), DatabaseReadOnly)
		if session != nil {
			session.Close()
			t.Fatalf("OpenExisting() returned a session, want nil")
		}
		if !errors.Is(err, ErrStorageNotFound) || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("OpenExisting() error = %v, want storage-not-found and os.ErrNotExist", err)
		}
		if _, err := os.Lstat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("data directory state error = %v, want not exist", err)
		}
	})

	t.Run("database file", func(t *testing.T) {
		directoryPath := filepath.Join(t.TempDir(), "forge")
		directory, err := PrepareDataDirectory(directoryPath, os.Geteuid())
		if err != nil {
			t.Fatalf("PrepareDataDirectory() error = %v", err)
		}
		if err := directory.Close(); err != nil {
			t.Fatalf("directory.Close() error = %v", err)
		}
		path := filepath.Join(directoryPath, databaseFilename)
		session, err := OpenExisting(context.Background(), path, os.Geteuid(), DatabaseReadOnly)
		if session != nil {
			session.Close()
			t.Fatalf("OpenExisting() returned a session, want nil")
		}
		if !errors.Is(err, ErrStorageNotFound) || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("OpenExisting() error = %v, want storage-not-found and os.ErrNotExist", err)
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("database state error = %v, want not exist", err)
		}
	})
}

func TestOpenExistingRejectsMigrationRequired(t *testing.T) {
	path := createEmptyTestDatabase(t)
	session, err := OpenExisting(context.Background(), path, os.Geteuid(), DatabaseReadOnly)
	if session != nil {
		session.Close()
		t.Fatalf("OpenExisting() returned a session, want nil")
	}
	if !errors.Is(err, ErrMigrationRequired) {
		t.Fatalf("OpenExisting() error = %v, want ErrMigrationRequired", err)
	}
	if !strings.Contains(err.Error(), "schema version 0, want 1") {
		t.Errorf("OpenExisting() error = %v, want version detail", err)
	}
}

func TestOpenExistingRejectsNewerSchema(t *testing.T) {
	path := createCurrentTestDatabase(t)
	modifyTestDatabase(t, path, func(t *testing.T, session *Session) {
		t.Helper()
		if _, err := session.Database().Exec(
			`INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)`,
			LatestSchemaVersion+1,
			"future.sql",
			"2026-08-25T00:00:00.000Z",
		); err != nil {
			t.Fatalf("insert future migration error = %v", err)
		}
	})

	session, err := OpenExisting(context.Background(), path, os.Geteuid(), DatabaseReadOnly)
	if session != nil {
		session.Close()
		t.Fatalf("OpenExisting() returned a session, want nil")
	}
	if err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("OpenExisting() error = %v, want newer-schema error", err)
	}
}

func TestOpenExistingRejectsInvalidArgumentsBeforeFilesystemAccess(t *testing.T) {
	missingRoot := t.TempDir()
	invalidModePath := filepath.Join(missingRoot, "mode", databaseFilename)
	wrongNamePath := filepath.Join(missingRoot, "name", "other.db")
	tests := []struct {
		name    string
		path    string
		mode    DatabaseOpenMode
		wantErr string
	}{
		{name: "create mode", path: invalidModePath, mode: DatabaseCreate, wantErr: "open-existing storage requires read-only or read-write mode"},
		{name: "relative path", path: "forge/forge.db", mode: DatabaseReadOnly, wantErr: "database path must be absolute"},
		{name: "wrong filename", path: wrongNamePath, mode: DatabaseReadOnly, wantErr: "database path must end with forge.db"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := OpenExisting(context.Background(), tt.path, os.Geteuid(), tt.mode)
			if session != nil {
				session.Close()
				t.Fatalf("OpenExisting() returned a session, want nil")
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("OpenExisting() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
	if _, err := os.Stat(filepath.Dir(invalidModePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid-mode directory state error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Dir(wrongNamePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("wrong-name directory state error = %v, want not exist", err)
	}
}

func TestSessionCloseIsIdempotent(t *testing.T) {
	path := createCurrentTestDatabase(t)
	session, err := OpenExisting(context.Background(), path, os.Geteuid(), DatabaseReadOnly)
	if err != nil {
		t.Fatalf("OpenExisting() error = %v", err)
	}
	db := session.Database()
	if err := session.Close(); err != nil {
		t.Fatalf("first Session.Close() error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("second Session.Close() error = %v", err)
	}
	if session.Database() != nil {
		t.Errorf("Session.Database() after close = %p, want nil", session.Database())
	}
	if err := db.Ping(); err == nil {
		t.Fatalf("closed database Ping() succeeded")
	}
	var nilSession *Session
	if err := nilSession.Close(); err != nil {
		t.Fatalf("nil Session.Close() error = %v", err)
	}
}

func createCurrentTestDatabase(t *testing.T) string {
	t.Helper()
	path := createEmptyTestDatabase(t)
	directory, database, db := openRawTestDatabase(t, path, DatabaseReadWrite)
	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	closeRawTestDatabase(t, directory, database, db)
	return path
}

func createEmptyTestDatabase(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "forge", databaseFilename)
	directory, err := PrepareDataDirectory(filepath.Dir(path), os.Geteuid())
	if err != nil {
		t.Fatalf("PrepareDataDirectory() error = %v", err)
	}
	database, err := OpenDatabaseFile(directory, DatabaseCreate, os.Geteuid())
	if err != nil {
		directory.Close()
		t.Fatalf("OpenDatabaseFile() error = %v", err)
	}
	if err := database.Close(); err != nil {
		directory.Close()
		t.Fatalf("database.Close() error = %v", err)
	}
	if err := directory.Close(); err != nil {
		t.Fatalf("directory.Close() error = %v", err)
	}
	return path
}

func openRawTestDatabase(t *testing.T, path string, mode DatabaseOpenMode) (*os.File, *os.File, *sql.DB) {
	t.Helper()
	directory, err := OpenDataDirectory(filepath.Dir(path), os.Geteuid())
	if err != nil {
		t.Fatalf("OpenDataDirectory() error = %v", err)
	}
	database, err := OpenDatabaseFile(directory, mode, os.Geteuid())
	if err != nil {
		directory.Close()
		t.Fatalf("OpenDatabaseFile() error = %v", err)
	}
	db, err := OpenSQLite(context.Background(), directory, database, mode)
	if err != nil {
		database.Close()
		directory.Close()
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	return directory, database, db
}

func closeRawTestDatabase(t *testing.T, directory, database *os.File, db *sql.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("database.Close() error = %v", err)
	}
	if err := directory.Close(); err != nil {
		t.Fatalf("directory.Close() error = %v", err)
	}
}

func modifyTestDatabase(t *testing.T, path string, modify func(*testing.T, *Session)) {
	t.Helper()
	session, err := OpenExisting(context.Background(), path, os.Geteuid(), DatabaseReadWrite)
	if err != nil {
		t.Fatalf("OpenExisting() error = %v", err)
	}
	modify(t, session)
	if err := session.Close(); err != nil {
		t.Fatalf("Session.Close() error = %v", err)
	}
}
