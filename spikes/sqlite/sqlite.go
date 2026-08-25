// Package sqliteprobe contains the disposable Phase 1 SQLite driver spike.
package sqliteprobe

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/damienomurchu/forge-cli/migrations"
	_ "modernc.org/sqlite"
)

const (
	MigrationVersion = 1
	MigrationName    = "001_initial.sql"
	BusyTimeoutMS    = 250
)

func Open(path string) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	u := &url.URL{Scheme: "file", Path: abs}
	query := u.Query()
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", BusyTimeoutMS))
	u.RawQuery = query.Encode()

	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	version, exists, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if exists && version > MigrationVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", version, MigrationVersion)
	}
	if exists && version == MigrationVersion {
		return nil
	}

	schema, err := migrations.Files.ReadFile(MigrationName)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", MigrationName, err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, string(schema)); err != nil {
		return fmt.Errorf("apply migration %s: %w", MigrationName, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, name, applied_at)
		 VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))`,
		MigrationVersion, MigrationName,
	); err != nil {
		return fmt.Errorf("record migration %s: %w", MigrationName, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", MigrationName, err)
	}
	return nil
}

func schemaVersion(ctx context.Context, db *sql.DB) (version int, exists bool, err error) {
	var tableName string
	err = db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_schema WHERE type = 'table' AND name = 'schema_migrations'`,
	).Scan(&tableName)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("inspect migration table: %w", err)
	}
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`,
	).Scan(&version); err != nil {
		return 0, true, fmt.Errorf("read schema version: %w", err)
	}
	return version, true, nil
}
