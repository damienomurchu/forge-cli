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

func TestReviewReturnsOnlyActionableFrictionNewestFirst(t *testing.T) {
	repository, _ := openTestRepository(t)
	statuses := []domain.Status{
		domain.StatusCaptured,
		domain.StatusReviewing,
		domain.StatusCandidate,
		domain.StatusAutomated,
		domain.StatusDismissed,
	}
	friction := make([]domain.Record, 0, len(statuses))
	for index, status := range statuses {
		record := newTestFriction(t, domain.FrictionInput{
			Description: "Friction " + status.String(),
			Frequency:   domain.FrequencyWeekly,
			Impact:      domain.ImpactMedium,
			Category:    domain.CategoryRepeatedAction,
		}, byte(60+index))
		record.Status = status
		timestamp := domain.NewTimestamp(time.Date(
			2026, time.August, 25, 10, index, 0, 0, time.UTC,
		))
		record.CreatedAt, record.UpdatedAt = timestamp, timestamp
		if err := repository.CreateFriction(context.Background(), record); err != nil {
			t.Fatalf("CreateFriction(%s) error = %v", status, err)
		}
		friction = append(friction, record)
	}
	capture := newTestCapture(t, domain.CaptureInput{
		Description: "Captured capture is not review friction",
		Kind:        domain.CaptureKindThought,
	}, 70)
	captureTime := domain.NewTimestamp(time.Date(2026, time.August, 26, 10, 0, 0, 0, time.UTC))
	capture.CreatedAt, capture.UpdatedAt = captureTime, captureTime
	if err := repository.CreateCapture(context.Background(), capture); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}

	got, err := repository.Review(context.Background())
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	want := []domain.Record{friction[2], friction[1], friction[0]}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Review() = %#v, want %#v", got, want)
	}
}

func TestReviewReturnsNonNilEmptySlice(t *testing.T) {
	repository, _ := openTestRepository(t)
	records, err := repository.Review(context.Background())
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if records == nil || len(records) != 0 {
		t.Errorf("Review() = %#v, want non-nil empty slice", records)
	}
}

func TestReviewFailsForMalformedIncludedRecord(t *testing.T) {
	repository, db := openTestRepository(t)
	record := newTestFriction(t, domain.FrictionInput{
		Description: "Malformed actionable friction",
		Frequency:   domain.FrequencyDaily,
		Impact:      domain.ImpactHigh,
		Category:    domain.CategoryWaiting,
	}, 71)
	record.Status = domain.StatusCandidate
	if err := repository.CreateFriction(context.Background(), record); err != nil {
		t.Fatalf("CreateFriction() error = %v", err)
	}
	if _, err := db.Exec(
		`UPDATE records SET created_at = 'malformed' WHERE id = ?`,
		record.ID.String(),
	); err != nil {
		t.Fatalf("corrupt timestamp error = %v", err)
	}

	got, err := repository.Review(context.Background())
	if got != nil {
		t.Errorf("Review() = %#v, want nil", got)
	}
	if err == nil || !strings.Contains(err.Error(), "review friction: decode record") {
		t.Fatalf("Review() error = %v, want decode error", err)
	}
}

func TestReviewDoesNotDecodeExcludedMalformedRows(t *testing.T) {
	repository, db := openTestRepository(t)
	excludedFriction := newTestFriction(t, domain.FrictionInput{
		Description: "Malformed dismissed friction",
		Frequency:   domain.FrequencyMonthly,
		Impact:      domain.ImpactLow,
		Category:    domain.CategoryOther,
	}, 72)
	excludedFriction.Status = domain.StatusDismissed
	if err := repository.CreateFriction(context.Background(), excludedFriction); err != nil {
		t.Fatalf("CreateFriction() error = %v", err)
	}
	excludedCapture := newTestCapture(t, domain.CaptureInput{
		Description: "Malformed capture",
		Kind:        domain.CaptureKindSeed,
	}, 73)
	if err := repository.CreateCapture(context.Background(), excludedCapture); err != nil {
		t.Fatalf("CreateCapture() error = %v", err)
	}
	if _, err := db.Exec(
		`UPDATE records SET updated_at = 'malformed' WHERE id IN (?, ?)`,
		excludedFriction.ID.String(),
		excludedCapture.ID.String(),
	); err != nil {
		t.Fatalf("corrupt excluded rows error = %v", err)
	}

	records, err := repository.Review(context.Background())
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if records == nil || len(records) != 0 {
		t.Errorf("Review() = %#v, want non-nil empty slice", records)
	}
}
