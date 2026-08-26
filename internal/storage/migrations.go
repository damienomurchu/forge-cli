package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	forgemigrations "github.com/damienomurchu/forge-cli/migrations"
)

const LatestSchemaVersion = 1

// ErrIncompatibleSchema identifies an existing database whose migration
// metadata was not created by this Go implementation.
var ErrIncompatibleSchema = errors.New("incompatible database schema")

type migration struct {
	version int
	name    string
	query   string
}

var migrationFiles = []struct {
	version int
	name    string
}{
	{version: 1, name: "001_initial.sql"},
	// Migration 002 is embedded and tested but remains inactive until the
	// unified repository and command cutover raises LatestSchemaVersion.
	{version: 2, name: "002_unified_captures.sql"},
}

// SchemaState describes whether an open database requires a migration.
type SchemaState struct {
	Version        int
	NeedsMigration bool
}

// InspectSchema reads the current schema version without changing the database.
// An empty database is version zero. Databases newer than this executable are
// rejected as incompatible.
func InspectSchema(ctx context.Context, db *sql.DB) (SchemaState, error) {
	if db == nil {
		return SchemaState{}, fmt.Errorf("sqlite database is required")
	}

	version, err := currentSchemaVersion(ctx, db)
	if err != nil {
		return SchemaState{}, err
	}
	if version > LatestSchemaVersion {
		return SchemaState{}, fmt.Errorf(
			"database schema version %d is newer than supported version %d",
			version,
			LatestSchemaVersion,
		)
	}
	return SchemaState{
		Version:        version,
		NeedsMigration: version < LatestSchemaVersion,
	}, nil
}

// ApplyMigrations applies every pending embedded migration in numeric order.
// Each migration and its schema_migrations row commit in one transaction.
func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	state, err := InspectSchema(ctx, db)
	if err != nil {
		return err
	}
	if !state.NeedsMigration {
		return nil
	}

	for _, file := range migrationFiles {
		if file.version > LatestSchemaVersion {
			break
		}
		if file.version <= state.Version {
			continue
		}
		query, err := forgemigrations.Files.ReadFile(file.name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file.name, err)
		}
		if err := applyMigration(ctx, db, migration{
			version: file.version,
			name:    file.name,
			query:   string(query),
		}); err != nil {
			return err
		}
	}
	return nil
}

func currentSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var tableName string
	err := db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_schema WHERE type = 'table' AND name = 'schema_migrations'`,
	).Scan(&tableName)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("inspect migration table: %w", err)
	}
	if err := validateMigrationTableLayout(ctx, db); err != nil {
		return 0, err
	}

	var version int
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`,
	).Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

func validateMigrationTableLayout(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(schema_migrations)`)
	if err != nil {
		return fmt.Errorf("inspect migration table layout: %w", err)
	}
	defer rows.Close()

	columns := make([]string, 0, 3)
	for rows.Next() {
		var (
			position     int
			name         string
			columnType   string
			notNull      int
			defaultValue any
			primaryKey   int
		)
		if err := rows.Scan(
			&position,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			return fmt.Errorf("inspect migration table layout: %w", err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect migration table layout: %w", err)
	}

	want := []string{"version", "name", "applied_at"}
	if len(columns) != len(want) {
		return unsupportedMigrationTableLayout()
	}
	for index := range want {
		if columns[index] != want[index] {
			return unsupportedMigrationTableLayout()
		}
	}
	return nil
}

func unsupportedMigrationTableLayout() error {
	return fmt.Errorf("%w: schema_migrations has an unsupported layout", ErrIncompatibleSchema)
}

func applyMigration(ctx context.Context, db *sql.DB, change migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", change.name, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, change.query); err != nil {
		return fmt.Errorf("apply migration %s: %w", change.name, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, name, applied_at)
		 VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))`,
		change.version,
		change.name,
	); err != nil {
		return fmt.Errorf("record migration %s: %w", change.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", change.name, err)
	}
	return nil
}
