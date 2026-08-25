//go:build linux || darwin

package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	// ErrStorageNotFound identifies an absent data directory or database file.
	ErrStorageNotFound = errors.New("forge storage not found")
	// ErrMigrationRequired identifies an older database that this open-only
	// lifecycle must not migrate.
	ErrMigrationRequired = errors.New("database migration required")
)

// Session owns every resource in one open Forge storage lifecycle.
type Session struct {
	db        *sql.DB
	database  *os.File
	directory *os.File
}

// OpenExisting opens an existing, current Forge database without creating or
// migrating any storage. Mode must be DatabaseReadOnly or DatabaseReadWrite.
func OpenExisting(
	ctx context.Context,
	databasePath string,
	effectiveUID int,
	mode DatabaseOpenMode,
) (*Session, error) {
	if mode != DatabaseReadOnly && mode != DatabaseReadWrite {
		return nil, fmt.Errorf("open-existing storage requires read-only or read-write mode")
	}
	if !filepath.IsAbs(databasePath) {
		return nil, fmt.Errorf("database path must be absolute")
	}
	databasePath = filepath.Clean(databasePath)
	if filepath.Base(databasePath) != databaseFilename {
		return nil, fmt.Errorf("database path must end with %s", databaseFilename)
	}

	directory, err := OpenDataDirectory(filepath.Dir(databasePath), effectiveUID)
	if err != nil {
		return nil, classifyStorageOpenError("open existing data directory", err)
	}
	session := &Session{directory: directory}
	database, err := OpenDatabaseFile(directory, mode, effectiveUID)
	if err != nil {
		return nil, closeFailedSession(
			session,
			classifyStorageOpenError("open existing database file", err),
		)
	}
	session.database = database
	db, err := OpenSQLite(ctx, directory, database, mode)
	if err != nil {
		return nil, closeFailedSession(
			session,
			fmt.Errorf("open existing sqlite database: %w", err),
		)
	}
	session.db = db

	state, err := InspectSchema(ctx, db)
	if err != nil {
		return nil, closeFailedSession(session, err)
	}
	if state.NeedsMigration {
		return nil, closeFailedSession(session, fmt.Errorf(
			"%w: database schema version %d, want %d",
			ErrMigrationRequired,
			state.Version,
			LatestSchemaVersion,
		))
	}
	return session, nil
}

// Database returns the configured SQLite pool owned by the session.
func (s *Session) Database() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

// Close releases SQLite, the database file, and the data directory in reverse
// opening order. It is safe to call more than once.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	var closeErrors []error
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close sqlite database: %w", err))
		}
		s.db = nil
	}
	if s.database != nil {
		if err := s.database.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close database file: %w", err))
		}
		s.database = nil
	}
	if s.directory != nil {
		if err := s.directory.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close data directory: %w", err))
		}
		s.directory = nil
	}
	return errors.Join(closeErrors...)
}

func classifyStorageOpenError(operation string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %s: %w", ErrStorageNotFound, operation, err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func closeFailedSession(session *Session, failure error) error {
	if closeErr := session.Close(); closeErr != nil {
		return errors.Join(failure, closeErr)
	}
	return failure
}
