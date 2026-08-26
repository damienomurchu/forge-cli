package domain

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestParseCaptureType(t *testing.T) {
	testVocabulary(t, "capture type", ParseCaptureType, []CaptureType{
		CaptureTypeFriction,
		CaptureTypeAction,
		CaptureTypeFollowUp,
		CaptureTypeDecision,
	})
}

func TestCaptureTypes(t *testing.T) {
	want := []CaptureType{
		CaptureTypeFriction,
		CaptureTypeAction,
		CaptureTypeFollowUp,
		CaptureTypeDecision,
	}
	got := CaptureTypes()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CaptureTypes() = %v, want %v", got, want)
	}

	got[0] = CaptureTypeDecision
	if next := CaptureTypes(); !reflect.DeepEqual(next, want) {
		t.Errorf("CaptureTypes() after caller mutation = %v, want %v", next, want)
	}
}

func TestParseRecordType(t *testing.T) {
	testVocabulary(t, "record type", ParseRecordType, []RecordType{
		RecordTypeCapture,
		RecordTypeFriction,
	})
}

func TestParseStatus(t *testing.T) {
	testVocabulary(t, "status", ParseStatus, []Status{
		StatusCaptured,
		StatusReviewing,
		StatusCandidate,
		StatusAutomated,
		StatusDismissed,
	})
}

func TestParseCaptureKind(t *testing.T) {
	testVocabulary(t, "capture kind", ParseCaptureKind, []CaptureKind{
		CaptureKindThought,
		CaptureKindIdea,
		CaptureKindObservation,
		CaptureKindQuestion,
		CaptureKindDecision,
		CaptureKindSeed,
	})
}

func TestParseFrequency(t *testing.T) {
	testVocabulary(t, "frequency", ParseFrequency, []Frequency{
		FrequencyDaily,
		FrequencyWeekly,
		FrequencyMonthly,
		FrequencyOccasional,
		FrequencyUnknown,
	})
}

func TestParseImpact(t *testing.T) {
	testVocabulary(t, "impact", ParseImpact, []Impact{
		ImpactLow,
		ImpactMedium,
		ImpactHigh,
		ImpactUnknown,
	})
}

func TestParseCategory(t *testing.T) {
	testVocabulary(t, "category", ParseCategory, []Category{
		CategoryInformationFinding,
		CategoryRepeatedAction,
		CategoryContextSwitching,
		CategoryRemembering,
		CategoryVerification,
		CategoryWaiting,
		CategoryOther,
	})
}

type vocabularyValue interface {
	comparable
	String() string
	Valid() bool
}

func testVocabulary[T vocabularyValue](
	t *testing.T,
	field string,
	parse func(string) (T, error),
	values []T,
) {
	t.Helper()

	for _, want := range values {
		t.Run(want.String(), func(t *testing.T) {
			got, err := parse(want.String())
			if err != nil {
				t.Fatalf("parse(%q) error = %v", want.String(), err)
			}
			if got != want {
				t.Errorf("parse(%q) = %q, want %q", want.String(), got, want)
			}
			if !got.Valid() {
				t.Errorf("%q.Valid() = false, want true", got)
			}
		})
	}

	for _, input := range []string{"", "invalid", "CAPTURE", " captured "} {
		t.Run("invalid_"+input, func(t *testing.T) {
			got, err := parse(input)
			var invalid *InvalidValueError
			if !errors.As(err, &invalid) {
				t.Fatalf("parse(%q) error = %T %v, want *InvalidValueError", input, err, err)
			}
			if invalid.Field != field || invalid.Value != input {
				t.Errorf("error = %#v, want field %q and value %q", invalid, field, input)
			}
			if got, want := err.Error(), fmt.Sprintf("invalid %s %q", field, input); got != want {
				t.Errorf("error text = %q, want %q", got, want)
			}
			var zero T
			if got != zero {
				t.Errorf("parse(%q) = %q, want zero value", input, got)
			}
			if got.Valid() {
				t.Errorf("zero value.Valid() = true, want false")
			}
		})
	}
}
