package domain

import (
	"errors"
	"testing"
	"time"
)

func TestRecordValidateAcceptsCanonicalRecords(t *testing.T) {
	emptyCapture := mutateCapture(func(r *Record) {
		r.Project = nil
		r.Details.Capture.Tags = []string{}
	})
	frictionWithoutWorkaround := mutateFriction(func(r *Record) {
		r.Details.Friction.CurrentWorkaround = nil
	})
	tests := []struct {
		name   string
		record Record
	}{
		{name: "capture", record: validCaptureRecord()},
		{name: "capture without optional values", record: emptyCapture},
		{name: "friction", record: validFrictionRecord()},
		{name: "friction without workaround", record: frictionWithoutWorkaround},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.record.Validate(); err != nil {
				t.Errorf("Record.Validate() error = %v", err)
			}
		})
	}
}

func TestRecordValidateRejectsInvalidRecords(t *testing.T) {
	tests := []struct {
		name      string
		record    Record
		wantField string
	}{
		{name: "invalid type", record: mutateCapture(func(r *Record) { r.Type = "invalid" }), wantField: "record type"},
		{name: "ID does not match type", record: mutateCapture(func(r *Record) { r.ID = validFrictionRecord().ID }), wantField: "record ID"},
		{name: "blank description", record: mutateCapture(func(r *Record) { r.Description = " \t" }), wantField: "description"},
		{name: "unnormalized description", record: mutateCapture(func(r *Record) { r.Description = " capture this " }), wantField: "description"},
		{name: "empty project", record: mutateCapture(func(r *Record) { r.Project = stringPointer("") }), wantField: "project"},
		{name: "unnormalized project", record: mutateCapture(func(r *Record) { r.Project = stringPointer(" forge ") }), wantField: "project"},
		{name: "invalid status", record: mutateCapture(func(r *Record) { r.Status = "invalid" }), wantField: "status"},
		{name: "updated before created", record: mutateCapture(func(r *Record) { r.UpdatedAt = NewTimestamp(r.CreatedAt.Time().Add(-time.Microsecond)) }), wantField: "updated_at"},
		{name: "capture details missing", record: mutateCapture(func(r *Record) { r.Details.Capture = nil }), wantField: "details"},
		{name: "capture has friction details", record: mutateCapture(func(r *Record) { r.Details.Friction = validFrictionRecord().Details.Friction }), wantField: "details"},
		{name: "invalid capture kind", record: mutateCapture(func(r *Record) { r.Details.Capture.Kind = "invalid" }), wantField: "capture kind"},
		{name: "nil capture tags", record: mutateCapture(func(r *Record) { r.Details.Capture.Tags = nil }), wantField: "tags"},
		{name: "empty capture tag", record: mutateCapture(func(r *Record) { r.Details.Capture.Tags = []string{"forge", ""} }), wantField: "tags"},
		{name: "unnormalized capture tags", record: mutateCapture(func(r *Record) { r.Details.Capture.Tags = []string{"Forge"} }), wantField: "tags"},
		{name: "duplicate capture tags", record: mutateCapture(func(r *Record) { r.Details.Capture.Tags = []string{"forge", "forge"} }), wantField: "tags"},
		{name: "friction details missing", record: mutateFriction(func(r *Record) { r.Details.Friction = nil }), wantField: "details"},
		{name: "friction has capture details", record: mutateFriction(func(r *Record) { r.Details.Capture = validCaptureRecord().Details.Capture }), wantField: "details"},
		{name: "invalid frequency", record: mutateFriction(func(r *Record) { r.Details.Friction.Frequency = "invalid" }), wantField: "frequency"},
		{name: "invalid impact", record: mutateFriction(func(r *Record) { r.Details.Friction.Impact = "invalid" }), wantField: "impact"},
		{name: "invalid category", record: mutateFriction(func(r *Record) { r.Details.Friction.Category = "invalid" }), wantField: "category"},
		{name: "empty workaround", record: mutateFriction(func(r *Record) { r.Details.Friction.CurrentWorkaround = stringPointer("") }), wantField: "current workaround"},
		{name: "unnormalized workaround", record: mutateFriction(func(r *Record) { r.Details.Friction.CurrentWorkaround = stringPointer(" checklist ") }), wantField: "current workaround"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.record.Validate()
			var invalid *InvalidValueError
			if !errors.As(err, &invalid) {
				t.Fatalf("Record.Validate() error = %T %v, want *InvalidValueError", err, err)
			}
			if invalid.Field != tt.wantField {
				t.Errorf("Record.Validate() field = %q, want %q", invalid.Field, tt.wantField)
			}
		})
	}
}

func validCaptureRecord() Record {
	project := "forge"
	created := NewTimestamp(time.Date(2026, time.August, 25, 9, 14, 3, 123456000, time.UTC))
	return Record{
		ID:          "cap_000102030405060708090a0b0c0d0e0f",
		Type:        RecordTypeCapture,
		Description: "Measure command startup time",
		Project:     &project,
		Status:      StatusCaptured,
		Details: RecordDetails{Capture: &CaptureDetails{
			Kind: CaptureKindObservation,
			Tags: []string{"performance", "cli"},
		}},
		CreatedAt: created,
		UpdatedAt: created,
	}
}

func validFrictionRecord() Record {
	workaround := "Follow a handwritten checklist"
	created := NewTimestamp(time.Date(2026, time.August, 25, 9, 18, 41, 654321000, time.UTC))
	updated := NewTimestamp(time.Date(2026, time.August, 26, 11, 2, 19, 0, time.UTC))
	return Record{
		ID:          "frc_f2308c1797cf4e77ac076e6af5ff1616",
		Type:        RecordTypeFriction,
		Description: "Releases require repeated manual checks",
		Status:      StatusReviewing,
		Details: RecordDetails{Friction: &FrictionDetails{
			Frequency:         FrequencyMonthly,
			Impact:            ImpactHigh,
			Category:          CategoryVerification,
			CurrentWorkaround: &workaround,
		}},
		CreatedAt: created,
		UpdatedAt: updated,
	}
}

func mutateCapture(mutate func(*Record)) Record {
	record := validCaptureRecord()
	mutate(&record)
	return record
}

func mutateFriction(mutate func(*Record)) Record {
	record := validFrictionRecord()
	mutate(&record)
	return record
}
