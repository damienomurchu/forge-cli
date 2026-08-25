//go:build linux || darwin

package repository

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestUpdateStatusChangesCaptureAndPreservesOtherFields(t *testing.T) {
	repository, _ := openTestRepository(t)
	original := newTestCapture(t, domain.CaptureInput{
		Description: "Update capture status",
		Project:     "forge",
		Kind:        domain.CaptureKindDecision,
		Tags:        "preserved,order",
	}, 50)
	if err := repository.CreateCapture(context.Background(), original); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}
	now := time.Date(2026, time.August, 26, 10, 11, 12, 123456789, time.UTC)
	want, err := original.WithStatus(domain.StatusCandidate, now)
	if err != nil {
		t.Fatalf("WithStatus() error = %v", err)
	}

	got, changed, err := repository.UpdateStatus(
		context.Background(), original.ID, domain.StatusCandidate, now,
	)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if !changed {
		t.Errorf("UpdateStatus() changed = false, want true")
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UpdateStatus() record = %#v, want %#v", got, want)
	}
	stored, err := repository.FindByID(context.Background(), original.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if !reflect.DeepEqual(stored, want) {
		t.Errorf("stored record = %#v, want %#v", stored, want)
	}
}

func TestUpdateStatusChangesFriction(t *testing.T) {
	repository, _ := openTestRepository(t)
	original := newTestFriction(t, domain.FrictionInput{
		Description:       "Update friction status",
		Project:           "forge",
		Frequency:         domain.FrequencyDaily,
		Impact:            domain.ImpactHigh,
		Category:          domain.CategoryRepeatedAction,
		CurrentWorkaround: "Keep doing it manually",
	}, 51)
	if err := repository.CreateFriction(context.Background(), original); err != nil {
		t.Fatalf("CreateFriction() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 8, 0, 0, 0, time.UTC)
	want, err := original.WithStatus(domain.StatusAutomated, now)
	if err != nil {
		t.Fatalf("WithStatus() error = %v", err)
	}

	got, changed, err := repository.UpdateStatus(
		context.Background(), original.ID, domain.StatusAutomated, now,
	)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if !changed || !reflect.DeepEqual(got, want) {
		t.Errorf("UpdateStatus() = %#v/%v, want %#v/true", got, changed, want)
	}
}

func TestUpdateStatusNoOpPreservesTimestamp(t *testing.T) {
	repository, _ := openTestRepository(t)
	original := newTestCapture(t, domain.CaptureInput{
		Description: "No-op update",
		Kind:        domain.CaptureKindThought,
	}, 52)
	if err := repository.CreateCapture(context.Background(), original); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}

	got, changed, err := repository.UpdateStatus(
		context.Background(), original.ID, original.Status, time.Time{},
	)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if changed {
		t.Errorf("UpdateStatus() changed = true, want false")
	}
	if !reflect.DeepEqual(got, original) {
		t.Errorf("UpdateStatus() record = %#v, want original", got)
	}
	stored, err := repository.FindByID(context.Background(), original.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if !reflect.DeepEqual(stored, original) {
		t.Errorf("stored no-op record = %#v, want original", stored)
	}
}

func TestUpdateStatusReturnsNotFound(t *testing.T) {
	repository, _ := openTestRepository(t)
	got, changed, err := repository.UpdateStatus(
		context.Background(), domain.ID("missing"), domain.StatusReviewing, time.Now(),
	)
	if got != (domain.Record{}) || changed {
		t.Errorf("UpdateStatus() = %#v/%v, want zero/false", got, changed)
	}
	if !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("UpdateStatus() error = %v, want ErrRecordNotFound", err)
	}
}

func TestUpdateStatusRejectsInvalidStatusBeforeDatabaseAccess(t *testing.T) {
	repository, db := openTestRepository(t)
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}
	got, changed, err := repository.UpdateStatus(
		context.Background(), domain.ID("anything"), domain.Status("invalid"), time.Now(),
	)
	if got != (domain.Record{}) || changed {
		t.Errorf("UpdateStatus() = %#v/%v, want zero/false", got, changed)
	}
	if err == nil || !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("UpdateStatus() error = %v, want invalid status", err)
	}
}

func TestUpdateStatusRejectsTimestampBeforeCreation(t *testing.T) {
	repository, _ := openTestRepository(t)
	original := newTestFriction(t, domain.FrictionInput{
		Description: "Reject old timestamp",
		Frequency:   domain.FrequencyMonthly,
		Impact:      domain.ImpactMedium,
		Category:    domain.CategoryWaiting,
	}, 53)
	if err := repository.CreateFriction(context.Background(), original); err != nil {
		t.Fatalf("CreateFriction() error = %v", err)
	}

	got, changed, err := repository.UpdateStatus(
		context.Background(),
		original.ID,
		domain.StatusDismissed,
		original.CreatedAt.Time().Add(-time.Microsecond),
	)
	if got != (domain.Record{}) || changed {
		t.Errorf("UpdateStatus() = %#v/%v, want zero/false", got, changed)
	}
	if err == nil || !strings.Contains(err.Error(), "invalid updated_at") {
		t.Fatalf("UpdateStatus() error = %v, want timestamp validation", err)
	}
	stored, err := repository.FindByID(context.Background(), original.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if !reflect.DeepEqual(stored, original) {
		t.Errorf("stored rejected update = %#v, want original", stored)
	}
}

func TestUpdateStatusRollsBackDatabaseFailure(t *testing.T) {
	repository, db := openTestRepository(t)
	original := newTestCapture(t, domain.CaptureInput{
		Description: "Rollback status update",
		Kind:        domain.CaptureKindObservation,
		Tags:        "still,present",
	}, 54)
	if err := repository.CreateCapture(context.Background(), original); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}
	if _, err := db.Exec(`CREATE TRIGGER reject_status_update
		BEFORE UPDATE OF status ON records
		BEGIN SELECT RAISE(ABORT, 'rejected test update'); END`); err != nil {
		t.Fatalf("create test trigger error = %v", err)
	}

	got, changed, err := repository.UpdateStatus(
		context.Background(), original.ID, domain.StatusReviewing,
		time.Date(2026, time.August, 28, 9, 0, 0, 0, time.UTC),
	)
	if got != (domain.Record{}) || changed {
		t.Errorf("UpdateStatus() = %#v/%v, want zero/false", got, changed)
	}
	if err == nil || !strings.Contains(err.Error(), "update record status") {
		t.Fatalf("UpdateStatus() error = %v, want update error", err)
	}
	stored, err := repository.FindByID(context.Background(), original.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if !reflect.DeepEqual(stored, original) {
		t.Errorf("stored failed update = %#v, want original", stored)
	}
}
