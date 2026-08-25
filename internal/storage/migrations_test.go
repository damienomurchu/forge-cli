//go:build linux || darwin

package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func TestInspectSchemaDoesNotModifyEmptyDatabase(t *testing.T) {
	db := openTestSQLite(t)

	state, err := InspectSchema(context.Background(), db)
	if err != nil {
		t.Fatalf("InspectSchema() error = %v", err)
	}
	if state.Version != 0 || !state.NeedsMigration {
		t.Errorf("InspectSchema() = %+v, want version 0 requiring migration", state)
	}
	assertTableMissing(t, db, "schema_migrations")
}

func TestApplyMigrationsInitializesFreshDatabase(t *testing.T) {
	db := openTestSQLite(t)

	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	state, err := InspectSchema(context.Background(), db)
	if err != nil {
		t.Fatalf("InspectSchema() error = %v", err)
	}
	if state.Version != LatestSchemaVersion || state.NeedsMigration {
		t.Errorf("InspectSchema() = %+v, want current version", state)
	}

	for _, table := range []string{"schema_migrations", "records", "record_tags"} {
		assertTableExists(t, db, table)
	}
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM schema_migrations
		 WHERE version = 1 AND name = '001_initial.sql' AND applied_at <> ''`,
	).Scan(&count); err != nil {
		t.Fatalf("read migration record error = %v", err)
	}
	if count != 1 {
		t.Errorf("migration record count = %d, want 1", count)
	}
}

func TestApplyMigrationsIsIdempotent(t *testing.T) {
	db := openTestSQLite(t)
	ctx := context.Background()
	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("first ApplyMigrations() error = %v", err)
	}
	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("second ApplyMigrations() error = %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("read migration count error = %v", err)
	}
	if count != 1 {
		t.Errorf("migration count = %d, want 1", count)
	}
}

func TestInspectSchemaRejectsNewerDatabase(t *testing.T) {
	db := openTestSQLite(t)
	ctx := context.Background()
	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)`,
		LatestSchemaVersion+1,
		"future.sql",
		"2026-08-25T00:00:00.000Z",
	); err != nil {
		t.Fatalf("insert future migration error = %v", err)
	}

	state, err := InspectSchema(ctx, db)
	if state != (SchemaState{}) {
		t.Errorf("InspectSchema() state = %+v, want zero", state)
	}
	want := "database schema version 2 is newer than supported version 1"
	if err == nil || err.Error() != want {
		t.Fatalf("InspectSchema() error = %v, want %q", err, want)
	}
	if err := ApplyMigrations(ctx, db); err == nil || err.Error() != want {
		t.Fatalf("ApplyMigrations() error = %v, want %q", err, want)
	}
}

func TestInspectSchemaRejectsArchivedPythonMigrationLayoutWithoutModification(t *testing.T) {
	db := openTestSQLite(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create archived migration table error = %v", err)
	}
	const appliedAt = "2026-08-23T07:58:05.290Z"
	if _, err := db.ExecContext(ctx,
		`INSERT INTO schema_migrations(name, applied_at) VALUES (?, ?)`,
		"001_initial.sql",
		appliedAt,
	); err != nil {
		t.Fatalf("insert archived migration row error = %v", err)
	}

	want := "incompatible database schema: schema_migrations has an unsupported layout"
	for name, inspect := range map[string]func() error{
		"inspect": func() error {
			_, err := InspectSchema(ctx, db)
			return err
		},
		"apply": func() error { return ApplyMigrations(ctx, db) },
	} {
		t.Run(name, func(t *testing.T) {
			err := inspect()
			if !errors.Is(err, ErrIncompatibleSchema) || err.Error() != want {
				t.Fatalf("schema operation error = %v, want %q", err, want)
			}
		})
	}

	var name, storedAppliedAt string
	if err := db.QueryRowContext(ctx,
		`SELECT name, applied_at FROM schema_migrations`,
	).Scan(&name, &storedAppliedAt); err != nil {
		t.Fatalf("read preserved migration row error = %v", err)
	}
	if name != "001_initial.sql" || storedAppliedAt != appliedAt {
		t.Errorf("migration row = %q/%q, want original values", name, storedAppliedAt)
	}
	assertTableMissing(t, db, "records")
}

func TestInspectSchemaRejectsUnexpectedMigrationColumns(t *testing.T) {
	db := openTestSQLite(t)
	if _, err := db.Exec(`CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at TEXT NOT NULL,
		unexpected TEXT
	)`); err != nil {
		t.Fatalf("create altered migration table error = %v", err)
	}

	_, err := InspectSchema(context.Background(), db)
	if !errors.Is(err, ErrIncompatibleSchema) {
		t.Fatalf("InspectSchema() error = %v, want ErrIncompatibleSchema", err)
	}
}

func TestApplyMigrationRollsBackFailedSQL(t *testing.T) {
	db := openTestSQLite(t)
	change := migration{
		version: 1,
		name:    "001_broken.sql",
		query: `CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		);
		CREATE TABLE transient (id INTEGER PRIMARY KEY);
		THIS IS NOT SQL;`,
	}

	err := applyMigration(context.Background(), db, change)
	if err == nil || !strings.Contains(err.Error(), "apply migration 001_broken.sql") {
		t.Fatalf("applyMigration() error = %v", err)
	}
	assertTableMissing(t, db, "schema_migrations")
	assertTableMissing(t, db, "transient")
}

func TestInspectSchemaRejectsNilDatabase(t *testing.T) {
	state, err := InspectSchema(context.Background(), nil)
	if state != (SchemaState{}) {
		t.Errorf("InspectSchema() state = %+v, want zero", state)
	}
	if err == nil || err.Error() != "sqlite database is required" {
		t.Fatalf("InspectSchema() error = %v", err)
	}
}

func openTestSQLite(t *testing.T) *sql.DB {
	t.Helper()
	directory, database := openTestDatabaseFile(t, DatabaseCreate)
	db, err := OpenSQLite(context.Background(), directory, database, DatabaseCreate)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func assertTableExists(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_schema WHERE type = 'table' AND name = ?`,
		name,
	).Scan(&count); err != nil {
		t.Fatalf("inspect table %s error = %v", name, err)
	}
	if count != 1 {
		t.Errorf("table %s count = %d, want 1", name, count)
	}
}

func assertTableMissing(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_schema WHERE type = 'table' AND name = ?`,
		name,
	).Scan(&count); err != nil {
		t.Fatalf("inspect table %s error = %v", name, err)
	}
	if count != 0 {
		t.Errorf("table %s count = %d, want 0", name, count)
	}
}
