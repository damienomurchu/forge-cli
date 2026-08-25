//go:build linux || darwin

package repository

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestListReturnsNonNilEmptySlice(t *testing.T) {
	repository, _ := openTestRepository(t)
	records, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if records == nil || len(records) != 0 {
		t.Errorf("List() = %#v, want non-nil empty slice", records)
	}
}

func TestListReturnsMixedRecordsNewestFirst(t *testing.T) {
	repository, _ := openTestRepository(t)
	older := newTestCapture(t, domain.CaptureInput{
		Description: "Older capture",
		Kind:        domain.CaptureKindThought,
		Tags:        "older,ordered",
	}, 30)
	newerCapture := newTestCapture(t, domain.CaptureInput{
		Description: "Newer capture",
		Kind:        domain.CaptureKindObservation,
		Tags:        "first,second",
	}, 31)
	newerFriction := newTestFriction(t, domain.FrictionInput{
		Description: "Newer friction",
		Frequency:   domain.FrequencyDaily,
		Impact:      domain.ImpactHigh,
		Category:    domain.CategoryContextSwitching,
	}, 32)

	olderTime := domain.NewTimestamp(time.Date(2026, time.August, 24, 9, 0, 0, 0, time.UTC))
	newerTime := domain.NewTimestamp(time.Date(2026, time.August, 25, 9, 0, 0, 0, time.UTC))
	older.CreatedAt, older.UpdatedAt = olderTime, olderTime
	newerCapture.CreatedAt, newerCapture.UpdatedAt = newerTime, newerTime
	newerFriction.CreatedAt, newerFriction.UpdatedAt = newerTime, newerTime

	if err := repository.CreateCapture(context.Background(), newerCapture); err != nil {
		t.Fatalf("CreateCapture(newer) error = %v", err)
	}
	if err := repository.CreateCapture(context.Background(), older); err != nil {
		t.Fatalf("CreateCapture(older) error = %v", err)
	}
	if err := repository.CreateFriction(context.Background(), newerFriction); err != nil {
		t.Fatalf("CreateFriction() error = %v", err)
	}

	got, err := repository.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []domain.Record{newerFriction, newerCapture, older}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %#v, want %#v", got, want)
	}
}

func TestListFailsWholeOperationForMalformedRecord(t *testing.T) {
	repository, db := openTestRepository(t)
	valid := newTestCapture(t, domain.CaptureInput{
		Description: "Valid capture",
		Kind:        domain.CaptureKindIdea,
	}, 33)
	malformed := newTestFriction(t, domain.FrictionInput{
		Description: "Malformed friction",
		Frequency:   domain.FrequencyMonthly,
		Impact:      domain.ImpactMedium,
		Category:    domain.CategoryVerification,
	}, 34)
	if err := repository.CreateCapture(context.Background(), valid); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}
	if err := repository.CreateFriction(context.Background(), malformed); err != nil {
		t.Fatalf("CreateFriction() error = %v", err)
	}
	if _, err := db.Exec(
		`UPDATE records SET updated_at = 'malformed' WHERE id = ?`,
		malformed.ID.String(),
	); err != nil {
		t.Fatalf("corrupt timestamp error = %v", err)
	}

	got, err := repository.List(context.Background())
	if got != nil {
		t.Errorf("List() = %#v, want nil after failure", got)
	}
	if err == nil || !strings.Contains(err.Error(), "decode listed record") {
		t.Fatalf("List() error = %v, want decode error", err)
	}
}
