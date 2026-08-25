//go:build linux || darwin

package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSQLiteConfiguresSingleConnection(t *testing.T) {
	directory, database := openTestDatabaseFile(t, DatabaseCreate)
	db, err := OpenSQLite(context.Background(), directory, database, DatabaseCreate)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if got := db.Stats().MaxOpenConnections; got != 1 {
		t.Errorf("MaxOpenConnections = %d, want 1", got)
	}
	assertPragma(t, db, "foreign_keys", 1)
	assertPragma(t, db, "busy_timeout", BusyTimeoutMS)
	if _, err := db.Exec(`CREATE TABLE configured (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create table error = %v", err)
	}
}

func TestOpenSQLiteReadOnlyCannotWrite(t *testing.T) {
	directory, database := openTestDatabaseFile(t, DatabaseCreate)
	writable, err := OpenSQLite(context.Background(), directory, database, DatabaseCreate)
	if err != nil {
		t.Fatalf("OpenSQLite(create) error = %v", err)
	}
	if _, err := writable.Exec(`CREATE TABLE existing (id INTEGER PRIMARY KEY)`); err != nil {
		writable.Close()
		t.Fatalf("create table error = %v", err)
	}
	if err := writable.Close(); err != nil {
		t.Fatalf("writable.Close() error = %v", err)
	}

	readOnlyFile, err := OpenDatabaseFile(directory, DatabaseReadOnly, os.Geteuid())
	if err != nil {
		t.Fatalf("OpenDatabaseFile(read only) error = %v", err)
	}
	t.Cleanup(func() { readOnlyFile.Close() })
	readOnly, err := OpenSQLite(context.Background(), directory, readOnlyFile, DatabaseReadOnly)
	if err != nil {
		t.Fatalf("OpenSQLite(read only) error = %v", err)
	}
	t.Cleanup(func() { readOnly.Close() })

	assertPragma(t, readOnly, "foreign_keys", 1)
	assertPragma(t, readOnly, "busy_timeout", BusyTimeoutMS)
	if _, err := readOnly.Exec(`CREATE TABLE forbidden (id INTEGER PRIMARY KEY)`); err == nil {
		t.Fatalf("read-only write succeeded")
	}
}

func TestOpenSQLiteRejectsChangedDatabasePath(t *testing.T) {
	t.Run("symbolic link", func(t *testing.T) {
		directory, database := openTestDatabaseFile(t, DatabaseCreate)
		path := filepath.Join(directory.Name(), databaseFilename)
		original := filepath.Join(directory.Name(), "original.db")
		if err := os.Rename(path, original); err != nil {
			t.Fatalf("Rename() error = %v", err)
		}
		if err := os.Symlink(original, path); err != nil {
			t.Fatalf("Symlink() error = %v", err)
		}

		db, err := OpenSQLite(context.Background(), directory, database, DatabaseReadOnly)
		if db != nil {
			db.Close()
			t.Fatalf("OpenSQLite() returned a database, want nil")
		}
		if err == nil || err.Error() != "sqlite database file changed during opening" {
			t.Fatalf("OpenSQLite() error = %v", err)
		}
	})

	t.Run("different regular file", func(t *testing.T) {
		directory, database := openTestDatabaseFile(t, DatabaseCreate)
		path := filepath.Join(directory.Name(), databaseFilename)
		if err := os.Rename(path, filepath.Join(directory.Name(), "original.db")); err != nil {
			t.Fatalf("Rename() error = %v", err)
		}
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		db, err := OpenSQLite(context.Background(), directory, database, DatabaseReadOnly)
		if db != nil {
			db.Close()
			t.Fatalf("OpenSQLite() returned a database, want nil")
		}
		if err == nil || err.Error() != "sqlite database file changed during opening" {
			t.Fatalf("OpenSQLite() error = %v", err)
		}
	})
}

func TestOpenSQLiteRejectsInvalidArguments(t *testing.T) {
	directory, database := openTestDatabaseFile(t, DatabaseCreate)
	tests := []struct {
		name    string
		dir     *os.File
		file    *os.File
		mode    DatabaseOpenMode
		wantErr string
	}{
		{name: "nil directory", file: database, mode: DatabaseReadOnly, wantErr: "data directory handle is required"},
		{name: "nil database", dir: directory, mode: DatabaseReadOnly, wantErr: "database file handle is required"},
		{name: "invalid mode", dir: directory, file: database, mode: DatabaseOpenMode(255), wantErr: "invalid database open mode 255"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := OpenSQLite(context.Background(), tt.dir, tt.file, tt.mode)
			if db != nil {
				db.Close()
				t.Fatalf("OpenSQLite() returned a database, want nil")
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("OpenSQLite() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func openTestDatabaseFile(t *testing.T, mode DatabaseOpenMode) (*os.File, *os.File) {
	t.Helper()
	directory := openTestDataDirectory(t)
	database, err := OpenDatabaseFile(directory, mode, os.Geteuid())
	if err != nil {
		t.Fatalf("OpenDatabaseFile() error = %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return directory, database
}

func assertPragma(t *testing.T, db interface{ QueryRow(string, ...any) *sql.Row }, name string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow("PRAGMA " + name).Scan(&got); err != nil {
		t.Fatalf("read PRAGMA %s error = %v", name, err)
	}
	if got != want {
		t.Errorf("PRAGMA %s = %d, want %d", name, got, want)
	}
}
