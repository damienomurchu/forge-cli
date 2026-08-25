package domain

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRecordWithStatusChangesOnlyStatusAndUpdatedAt(t *testing.T) {
	record := validCaptureRecord()
	original := record
	now := time.Date(
		2026, time.August, 26, 12, 32, 19, 987654999,
		time.FixedZone("test", 90*60),
	)

	got, err := record.WithStatus(StatusCandidate, now)
	if err != nil {
		t.Fatalf("Record.WithStatus() error = %v", err)
	}
	want := record
	want.Status = StatusCandidate
	want.UpdatedAt = NewTimestamp(now)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Record.WithStatus() = %#v, want %#v", got, want)
	}
	if got.UpdatedAt.String() != "2026-08-26T11:02:19.987654Z" {
		t.Errorf("updated_at = %q", got.UpdatedAt)
	}
	if !reflect.DeepEqual(record, original) {
		t.Errorf("Record.WithStatus() mutated its receiver")
	}
	if err := got.Validate(); err != nil {
		t.Errorf("Record.WithStatus() produced invalid record: %v", err)
	}
}

func TestRecordWithStatusNoOpPreservesRecordExactly(t *testing.T) {
	record := validFrictionRecord()
	nowBeforeCreation := record.CreatedAt.Time().Add(-time.Hour)

	got, err := record.WithStatus(record.Status, nowBeforeCreation)
	if err != nil {
		t.Fatalf("Record.WithStatus() error = %v", err)
	}
	if !reflect.DeepEqual(got, record) {
		t.Errorf("Record.WithStatus() = %#v, want unchanged %#v", got, record)
	}
}

func TestRecordWithStatusRejectsInvalidStatus(t *testing.T) {
	got, err := validCaptureRecord().WithStatus("invalid", time.Now())
	assertFailedStatusUpdate(t, got, err, "status")
}

func TestRecordWithStatusRejectsInvalidSourceRecord(t *testing.T) {
	record := validCaptureRecord()
	record.Description = " invalid "

	got, err := record.WithStatus(StatusReviewing, time.Now())
	assertFailedStatusUpdate(t, got, err, "description")
}

func TestRecordWithStatusRejectsTimeBeforeCreation(t *testing.T) {
	record := validCaptureRecord()
	now := record.CreatedAt.Time().Add(-time.Microsecond)

	got, err := record.WithStatus(StatusReviewing, now)
	assertFailedStatusUpdate(t, got, err, "updated_at")
}

func assertFailedStatusUpdate(t *testing.T, got Record, err error, wantField string) {
	t.Helper()
	var invalid *InvalidValueError
	if !errors.As(err, &invalid) {
		t.Fatalf("Record.WithStatus() error = %T %v, want *InvalidValueError", err, err)
	}
	if invalid.Field != wantField {
		t.Errorf("error field = %q, want %q", invalid.Field, wantField)
	}
	if !reflect.DeepEqual(got, Record{}) {
		t.Errorf("Record.WithStatus() = %#v, want zero record", got)
	}
}
