//go:build linux || darwin

package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	forgemigrations "github.com/damienomurchu/forge-cli/migrations"
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

func TestApplyMigrationsDoesNotApplyRegisteredFutureMigration(t *testing.T) {
	if got := migrationFiles[len(migrationFiles)-1]; got.version != 2 || got.name != "002_unified_captures.sql" {
		t.Fatalf("last registered migration = %+v, want staged migration 002", got)
	}
	db := openTestSQLite(t)
	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	assertSchemaVersion(t, db, LatestSchemaVersion)
	assertTableExists(t, db, "record_tags")
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 2`).Scan(&count); err != nil {
		t.Fatalf("read staged migration count error = %v", err)
	}
	if count != 0 {
		t.Errorf("staged migration count = %d, want 0", count)
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

func TestUnifiedCaptureMigrationUpgradesEmptyInitialSchema(t *testing.T) {
	db := openInitialSchemaDatabase(t)
	applyUnifiedCaptureMigration(t, db)

	assertSchemaVersion(t, db, 2)
	assertTableExists(t, db, "records")
	assertTableMissing(t, db, "record_tags")
	assertColumns(t, db, "records", []string{
		"id",
		"capture_type",
		"description",
		"friction_project",
		"friction_frequency",
		"friction_impact",
		"friction_category",
		"friction_current_workaround",
		"created_at",
		"updated_at",
	})
	for _, name := range []string{
		"idx_records_created",
		"idx_records_type_created",
		"idx_records_project_created",
	} {
		assertIndexExists(t, db, name)
	}
	for _, name := range []string{
		"records_type_immutable",
		"record_tags_capture_only_insert",
		"record_tags_capture_only_update",
	} {
		assertSchemaObjectMissing(t, db, "trigger", name)
	}
}

func TestUnifiedCaptureMigrationPreservesFriction(t *testing.T) {
	db := openInitialSchemaDatabase(t)
	if _, err := db.Exec(`INSERT INTO records (
		id, type, description, project, status, capture_kind,
		friction_frequency, friction_impact, friction_category,
		current_workaround, created_at, updated_at
	) VALUES (?, 'friction', ?, ?, 'captured', NULL, ?, ?, ?, ?, ?, ?)`,
		"frc_000102030405060708090a0b0c0d0e0f",
		"Repeated release checks",
		"forge",
		"weekly",
		"high",
		"verification",
		"Use a checklist",
		"2026-08-25T12:00:00.000000Z",
		"2026-08-26T13:00:00.000000Z",
	); err != nil {
		t.Fatalf("insert migration-001 friction error = %v", err)
	}

	applyUnifiedCaptureMigration(t, db)

	var (
		id, captureType, description, project, frequency   string
		impact, category, workaround, createdAt, updatedAt string
	)
	if err := db.QueryRow(`SELECT
		id, capture_type, description, friction_project,
		friction_frequency, friction_impact, friction_category,
		friction_current_workaround, created_at, updated_at
		FROM records`).Scan(
		&id, &captureType, &description, &project, &frequency,
		&impact, &category, &workaround, &createdAt, &updatedAt,
	); err != nil {
		t.Fatalf("read migrated friction error = %v", err)
	}
	want := []string{
		"frc_000102030405060708090a0b0c0d0e0f",
		"friction",
		"Repeated release checks",
		"forge",
		"weekly",
		"high",
		"verification",
		"Use a checklist",
		"2026-08-25T12:00:00.000000Z",
		"2026-08-26T13:00:00.000000Z",
	}
	got := []string{id, captureType, description, project, frequency, impact, category, workaround, createdAt, updatedAt}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("migrated value %d = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestUnifiedCaptureMigrationTargetConstraints(t *testing.T) {
	db := openInitialSchemaDatabase(t)
	applyUnifiedCaptureMigration(t, db)

	valid := []struct {
		id          string
		captureType string
		frequency   any
		impact      any
		category    any
	}{
		{id: "cap_00000000000000000000000000000000", captureType: "friction", frequency: "unknown", impact: "unknown", category: "other"},
		{id: "cap_01010101010101010101010101010101", captureType: "action"},
		{id: "cap_02020202020202020202020202020202", captureType: "follow-up"},
		{id: "cap_03030303030303030303030303030303", captureType: "decision"},
	}
	for _, record := range valid {
		if _, err := db.Exec(`INSERT INTO records (
			id, capture_type, description,
			friction_frequency, friction_impact, friction_category,
			created_at, updated_at
		) VALUES (?, ?, 'description', ?, ?, ?, ?, ?)`,
			record.id, record.captureType, record.frequency, record.impact, record.category,
			"2026-08-25T12:00:00.000000Z", "2026-08-25T12:00:00.000000Z",
		); err != nil {
			t.Errorf("insert valid %s error = %v", record.captureType, err)
		}
	}

	invalid := []struct {
		name        string
		id          string
		captureType string
		project     any
		frequency   any
		impact      any
		category    any
	}{
		{name: "unknown type", id: "cap_10101010101010101010101010101010", captureType: "thought"},
		{name: "friction missing classification", id: "cap_11111111111111111111111111111111", captureType: "friction"},
		{name: "action with friction details", id: "cap_12121212121212121212121212121212", captureType: "action", frequency: "unknown", impact: "unknown", category: "other"},
		{name: "legacy ID on action", id: "frc_13131313131313131313131313131313", captureType: "action"},
		{name: "unnormalized project", id: "cap_14141414141414141414141414141414", captureType: "friction", project: " forge ", frequency: "unknown", impact: "unknown", category: "other"},
	}
	for _, record := range invalid {
		t.Run(record.name, func(t *testing.T) {
			_, err := db.Exec(`INSERT INTO records (
				id, capture_type, description, friction_project,
				friction_frequency, friction_impact, friction_category,
				created_at, updated_at
			) VALUES (?, ?, 'description', ?, ?, ?, ?, ?, ?)`,
				record.id, record.captureType, record.project,
				record.frequency, record.impact, record.category,
				"2026-08-25T12:00:00.000000Z", "2026-08-25T12:00:00.000000Z",
			)
			if err == nil {
				t.Fatal("invalid unified capture insert succeeded")
			}
		})
	}
}

func TestUnifiedCaptureMigrationRejectsUnrepresentableLegacyDataAndRollsBack(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, *sql.DB)
	}{
		{
			name: "capture row",
			setup: func(t *testing.T, db *sql.DB) {
				t.Helper()
				insertLegacyCapture(t, db)
			},
		},
		{
			name: "tag row",
			setup: func(t *testing.T, db *sql.DB) {
				t.Helper()
				insertLegacyCapture(t, db)
				if _, err := db.Exec(`INSERT INTO record_tags(record_id, position, tag)
					VALUES ('cap_00000000000000000000000000000000', 0, 'legacy')`); err != nil {
					t.Fatalf("insert legacy tag error = %v", err)
				}
				if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
					t.Fatalf("disable foreign keys error = %v", err)
				}
				if _, err := db.Exec(`DELETE FROM records
					WHERE id = 'cap_00000000000000000000000000000000'`); err != nil {
					t.Fatalf("delete legacy capture error = %v", err)
				}
				if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
					t.Fatalf("enable foreign keys error = %v", err)
				}
			},
		},
		{
			name: "non-captured friction",
			setup: func(t *testing.T, db *sql.DB) {
				t.Helper()
				if _, err := db.Exec(`INSERT INTO records (
					id, type, description, status, capture_kind,
					friction_frequency, friction_impact, friction_category,
					created_at, updated_at
				) VALUES (
					'frc_00000000000000000000000000000000', 'friction',
					'legacy friction', 'reviewing', NULL,
					'unknown', 'unknown', 'other',
					'2026-08-25T12:00:00.000000Z', '2026-08-25T12:00:00.000000Z'
				)`); err != nil {
					t.Fatalf("insert legacy friction error = %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openInitialSchemaDatabase(t)
			tt.setup(t, db)
			err := runUnifiedCaptureMigration(db)
			if err == nil || !strings.Contains(err.Error(), "apply migration 002_unified_captures.sql") {
				t.Fatalf("migration error = %v, want migration-002 failure", err)
			}
			assertSchemaVersion(t, db, 1)
			assertTableExists(t, db, "record_tags")
			assertColumns(t, db, "records", []string{
				"id", "type", "description", "project", "status", "capture_kind",
				"friction_frequency", "friction_impact", "friction_category",
				"current_workaround", "created_at", "updated_at",
			})
			assertTableMissing(t, db, "records_v2")
		})
	}
}

func openInitialSchemaDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db := openTestSQLite(t)
	if err := ApplyMigrations(context.Background(), db); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	assertSchemaVersion(t, db, 1)
	return db
}

func applyUnifiedCaptureMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := runUnifiedCaptureMigration(db); err != nil {
		t.Fatalf("unified capture migration error = %v", err)
	}
}

func runUnifiedCaptureMigration(db *sql.DB) error {
	query, err := forgemigrations.Files.ReadFile("002_unified_captures.sql")
	if err != nil {
		return err
	}
	return applyMigration(context.Background(), db, migration{
		version: 2,
		name:    "002_unified_captures.sql",
		query:   string(query),
	})
}

func insertLegacyCapture(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO records (
		id, type, description, status, capture_kind,
		friction_frequency, friction_impact, friction_category,
		created_at, updated_at
	) VALUES (
		'cap_00000000000000000000000000000000', 'capture',
		'legacy capture', 'captured', 'thought', NULL, NULL, NULL,
		'2026-08-25T12:00:00.000000Z', '2026-08-25T12:00:00.000000Z'
	)`); err != nil {
		t.Fatalf("insert legacy capture error = %v", err)
	}
}

func assertSchemaVersion(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&got); err != nil {
		t.Fatalf("read schema version error = %v", err)
	}
	if got != want {
		t.Errorf("schema version = %d, want %d", got, want)
	}
}

func assertColumns(t *testing.T, db *sql.DB, table string, want []string) {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("inspect columns for %s error = %v", table, err)
	}
	defer rows.Close()
	got := make([]string, 0, len(want))
	for rows.Next() {
		var position, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&position, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan columns for %s error = %v", table, err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("inspect columns for %s error = %v", table, err)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("columns for %s = %v, want %v", table, got, want)
	}
}

func assertIndexExists(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_schema WHERE type = 'index' AND name = ?`, name,
	).Scan(&count); err != nil {
		t.Fatalf("inspect index %s error = %v", name, err)
	}
	if count != 1 {
		t.Errorf("index %s count = %d, want 1", name, count)
	}
}

func assertSchemaObjectMissing(t *testing.T, db *sql.DB, objectType, name string) {
	t.Helper()
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_schema WHERE type = ? AND name = ?`, objectType, name,
	).Scan(&count); err != nil {
		t.Fatalf("inspect %s %s error = %v", objectType, name, err)
	}
	if count != 0 {
		t.Errorf("%s %s count = %d, want 0", objectType, name, count)
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
