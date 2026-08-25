package domain

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
	"testing/iotest"
	"time"
)

func TestNewCaptureConstructsCanonicalRecord(t *testing.T) {
	now := time.Date(
		2026, time.August, 25, 10, 44, 3, 123456789,
		time.FixedZone("test", 90*60),
	)
	input := CaptureInput{
		Description: " \tMeasure\tstartup time\u2003",
		Project:     " forge ",
		Kind:        CaptureKindObservation,
		Tags:        " Performance, CLI,performance, ",
	}
	random := bytes.NewReader([]byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	})

	got, err := NewCapture(input, now, random)
	if err != nil {
		t.Fatalf("NewCapture() error = %v", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("NewCapture() produced invalid record: %v", err)
	}
	if got.ID != "cap_000102030405060708090a0b0c0d0e0f" {
		t.Errorf("ID = %q", got.ID)
	}
	if got.Type != RecordTypeCapture || got.Status != StatusCaptured {
		t.Errorf("type/status = %q/%q, want capture/captured", got.Type, got.Status)
	}
	if got.Description != "Measure\tstartup time" {
		t.Errorf("description = %q", got.Description)
	}
	if got.Project == nil || *got.Project != "forge" {
		t.Errorf("project = %v, want forge", got.Project)
	}
	if got.Details.Capture == nil || got.Details.Friction != nil {
		t.Fatalf("details = %#v, want capture only", got.Details)
	}
	if got.Details.Capture.Kind != CaptureKindObservation {
		t.Errorf("kind = %q, want observation", got.Details.Capture.Kind)
	}
	wantTags := []string{"performance", "cli"}
	if len(got.Details.Capture.Tags) != len(wantTags) {
		t.Fatalf("tags = %q, want %q", got.Details.Capture.Tags, wantTags)
	}
	for i := range wantTags {
		if got.Details.Capture.Tags[i] != wantTags[i] {
			t.Errorf("tags = %q, want %q", got.Details.Capture.Tags, wantTags)
		}
	}
	wantTimestamp := "2026-08-25T09:14:03.123456Z"
	if got.CreatedAt.String() != wantTimestamp || got.UpdatedAt.String() != wantTimestamp {
		t.Errorf("timestamps = %q/%q, want %q", got.CreatedAt, got.UpdatedAt, wantTimestamp)
	}
}

func TestNewCaptureKeepsOptionalValuesAbsent(t *testing.T) {
	got, err := NewCapture(CaptureInput{
		Description: "A thought",
		Kind:        CaptureKindThought,
	}, time.Time{}, bytes.NewReader(make([]byte, randomIDBytes)))
	if err != nil {
		t.Fatalf("NewCapture() error = %v", err)
	}
	if got.Project != nil {
		t.Errorf("project = %v, want nil", got.Project)
	}
	if got.Details.Capture.Tags == nil || len(got.Details.Capture.Tags) != 0 {
		t.Errorf("tags = %#v, want non-nil empty slice", got.Details.Capture.Tags)
	}
}

func TestNewCaptureValidatesBeforeReadingRandomness(t *testing.T) {
	tests := []struct {
		name      string
		input     CaptureInput
		wantField string
	}{
		{name: "blank description", input: CaptureInput{Description: " \t", Kind: CaptureKindThought}, wantField: "description"},
		{name: "invalid kind", input: CaptureInput{Description: "A thought", Kind: "invalid"}, wantField: "capture kind"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCapture(tt.input, time.Now(), panicReader{})
			var invalid *InvalidValueError
			if !errors.As(err, &invalid) {
				t.Fatalf("NewCapture() error = %T %v, want *InvalidValueError", err, err)
			}
			if invalid.Field != tt.wantField {
				t.Errorf("error field = %q, want %q", invalid.Field, tt.wantField)
			}
			if !reflect.DeepEqual(got, Record{}) {
				t.Errorf("NewCapture() = %#v, want zero record", got)
			}
		})
	}
}

func TestNewCapturePropagatesRandomnessFailure(t *testing.T) {
	wantErr := errors.New("random source failed")
	got, err := NewCapture(CaptureInput{
		Description: "A thought",
		Kind:        CaptureKindThought,
	}, time.Now(), iotest.ErrReader(wantErr))
	if !errors.Is(err, wantErr) {
		t.Fatalf("NewCapture() error = %v, want wrapped %v", err, wantErr)
	}
	if !reflect.DeepEqual(got, Record{}) {
		t.Errorf("NewCapture() = %#v, want zero record", got)
	}
}
