//go:build linux || darwin

package repository

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/storage"
)

func TestCreateCaptureStoresEveryType(t *testing.T) {
	tests := []struct {
		captureType    domain.CaptureType
		wantProject    sql.NullString
		wantFrequency  sql.NullString
		wantImpact     sql.NullString
		wantCategory   sql.NullString
		wantWorkaround sql.NullString
	}{
		{
			captureType:    domain.CaptureTypeFriction,
			wantProject:    sql.NullString{String: "forge", Valid: true},
			wantFrequency:  sql.NullString{String: "weekly", Valid: true},
			wantImpact:     sql.NullString{String: "high", Valid: true},
			wantCategory:   sql.NullString{String: "verification", Valid: true},
			wantWorkaround: sql.NullString{String: "Use a checklist", Valid: true},
		},
		{captureType: domain.CaptureTypeAction},
		{captureType: domain.CaptureTypeFollowUp},
		{captureType: domain.CaptureTypeDecision},
	}

	for index, tt := range tests {
		t.Run(tt.captureType.String(), func(t *testing.T) {
			repository, db := openTestRepository(t)
			capture := newTestCapture(t, tt.captureType, byte(index))
			if err := repository.CreateCapture(context.Background(), capture); err != nil {
				t.Fatalf("CreateCapture() error = %v", err)
			}

			var id, captureType, description, createdAt, updatedAt string
			var project, frequency, impact, category, workaround sql.NullString
			if err := db.QueryRow(`SELECT
				id, capture_type, description, friction_project,
				friction_frequency, friction_impact, friction_category,
				friction_current_workaround, created_at, updated_at
				FROM records WHERE id = ?`, capture.ID.String()).Scan(
				&id, &captureType, &description, &project,
				&frequency, &impact, &category, &workaround, &createdAt, &updatedAt,
			); err != nil {
				t.Fatalf("read stored capture error = %v", err)
			}
			if id != capture.ID.String() || captureType != capture.Type.String() || description != capture.Description {
				t.Errorf("stored identity = %q/%q/%q", id, captureType, description)
			}
			if project != tt.wantProject || frequency != tt.wantFrequency || impact != tt.wantImpact ||
				category != tt.wantCategory || workaround != tt.wantWorkaround {
				t.Errorf("stored details = %#v/%#v/%#v/%#v/%#v", project, frequency, impact, category, workaround)
			}
			if createdAt != capture.CreatedAt.String() || updatedAt != capture.UpdatedAt.String() {
				t.Errorf("stored timestamps = %q/%q", createdAt, updatedAt)
			}
		})
	}
}

func TestCreateCaptureStoresAbsentOptionalFrictionText(t *testing.T) {
	repository, db := openTestRepository(t)
	capture := newTestCapture(t, domain.CaptureTypeFriction, 4)
	capture.Details.Friction.Project = nil
	capture.Details.Friction.CurrentWorkaround = nil
	if err := repository.CreateCapture(context.Background(), capture); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}
	var project, workaround sql.NullString
	if err := db.QueryRow(`SELECT friction_project, friction_current_workaround
		FROM records WHERE id = ?`, capture.ID.String()).Scan(&project, &workaround); err != nil {
		t.Fatalf("read optional friction text error = %v", err)
	}
	if project.Valid || workaround.Valid {
		t.Errorf("stored optional friction text = %#v/%#v, want NULL/NULL", project, workaround)
	}
}

func TestCreateCaptureStoresMigratedFrictionID(t *testing.T) {
	repository, db := openTestRepository(t)
	capture := newTestCapture(t, domain.CaptureTypeFriction, 8)
	capture.ID = "frc_08080808080808080808080808080808"
	if err := repository.CreateCapture(context.Background(), capture); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}
	var captureType string
	if err := db.QueryRow(`SELECT capture_type FROM records WHERE id = ?`, capture.ID.String()).Scan(&captureType); err != nil {
		t.Fatalf("read migrated friction error = %v", err)
	}
	if captureType != "friction" {
		t.Errorf("capture type = %q, want friction", captureType)
	}
}

func TestCreateCaptureDuplicatePreservesOriginal(t *testing.T) {
	repository, db := openTestRepository(t)
	original := newTestCapture(t, domain.CaptureTypeAction, 5)
	if err := repository.CreateCapture(context.Background(), original); err != nil {
		t.Fatalf("first CreateCapture() error = %v", err)
	}
	duplicate := original
	duplicate.Description = "Different description"
	err := repository.CreateCapture(context.Background(), duplicate)
	if err == nil || !strings.Contains(err.Error(), "insert capture "+original.ID.String()) {
		t.Fatalf("duplicate CreateCapture() error = %v", err)
	}
	var count int
	var description string
	if err := db.QueryRow(`SELECT COUNT(*), description FROM records WHERE id = ?`, original.ID.String()).Scan(&count, &description); err != nil {
		t.Fatalf("read duplicate state error = %v", err)
	}
	if count != 1 || description != original.Description {
		t.Errorf("duplicate state = %d/%q, want 1/%q", count, description, original.Description)
	}
}

func TestCreateCaptureRejectsInvalidBeforeDatabaseAccess(t *testing.T) {
	repository, db := openTestRepository(t)
	capture := newTestCapture(t, domain.CaptureTypeDecision, 6)
	capture.Description = " invalid "
	err := repository.CreateCapture(context.Background(), capture)
	var invalid *domain.InvalidValueError
	if !errors.As(err, &invalid) || !strings.Contains(err.Error(), "validate capture") {
		t.Fatalf("CreateCapture() error = %T %v, want wrapped validation error", err, err)
	}
	assertRecordCount(t, db, 0)
}

func TestCreateCaptureHonorsCancelledContext(t *testing.T) {
	repository, db := openTestRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := repository.CreateCapture(ctx, newTestCapture(t, domain.CaptureTypeFollowUp, 7))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateCapture() error = %v, want context.Canceled", err)
	}
	assertRecordCount(t, db, 0)
}

func openTestRepository(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()
	directory, err := storage.PrepareDataDirectory(filepath.Join(t.TempDir(), "forge"), os.Geteuid())
	if err != nil {
		t.Fatalf("PrepareDataDirectory() error = %v", err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	database, err := storage.OpenDatabaseFile(directory, storage.DatabaseCreate, os.Geteuid())
	if err != nil {
		t.Fatalf("OpenDatabaseFile() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	db, err := storage.OpenSQLite(context.Background(), directory, database, storage.DatabaseCreate)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := storage.ApplyMigrations(context.Background(), db); err != nil {
		t.Fatalf("ApplyMigrations() error = %v", err)
	}
	repository, err := New(db)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return repository, db
}

func newTestCapture(t *testing.T, captureType domain.CaptureType, seed byte) domain.Capture {
	t.Helper()
	input := domain.ProposedCaptureInput{
		Type:        captureType,
		Description: " " + captureType.String(),
	}
	switch captureType {
	case domain.CaptureTypeFriction:
		input.Details.Friction = &domain.FrictionCaptureInput{
			Project: "forge", Frequency: domain.FrequencyWeekly,
			Impact: domain.ImpactHigh, Category: domain.CategoryVerification,
			CurrentWorkaround: "Use a checklist",
		}
	case domain.CaptureTypeAction:
		input.Details.Action = &domain.ActionCaptureDetails{}
	case domain.CaptureTypeFollowUp:
		input.Details.FollowUp = &domain.FollowUpCaptureDetails{}
	case domain.CaptureTypeDecision:
		input.Details.Decision = &domain.DecisionCaptureDetails{}
	default:
		t.Fatalf("unsupported capture type %q", captureType)
	}
	proposed, err := domain.NewProposedCapture(input)
	if err != nil {
		t.Fatalf("NewProposedCapture() error = %v", err)
	}
	capture, err := domain.NewPersistedCapture(
		proposed,
		time.Date(2026, time.August, 25, 12, 0, int(seed), 123456000, time.UTC),
		bytes.NewReader(bytes.Repeat([]byte{seed}, 16)),
	)
	if err != nil {
		t.Fatalf("NewPersistedCapture() error = %v", err)
	}
	return capture
}

func assertRecordCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM records`).Scan(&got); err != nil {
		t.Fatalf("read record count error = %v", err)
	}
	if got != want {
		t.Errorf("record count = %d, want %d", got, want)
	}
}
