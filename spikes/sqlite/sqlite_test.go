package sqliteprobe

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testTimestamp = "2026-08-25T09:14:03.123456Z"

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "forge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func captureID(char string) string  { return "cap_" + strings.Repeat(char, 32) }
func frictionID(char string) string { return "frc_" + strings.Repeat(char, 32) }

func insertCapture(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, id string) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO records(
			id, type, description, project, status, capture_kind,
			friction_frequency, friction_impact, friction_category,
			current_workaround, created_at, updated_at
		) VALUES (?, 'capture', 'Measure startup', 'forge', 'captured', 'thought',
			NULL, NULL, NULL, NULL, ?, ?)`, id, testTimestamp, testTimestamp)
	return err
}

func insertFriction(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, id string) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO records(
			id, type, description, project, status, capture_kind,
			friction_frequency, friction_impact, friction_category,
			current_workaround, created_at, updated_at
		) VALUES (?, 'friction', 'Repeated manual checks', NULL, 'captured', NULL,
			'weekly', 'high', 'verification', 'Use a checklist', ?, ?)`,
		id, testTimestamp, testTimestamp)
	return err
}

func TestMigrationIsValidAndIdempotent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second migration: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = 1 AND name = '001_initial.sql'`,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("migration rows = %d, want 1", count)
	}

	var foreignKeys int
	if err := db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
}

func TestCRUDTagsAndStatusUpdate(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	id := captureID("a")
	if err := insertCapture(ctx, db, id); err != nil {
		t.Fatal(err)
	}
	for position, tag := range []string{"performance", "cli"} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO record_tags(record_id, position, tag) VALUES (?, ?, ?)`,
			id, position, tag,
		); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := db.QueryContext(ctx,
		`SELECT tag FROM record_tags WHERE record_id = ? ORDER BY position ASC`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			t.Fatal(err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tags, ","); got != "performance,cli" {
		t.Fatalf("tags = %q", got)
	}

	updated := "2026-08-26T11:02:19.000000Z"
	result, err := db.ExecContext(ctx,
		`UPDATE records SET status = ?, updated_at = ? WHERE id = ? AND status <> ?`,
		"reviewing", updated, id, "reviewing")
	if err != nil {
		t.Fatal(err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		t.Fatalf("changed rows = %d, want 1", affected)
	}
	result, err = db.ExecContext(ctx,
		`UPDATE records SET status = ?, updated_at = ? WHERE id = ? AND status <> ?`,
		"reviewing", "2027-01-01T00:00:00.000000Z", id, "reviewing")
	if err != nil {
		t.Fatal(err)
	}
	if affected, _ := result.RowsAffected(); affected != 0 {
		t.Fatalf("no-op changed rows = %d, want 0", affected)
	}
	var status, storedUpdated string
	if err := db.QueryRowContext(ctx,
		`SELECT status, updated_at FROM records WHERE id = ?`, id,
	).Scan(&status, &storedUpdated); err != nil {
		t.Fatal(err)
	}
	if status != "reviewing" || storedUpdated != updated {
		t.Fatalf("status/update = %q/%q", status, storedUpdated)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM records WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM record_tags WHERE record_id = ?`, id,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("tags after cascade = %d", count)
	}
}

func TestConstraintsRejectInvalidRecords(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	friction := frictionID("b")
	if err := insertFriction(ctx, db, friction); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		query string
		args  []any
	}{
		{
			"capture without kind",
			`INSERT INTO records(id, type, description, status, created_at, updated_at)
			 VALUES (?, 'capture', 'Missing kind', 'captured', ?, ?)`,
			[]any{captureID("9"), testTimestamp, testTimestamp},
		},
		{
			"friction with capture fields",
			`INSERT INTO records(
				id, type, description, status, capture_kind,
				friction_frequency, friction_impact, friction_category,
				created_at, updated_at
			) VALUES (?, 'friction', 'Mixed details', 'captured', 'thought',
				'weekly', 'high', 'verification', ?, ?)`,
			[]any{frictionID("8"), testTimestamp, testTimestamp},
		},
		{"bad id", `UPDATE records SET id = 'frc_NOT_HEX' WHERE id = ?`, []any{friction}},
		{"bad status", `UPDATE records SET status = 'done' WHERE id = ?`, []any{friction}},
		{"change type", `UPDATE records SET type = 'capture' WHERE id = ?`, []any{friction}},
		{"tag on friction", `INSERT INTO record_tags(record_id, position, tag) VALUES (?, 0, 'bad')`, []any{friction}},
		{"uppercase tag", `INSERT INTO record_tags(record_id, position, tag) VALUES (?, 0, 'Bad')`, []any{captureID("c")}},
	}
	if err := insertCapture(ctx, db, captureID("c")); err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := db.ExecContext(ctx, test.query, test.args...); err == nil {
				t.Fatal("expected constraint failure")
			}
		})
	}
}

func TestTransactionRollbackLeavesNoPartialCapture(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	id := captureID("d")
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := insertCapture(ctx, tx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO record_tags(record_id, position, tag) VALUES (?, 0, 'same'), (?, 1, 'same')`,
		id, id,
	); err == nil {
		t.Fatal("expected duplicate tag failure")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM records WHERE id = ?`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("records after rollback = %d", count)
	}
}

func TestNewerMigrationIsRejected(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, name, applied_at) VALUES (2, 'future.sql', ?)`,
		testTimestamp,
	); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("Migrate() error = %v, want newer-version error", err)
	}
}

func TestBusyTimeoutIsFinite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "forge.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if err := Migrate(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	ctx := context.Background()
	tx, err := first.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := insertCapture(ctx, tx, captureID("e")); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	err = insertFriction(ctx, second, frictionID("f"))
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected locked database error")
	}
	if elapsed < 150*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("busy timeout elapsed %s, want roughly %dms", elapsed, BusyTimeoutMS)
	}
}

func TestQueryPlansUseInitialIndexes(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	statuses := []string{"captured", "reviewing", "candidate", "automated", "dismissed"}
	for i := range 500 {
		id := fmt.Sprintf("frc_%032x", i+1)
		if err := insertFriction(ctx, db, id); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx,
			`UPDATE records SET status = ? WHERE id = ?`, statuses[i%len(statuses)], id,
		); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `ANALYZE`); err != nil {
		t.Fatal(err)
	}

	assertPlanUses(t, db,
		`EXPLAIN QUERY PLAN SELECT id FROM records
		 WHERE type = 'friction' AND status IN ('captured', 'reviewing', 'candidate')
		 ORDER BY created_at DESC, id DESC LIMIT 10`,
		"idx_records_type_created")
	assertPlanUses(t, db,
		`EXPLAIN QUERY PLAN SELECT id FROM records
		 WHERE project = 'forge' ORDER BY created_at DESC, id DESC LIMIT 10`,
		"idx_records_project_created")
}

func assertPlanUses(t *testing.T, db *sql.DB, query, index string) {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var details []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	plan := strings.Join(details, "\n")
	if !strings.Contains(plan, index) {
		t.Fatalf("query plan does not use %s:\n%s", index, plan)
	}
}
