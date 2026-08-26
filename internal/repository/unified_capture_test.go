//go:build linux || darwin

package repository

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
	forgemigrations "github.com/damienomurchu/forge-cli/migrations"
)

func TestCreateUnifiedCaptureStoresEveryType(t *testing.T) {
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
			repository, db := openUnifiedTestRepository(t)
			capture := newUnifiedTestCapture(t, tt.captureType, byte(index))
			if err := repository.CreateUnifiedCapture(context.Background(), capture); err != nil {
				t.Fatalf("CreateUnifiedCapture() error = %v", err)
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
				t.Fatalf("read stored unified capture error = %v", err)
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

func TestCreateUnifiedCaptureStoresAbsentOptionalFrictionText(t *testing.T) {
	repository, db := openUnifiedTestRepository(t)
	capture := newUnifiedTestCapture(t, domain.CaptureTypeFriction, 4)
	capture.Details.Friction.Project = nil
	capture.Details.Friction.CurrentWorkaround = nil
	if err := repository.CreateUnifiedCapture(context.Background(), capture); err != nil {
		t.Fatalf("CreateUnifiedCapture() error = %v", err)
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

func TestCreateUnifiedCaptureStoresMigratedFrictionID(t *testing.T) {
	repository, db := openUnifiedTestRepository(t)
	capture := newUnifiedTestCapture(t, domain.CaptureTypeFriction, 8)
	capture.ID = "frc_08080808080808080808080808080808"
	if err := repository.CreateUnifiedCapture(context.Background(), capture); err != nil {
		t.Fatalf("CreateUnifiedCapture() error = %v", err)
	}
	var captureType string
	if err := db.QueryRow(`SELECT capture_type FROM records WHERE id = ?`, capture.ID.String()).Scan(&captureType); err != nil {
		t.Fatalf("read migrated friction error = %v", err)
	}
	if captureType != "friction" {
		t.Errorf("capture type = %q, want friction", captureType)
	}
}

func TestCreateUnifiedCaptureDuplicatePreservesOriginal(t *testing.T) {
	repository, db := openUnifiedTestRepository(t)
	original := newUnifiedTestCapture(t, domain.CaptureTypeAction, 5)
	if err := repository.CreateUnifiedCapture(context.Background(), original); err != nil {
		t.Fatalf("first CreateUnifiedCapture() error = %v", err)
	}
	duplicate := original
	duplicate.Description = "Different description"
	err := repository.CreateUnifiedCapture(context.Background(), duplicate)
	if err == nil || !strings.Contains(err.Error(), "insert unified capture "+original.ID.String()) {
		t.Fatalf("duplicate CreateUnifiedCapture() error = %v", err)
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

func TestCreateUnifiedCaptureRejectsInvalidBeforeDatabaseAccess(t *testing.T) {
	repository, db := openUnifiedTestRepository(t)
	capture := newUnifiedTestCapture(t, domain.CaptureTypeDecision, 6)
	capture.Description = " invalid "
	err := repository.CreateUnifiedCapture(context.Background(), capture)
	var invalid *domain.InvalidValueError
	if !errors.As(err, &invalid) || !strings.Contains(err.Error(), "validate unified capture") {
		t.Fatalf("CreateUnifiedCapture() error = %T %v, want wrapped validation error", err, err)
	}
	assertUnifiedRecordCount(t, db, 0)
}

func TestCreateUnifiedCaptureHonorsCancelledContext(t *testing.T) {
	repository, db := openUnifiedTestRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := repository.CreateUnifiedCapture(ctx, newUnifiedTestCapture(t, domain.CaptureTypeFollowUp, 7))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateUnifiedCapture() error = %v, want context.Canceled", err)
	}
	assertUnifiedRecordCount(t, db, 0)
}

func openUnifiedTestRepository(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()
	repository, db := openTestRepository(t)
	query, err := forgemigrations.Files.ReadFile("002_unified_captures.sql")
	if err != nil {
		t.Fatalf("read unified migration error = %v", err)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin unified migration error = %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(string(query)); err != nil {
		t.Fatalf("apply unified migration error = %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations(version, name, applied_at)
		VALUES (2, '002_unified_captures.sql', '2026-08-25T12:00:00.000Z')`); err != nil {
		t.Fatalf("record unified migration error = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit unified migration error = %v", err)
	}
	return repository, db
}

func newUnifiedTestCapture(t *testing.T, captureType domain.CaptureType, seed byte) domain.Capture {
	t.Helper()
	input := domain.ProposedCaptureInput{
		Type:        captureType,
		Description: "Unified " + captureType.String(),
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

func assertUnifiedRecordCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM records`).Scan(&got); err != nil {
		t.Fatalf("read unified record count error = %v", err)
	}
	if got != want {
		t.Errorf("unified record count = %d, want %d", got, want)
	}
}
