//go:build linux || darwin

package repository

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestCreateFrictionStoresCompleteRecord(t *testing.T) {
	repository, db := openTestRepository(t)
	record := newTestFriction(t, domain.FrictionInput{
		Description:       "Releases require repeated manual checks",
		Project:           "forge",
		Frequency:         domain.FrequencyMonthly,
		Impact:            domain.ImpactHigh,
		Category:          domain.CategoryVerification,
		CurrentWorkaround: "Follow a handwritten checklist",
	}, 10)

	if err := repository.CreateFriction(context.Background(), record); err != nil {
		t.Fatalf("CreateFriction() error = %v", err)
	}

	var (
		id, recordType, description, project, status string
		captureKind                                  sql.NullString
		frequency, impact, category, workaround      string
		createdAt, updatedAt                         string
	)
	if err := db.QueryRow(`SELECT
		id, type, description, project, status, capture_kind,
		friction_frequency, friction_impact, friction_category,
		current_workaround, created_at, updated_at
		FROM records WHERE id = ?`, record.ID.String()).Scan(
		&id, &recordType, &description, &project, &status, &captureKind,
		&frequency, &impact, &category, &workaround, &createdAt, &updatedAt,
	); err != nil {
		t.Fatalf("read stored friction error = %v", err)
	}
	if id != record.ID.String() || recordType != "friction" || description != record.Description {
		t.Errorf("stored identity = %q/%q/%q", id, recordType, description)
	}
	if project != "forge" || status != "captured" || captureKind.Valid {
		t.Errorf("stored shared/capture values = %q/%q/%#v", project, status, captureKind)
	}
	if frequency != "monthly" || impact != "high" || category != "verification" {
		t.Errorf("stored classification = %q/%q/%q", frequency, impact, category)
	}
	if workaround != "Follow a handwritten checklist" {
		t.Errorf("stored workaround = %q", workaround)
	}
	if createdAt != record.CreatedAt.String() || updatedAt != record.UpdatedAt.String() {
		t.Errorf("stored timestamps = %q/%q", createdAt, updatedAt)
	}
	assertRecordAndTagCounts(t, db, 1, 0)
}

func TestCreateFrictionStoresAbsentOptionalValues(t *testing.T) {
	repository, db := openTestRepository(t)
	record := newTestFriction(t, domain.FrictionInput{
		Description: "Waiting for a slow build",
		Frequency:   domain.FrequencyDaily,
		Impact:      domain.ImpactMedium,
		Category:    domain.CategoryWaiting,
	}, 11)

	if err := repository.CreateFriction(context.Background(), record); err != nil {
		t.Fatalf("CreateFriction() error = %v", err)
	}
	var project, workaround sql.NullString
	if err := db.QueryRow(
		`SELECT project, current_workaround FROM records WHERE id = ?`,
		record.ID.String(),
	).Scan(&project, &workaround); err != nil {
		t.Fatalf("read optional values error = %v", err)
	}
	if project.Valid || workaround.Valid {
		t.Errorf("optional values = %#v/%#v, want NULL", project, workaround)
	}
	assertRecordAndTagCounts(t, db, 1, 0)
}

func TestCreateFrictionRollsBackDuplicateID(t *testing.T) {
	repository, db := openTestRepository(t)
	record := newTestFriction(t, domain.FrictionInput{
		Description: "Original friction",
		Frequency:   domain.FrequencyWeekly,
		Impact:      domain.ImpactLow,
		Category:    domain.CategoryRepeatedAction,
	}, 12)
	if err := repository.CreateFriction(context.Background(), record); err != nil {
		t.Fatalf("first CreateFriction() error = %v", err)
	}

	duplicate := record
	duplicate.Description = "Duplicate friction"
	duplicate.Details.Friction = &domain.FrictionDetails{
		Frequency: domain.FrequencyDaily,
		Impact:    domain.ImpactHigh,
		Category:  domain.CategoryOther,
	}
	err := repository.CreateFriction(context.Background(), duplicate)
	if err == nil || !strings.Contains(err.Error(), "insert friction "+record.ID.String()) {
		t.Fatalf("duplicate CreateFriction() error = %v", err)
	}
	assertRecordAndTagCounts(t, db, 1, 0)

	var description string
	if err := db.QueryRow(`SELECT description FROM records WHERE id = ?`, record.ID.String()).Scan(&description); err != nil {
		t.Fatalf("read description error = %v", err)
	}
	if description != "Original friction" {
		t.Errorf("stored description = %q, want original", description)
	}
}

func TestCreateFrictionRollsBackConstraintFailure(t *testing.T) {
	repository, db := openTestRepository(t)
	if _, err := db.Exec(`CREATE TRIGGER reject_test_friction
		BEFORE INSERT ON records WHEN NEW.description = 'Rejected friction'
		BEGIN SELECT RAISE(ABORT, 'rejected test friction'); END`); err != nil {
		t.Fatalf("create test trigger error = %v", err)
	}
	record := newTestFriction(t, domain.FrictionInput{
		Description: "Rejected friction",
		Frequency:   domain.FrequencyUnknown,
		Impact:      domain.ImpactUnknown,
		Category:    domain.CategoryOther,
	}, 13)

	err := repository.CreateFriction(context.Background(), record)
	if err == nil || !strings.Contains(err.Error(), "insert friction") {
		t.Fatalf("CreateFriction() error = %v, want insert error", err)
	}
	assertRecordAndTagCounts(t, db, 0, 0)
}

func TestCreateFrictionRejectsInvalidRecord(t *testing.T) {
	repository, db := openTestRepository(t)
	record := newTestFriction(t, domain.FrictionInput{
		Description: "Initially valid",
		Frequency:   domain.FrequencyOccasional,
		Impact:      domain.ImpactLow,
		Category:    domain.CategoryRemembering,
	}, 14)
	record.Details.Friction.Impact = "invalid"

	err := repository.CreateFriction(context.Background(), record)
	if err == nil || !strings.Contains(err.Error(), "validate friction") {
		t.Fatalf("CreateFriction() error = %v, want validation error", err)
	}
	assertRecordAndTagCounts(t, db, 0, 0)
}

func TestCreateFrictionRejectsCapture(t *testing.T) {
	repository, db := openTestRepository(t)
	record := newTestCapture(t, domain.CaptureInput{
		Description: "Wrong record type",
		Kind:        domain.CaptureKindThought,
	}, 15)

	err := repository.CreateFriction(context.Background(), record)
	if err == nil || err.Error() != `validate friction: record type is "capture"` {
		t.Fatalf("CreateFriction() error = %v", err)
	}
	assertRecordAndTagCounts(t, db, 0, 0)
}

func newTestFriction(t *testing.T, input domain.FrictionInput, seed byte) domain.Record {
	t.Helper()
	record, err := domain.NewFriction(
		input,
		time.Date(2026, time.August, 25, 9, 18, int(seed), 654321000, time.UTC),
		bytes.NewReader(bytes.Repeat([]byte{seed}, 16)),
	)
	if err != nil {
		t.Fatalf("NewFriction() error = %v", err)
	}
	return record
}
