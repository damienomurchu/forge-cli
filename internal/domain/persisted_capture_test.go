package domain

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
	"testing/iotest"
	"time"
)

func TestNewPersistedCaptureConstructsEachType(t *testing.T) {
	now := time.Date(2026, time.August, 25, 10, 48, 41, 654321999, time.FixedZone("test", 90*60))
	types := CaptureTypes()
	for _, captureType := range types {
		t.Run(captureType.String(), func(t *testing.T) {
			proposed := proposedCaptureForType(t, captureType)
			got, err := NewPersistedCapture(proposed, now, bytes.NewReader(sequentialIDBytes()))
			if err != nil {
				t.Fatalf("NewPersistedCapture() error = %v", err)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("persisted capture is invalid: %v", err)
			}
			if got.ID != "cap_000102030405060708090a0b0c0d0e0f" {
				t.Errorf("ID = %q", got.ID)
			}
			if got.Type != captureType || got.Description != proposed.Description {
				t.Errorf("type/description = %q/%q", got.Type, got.Description)
			}
			wantTimestamp := "2026-08-25T09:18:41.654321Z"
			if got.CreatedAt.String() != wantTimestamp || got.UpdatedAt.String() != wantTimestamp {
				t.Errorf("timestamps = %q/%q, want %q", got.CreatedAt, got.UpdatedAt, wantTimestamp)
			}
		})
	}
}

func TestNewPersistedCaptureCopiesProposedDetails(t *testing.T) {
	proposed := proposedCaptureForType(t, CaptureTypeFriction)
	got, err := NewPersistedCapture(proposed, time.Time{}, bytes.NewReader(sequentialIDBytes()))
	if err != nil {
		t.Fatalf("NewPersistedCapture() error = %v", err)
	}
	*proposed.Details.Friction.Project = "changed"
	*proposed.Details.Friction.CurrentWorkaround = "changed"
	if *got.Details.Friction.Project != "forge" || *got.Details.Friction.CurrentWorkaround != "Use a checklist" {
		t.Errorf("persisted details changed with proposal: %#v", got.Details.Friction)
	}
}

func TestNewPersistedCaptureValidatesBeforeReadingRandomness(t *testing.T) {
	proposed := proposedCaptureForType(t, CaptureTypeAction)
	proposed.Description = " \t "
	got, err := NewPersistedCapture(proposed, time.Now(), panicReader{})
	var invalid *InvalidValueError
	if !errors.As(err, &invalid) || invalid.Field != "description" {
		t.Fatalf("NewPersistedCapture() error = %T %v, want invalid description", err, err)
	}
	if !reflect.DeepEqual(got, Capture{}) {
		t.Errorf("NewPersistedCapture() = %#v, want zero value", got)
	}
}

func TestNewPersistedCapturePropagatesRandomnessFailure(t *testing.T) {
	wantErr := errors.New("random source failed")
	got, err := NewPersistedCapture(
		proposedCaptureForType(t, CaptureTypeDecision),
		time.Now(),
		iotest.ErrReader(wantErr),
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("NewPersistedCapture() error = %v, want %v", err, wantErr)
	}
	if !reflect.DeepEqual(got, Capture{}) {
		t.Errorf("NewPersistedCapture() = %#v, want zero value", got)
	}
}

func TestCaptureValidateAcceptsMigratedFrictionID(t *testing.T) {
	capture := persistedCaptureForType(t, CaptureTypeFriction)
	capture.ID = "frc_000102030405060708090a0b0c0d0e0f"
	if err := capture.Validate(); err != nil {
		t.Fatalf("Validate() migrated friction error = %v", err)
	}
}

func TestCaptureValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Capture)
		wantField string
	}{
		{name: "capture type", mutate: func(capture *Capture) { capture.Type = "invalid" }, wantField: "capture type"},
		{name: "capture ID", mutate: func(capture *Capture) { capture.ID = "invalid" }, wantField: "capture ID"},
		{name: "legacy ID on action", mutate: func(capture *Capture) {
			capture.Type = CaptureTypeAction
			capture.Details = ProposedCaptureDetails{Action: &ActionCaptureDetails{}}
			capture.ID = "frc_000102030405060708090a0b0c0d0e0f"
		}, wantField: "capture ID"},
		{name: "description", mutate: func(capture *Capture) { capture.Description = " padded " }, wantField: "description"},
		{name: "details", mutate: func(capture *Capture) { capture.Details.Action = &ActionCaptureDetails{} }, wantField: "details"},
		{name: "updated before created", mutate: func(capture *Capture) {
			capture.UpdatedAt = NewTimestamp(capture.CreatedAt.Time().Add(-time.Microsecond))
		}, wantField: "updated_at"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture := persistedCaptureForType(t, CaptureTypeFriction)
			tt.mutate(&capture)
			var invalid *InvalidValueError
			if err := capture.Validate(); !errors.As(err, &invalid) || invalid.Field != tt.wantField {
				t.Fatalf("Validate() error = %T %v, want invalid %s", err, err, tt.wantField)
			}
		})
	}
}

func persistedCaptureForType(t *testing.T, captureType CaptureType) Capture {
	t.Helper()
	capture, err := NewPersistedCapture(
		proposedCaptureForType(t, captureType),
		time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC),
		bytes.NewReader(sequentialIDBytes()),
	)
	if err != nil {
		t.Fatalf("NewPersistedCapture() error = %v", err)
	}
	return capture
}

func proposedCaptureForType(t *testing.T, captureType CaptureType) ProposedCapture {
	t.Helper()
	input := ProposedCaptureInput{Type: captureType, Description: "A captured item"}
	switch captureType {
	case CaptureTypeFriction:
		input.Details.Friction = &FrictionCaptureInput{
			Project: "forge", Frequency: FrequencyWeekly, Impact: ImpactHigh,
			Category: CategoryVerification, CurrentWorkaround: "Use a checklist",
		}
	case CaptureTypeAction:
		input.Details.Action = &ActionCaptureDetails{}
	case CaptureTypeFollowUp:
		input.Details.FollowUp = &FollowUpCaptureDetails{}
	case CaptureTypeDecision:
		input.Details.Decision = &DecisionCaptureDetails{}
	default:
		t.Fatalf("unsupported capture type %q", captureType)
	}
	proposed, err := NewProposedCapture(input)
	if err != nil {
		t.Fatalf("NewProposedCapture() error = %v", err)
	}
	return proposed
}

func sequentialIDBytes() []byte {
	return []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	}
}

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("random source must not be read")
}
