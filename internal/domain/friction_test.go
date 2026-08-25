package domain

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
	"testing/iotest"
	"time"
)

func TestNewFrictionConstructsCanonicalRecord(t *testing.T) {
	now := time.Date(
		2026, time.August, 25, 10, 48, 41, 654321999,
		time.FixedZone("test", 90*60),
	)
	input := FrictionInput{
		Description:       " \tReleases require\nmanual checks\u2003",
		Project:           " forge ",
		Frequency:         FrequencyMonthly,
		Impact:            ImpactHigh,
		Category:          CategoryVerification,
		CurrentWorkaround: " Follow a handwritten checklist ",
	}
	random := bytes.NewReader([]byte{
		0xf0, 0xe1, 0xd2, 0xc3, 0xb4, 0xa5, 0x96, 0x87,
		0x78, 0x69, 0x5a, 0x4b, 0x3c, 0x2d, 0x1e, 0x0f,
	})

	got, err := NewFriction(input, now, random)
	if err != nil {
		t.Fatalf("NewFriction() error = %v", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("NewFriction() produced invalid record: %v", err)
	}
	if got.ID != "frc_f0e1d2c3b4a5968778695a4b3c2d1e0f" {
		t.Errorf("ID = %q", got.ID)
	}
	if got.Type != RecordTypeFriction || got.Status != StatusCaptured {
		t.Errorf("type/status = %q/%q, want friction/captured", got.Type, got.Status)
	}
	if got.Description != "Releases require\nmanual checks" {
		t.Errorf("description = %q", got.Description)
	}
	if got.Project == nil || *got.Project != "forge" {
		t.Errorf("project = %v, want forge", got.Project)
	}
	if got.Details.Friction == nil || got.Details.Capture != nil {
		t.Fatalf("details = %#v, want friction only", got.Details)
	}
	details := got.Details.Friction
	if details.Frequency != FrequencyMonthly || details.Impact != ImpactHigh || details.Category != CategoryVerification {
		t.Errorf("classifications = %q/%q/%q", details.Frequency, details.Impact, details.Category)
	}
	if details.CurrentWorkaround == nil || *details.CurrentWorkaround != "Follow a handwritten checklist" {
		t.Errorf("current workaround = %v", details.CurrentWorkaround)
	}
	wantTimestamp := "2026-08-25T09:18:41.654321Z"
	if got.CreatedAt.String() != wantTimestamp || got.UpdatedAt.String() != wantTimestamp {
		t.Errorf("timestamps = %q/%q, want %q", got.CreatedAt, got.UpdatedAt, wantTimestamp)
	}
}

func TestNewFrictionKeepsOptionalValuesAbsent(t *testing.T) {
	got, err := NewFriction(FrictionInput{
		Description: "A recurring problem",
		Frequency:   FrequencyUnknown,
		Impact:      ImpactUnknown,
		Category:    CategoryOther,
	}, time.Time{}, bytes.NewReader(make([]byte, randomIDBytes)))
	if err != nil {
		t.Fatalf("NewFriction() error = %v", err)
	}
	if got.Project != nil {
		t.Errorf("project = %v, want nil", got.Project)
	}
	if got.Details.Friction.CurrentWorkaround != nil {
		t.Errorf("current workaround = %v, want nil", got.Details.Friction.CurrentWorkaround)
	}
}

func TestNewFrictionValidatesBeforeReadingRandomness(t *testing.T) {
	valid := FrictionInput{
		Description: "A recurring problem",
		Frequency:   FrequencyWeekly,
		Impact:      ImpactMedium,
		Category:    CategoryRepeatedAction,
	}
	tests := []struct {
		name      string
		mutate    func(*FrictionInput)
		wantField string
	}{
		{name: "blank description", mutate: func(input *FrictionInput) { input.Description = " \t" }, wantField: "description"},
		{name: "invalid frequency", mutate: func(input *FrictionInput) { input.Frequency = "invalid" }, wantField: "frequency"},
		{name: "invalid impact", mutate: func(input *FrictionInput) { input.Impact = "invalid" }, wantField: "impact"},
		{name: "invalid category", mutate: func(input *FrictionInput) { input.Category = "invalid" }, wantField: "category"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valid
			tt.mutate(&input)
			got, err := NewFriction(input, time.Now(), panicReader{})
			var invalid *InvalidValueError
			if !errors.As(err, &invalid) {
				t.Fatalf("NewFriction() error = %T %v, want *InvalidValueError", err, err)
			}
			if invalid.Field != tt.wantField {
				t.Errorf("error field = %q, want %q", invalid.Field, tt.wantField)
			}
			if !reflect.DeepEqual(got, Record{}) {
				t.Errorf("NewFriction() = %#v, want zero record", got)
			}
		})
	}
}

func TestNewFrictionPropagatesRandomnessFailure(t *testing.T) {
	wantErr := errors.New("random source failed")
	got, err := NewFriction(FrictionInput{
		Description: "A recurring problem",
		Frequency:   FrequencyWeekly,
		Impact:      ImpactMedium,
		Category:    CategoryRepeatedAction,
	}, time.Now(), iotest.ErrReader(wantErr))
	if !errors.Is(err, wantErr) {
		t.Fatalf("NewFriction() error = %v, want wrapped %v", err, wantErr)
	}
	if !reflect.DeepEqual(got, Record{}) {
		t.Errorf("NewFriction() = %#v, want zero record", got)
	}
}
