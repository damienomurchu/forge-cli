//go:build linux || darwin

package repository

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/storage"
)

func TestCreateCaptureStoresRecordAndOrderedTags(t *testing.T) {
	repository, db := openTestRepository(t)
	record := newTestCapture(t, domain.CaptureInput{
		Description: "Measure command startup time",
		Project:     "forge",
		Kind:        domain.CaptureKindObservation,
		Tags:        "performance,cli",
	}, 0)

	if err := repository.CreateCapture(context.Background(), record); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}

	var (
		id, recordType, description, project, status, kind string
		frequency, impact, category, workaround            sql.NullString
		createdAt, updatedAt                               string
	)
	if err := db.QueryRow(`SELECT
		id, type, description, project, status, capture_kind,
		friction_frequency, friction_impact, friction_category,
		current_workaround, created_at, updated_at
		FROM records WHERE id = ?`, record.ID.String()).Scan(
		&id, &recordType, &description, &project, &status, &kind,
		&frequency, &impact, &category, &workaround, &createdAt, &updatedAt,
	); err != nil {
		t.Fatalf("read stored capture error = %v", err)
	}
	if id != record.ID.String() || recordType != "capture" || description != record.Description {
		t.Errorf("stored identity = %q/%q/%q", id, recordType, description)
	}
	if project != "forge" || status != "captured" || kind != "observation" {
		t.Errorf("stored capture values = %q/%q/%q", project, status, kind)
	}
	if frequency.Valid || impact.Valid || category.Valid || workaround.Valid {
		t.Errorf("stored friction values = %#v/%#v/%#v/%#v, want NULL", frequency, impact, category, workaround)
	}
	if createdAt != record.CreatedAt.String() || updatedAt != record.UpdatedAt.String() {
		t.Errorf("stored timestamps = %q/%q", createdAt, updatedAt)
	}

	rows, err := db.Query(`SELECT position, tag FROM record_tags WHERE record_id = ? ORDER BY position`, record.ID.String())
	if err != nil {
		t.Fatalf("query tags error = %v", err)
	}
	defer rows.Close()
	wantTags := []string{"performance", "cli"}
	for position, want := range wantTags {
		if !rows.Next() {
			t.Fatalf("tag %d missing", position)
		}
		var gotPosition int
		var gotTag string
		if err := rows.Scan(&gotPosition, &gotTag); err != nil {
			t.Fatalf("scan tag %d error = %v", position, err)
		}
		if gotPosition != position || gotTag != want {
			t.Errorf("tag %d = %d/%q, want %d/%q", position, gotPosition, gotTag, position, want)
		}
	}
	if rows.Next() {
		t.Errorf("unexpected extra tag")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tags error = %v", err)
	}
}

func TestCreateCaptureStoresAbsentOptionalValues(t *testing.T) {
	repository, db := openTestRepository(t)
	record := newTestCapture(t, domain.CaptureInput{
		Description: "Keep this thought",
		Kind:        domain.CaptureKindThought,
	}, 1)

	if err := repository.CreateCapture(context.Background(), record); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}
	var project sql.NullString
	var tagCount int
	if err := db.QueryRow(`SELECT project FROM records WHERE id = ?`, record.ID.String()).Scan(&project); err != nil {
		t.Fatalf("read project error = %v", err)
	}
	if project.Valid {
		t.Errorf("project = %q, want NULL", project.String)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM record_tags WHERE record_id = ?`, record.ID.String()).Scan(&tagCount); err != nil {
		t.Fatalf("read tag count error = %v", err)
	}
	if tagCount != 0 {
		t.Errorf("tag count = %d, want 0", tagCount)
	}
}

func TestCreateCaptureRollsBackDuplicateID(t *testing.T) {
	repository, db := openTestRepository(t)
	record := newTestCapture(t, domain.CaptureInput{
		Description: "Original",
		Kind:        domain.CaptureKindThought,
		Tags:        "original",
	}, 2)
	if err := repository.CreateCapture(context.Background(), record); err != nil {
		t.Fatalf("first CreateCapture() error = %v", err)
	}

	duplicate := record
	duplicate.Description = "Duplicate"
	duplicate.Details.Capture = &domain.CaptureDetails{
		Kind: domain.CaptureKindIdea,
		Tags: []string{"duplicate"},
	}
	err := repository.CreateCapture(context.Background(), duplicate)
	if err == nil || !strings.Contains(err.Error(), "insert capture "+record.ID.String()) {
		t.Fatalf("duplicate CreateCapture() error = %v", err)
	}
	assertRecordAndTagCounts(t, db, 1, 1)

	var description, tag string
	if err := db.QueryRow(`SELECT description FROM records WHERE id = ?`, record.ID.String()).Scan(&description); err != nil {
		t.Fatalf("read description error = %v", err)
	}
	if err := db.QueryRow(`SELECT tag FROM record_tags WHERE record_id = ?`, record.ID.String()).Scan(&tag); err != nil {
		t.Fatalf("read tag error = %v", err)
	}
	if description != "Original" || tag != "original" {
		t.Errorf("stored duplicate state = %q/%q, want original", description, tag)
	}
}

func TestCreateCaptureRollsBackTagFailure(t *testing.T) {
	repository, db := openTestRepository(t)
	if _, err := db.Exec(`CREATE TRIGGER reject_blocked_tag
		BEFORE INSERT ON record_tags WHEN NEW.tag = 'blocked'
		BEGIN SELECT RAISE(ABORT, 'blocked test tag'); END`); err != nil {
		t.Fatalf("create test trigger error = %v", err)
	}
	record := newTestCapture(t, domain.CaptureInput{
		Description: "Must roll back",
		Kind:        domain.CaptureKindThought,
		Tags:        "first,blocked",
	}, 3)

	err := repository.CreateCapture(context.Background(), record)
	if err == nil || !strings.Contains(err.Error(), "tag 1") {
		t.Fatalf("CreateCapture() error = %v, want tag error", err)
	}
	assertRecordAndTagCounts(t, db, 0, 0)
}

func TestCreateCaptureRejectsInvalidRecord(t *testing.T) {
	repository, db := openTestRepository(t)
	record := newTestCapture(t, domain.CaptureInput{
		Description: "Valid first",
		Kind:        domain.CaptureKindThought,
	}, 4)
	record.Description = " "

	err := repository.CreateCapture(context.Background(), record)
	if err == nil || !strings.Contains(err.Error(), "validate capture") {
		t.Fatalf("CreateCapture() error = %v, want validation error", err)
	}
	assertRecordAndTagCounts(t, db, 0, 0)
}

func TestNewRejectsNilDatabase(t *testing.T) {
	repository, err := New(nil)
	if repository != nil {
		t.Fatalf("New() = %#v, want nil", repository)
	}
	if err == nil || err.Error() != "sqlite database is required" {
		t.Fatalf("New() error = %v", err)
	}
}

func openTestRepository(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()
	directory, err := storage.PrepareDataDirectory(filepath.Join(t.TempDir(), "forge"), os.Geteuid())
	if err != nil {
		t.Fatalf("PrepareDataDirectory() error = %v", err)
	}
	t.Cleanup(func() { directory.Close() })
	database, err := storage.OpenDatabaseFile(directory, storage.DatabaseCreate, os.Geteuid())
	if err != nil {
		t.Fatalf("OpenDatabaseFile() error = %v", err)
	}
	t.Cleanup(func() { database.Close() })
	db, err := storage.OpenSQLite(context.Background(), directory, database, storage.DatabaseCreate)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := storage.ApplyMigrations(context.Background(), db); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	repository, err := New(db)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return repository, db
}

func newTestCapture(t *testing.T, input domain.CaptureInput, seed byte) domain.Record {
	t.Helper()
	record, err := domain.NewCapture(
		input,
		time.Date(2026, time.August, 25, 9, 14, int(seed), 123456000, time.UTC),
		bytes.NewReader(bytes.Repeat([]byte{seed}, 16)),
	)
	if err != nil {
		t.Fatalf("NewCapture() error = %v", err)
	}
	return record
}

func assertRecordAndTagCounts(t *testing.T, db *sql.DB, wantRecords, wantTags int) {
	t.Helper()
	var records, tags int
	if err := db.QueryRow(`SELECT COUNT(*) FROM records`).Scan(&records); err != nil {
		t.Fatalf("read record count error = %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM record_tags`).Scan(&tags); err != nil {
		t.Fatalf("read tag count error = %v", err)
	}
	if records != wantRecords || tags != wantTags {
		t.Errorf("record/tag counts = %d/%d, want %d/%d", records, tags, wantRecords, wantTags)
	}
}
