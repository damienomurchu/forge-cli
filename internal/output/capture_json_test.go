package output

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestWriteCaptureJSONMatchesGolden(t *testing.T) {
	tests := []struct {
		name        string
		captureType domain.CaptureType
	}{
		{name: "friction", captureType: domain.CaptureTypeFriction},
		{name: "action", captureType: domain.CaptureTypeAction},
		{name: "follow-up", captureType: domain.CaptureTypeFollowUp},
		{name: "decision", captureType: domain.CaptureTypeDecision},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", ""+tt.name+".golden"))
			if err != nil {
				t.Fatal(err)
			}
			var got bytes.Buffer
			if err := WriteCaptureJSON(&got, testCapture(t, tt.captureType)); err != nil {
				t.Fatalf("WriteCaptureJSON() error = %v", err)
			}
			if got.String() != string(want) {
				t.Errorf("JSON mismatch\ngot:  %s\nwant: %s", got.Bytes(), want)
			}
		})
	}
}

func TestWriteCaptureJSONEmitsNullOptionalFrictionText(t *testing.T) {
	proposed, err := domain.NewProposedCapture(domain.ProposedCaptureInput{
		Type:        domain.CaptureTypeFriction,
		Description: "Recurring checks",
		Details: domain.ProposedCaptureDetailsInput{Friction: &domain.FrictionCaptureInput{
			Frequency: domain.FrequencyUnknown,
			Impact:    domain.ImpactUnknown,
			Category:  domain.CategoryOther,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	capture, err := domain.NewPersistedCapture(
		proposed,
		time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC),
		bytes.NewReader(make([]byte, 16)),
	)
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := WriteCaptureJSON(&got, capture); err != nil {
		t.Fatalf("WriteCaptureJSON() error = %v", err)
	}
	want := `"project":null`
	if !bytes.Contains(got.Bytes(), []byte(want)) ||
		!bytes.Contains(got.Bytes(), []byte(`"current_workaround":null`)) {
		t.Errorf("WriteCaptureJSON() = %q, want null optional friction fields", got.String())
	}
}

func TestWriteCaptureJSONAcceptsMigratedFrictionID(t *testing.T) {
	capture := testCapture(t, domain.CaptureTypeFriction)
	capture.ID = "frc_00000000000000000000000000000000"
	var got bytes.Buffer
	if err := WriteCaptureJSON(&got, capture); err != nil {
		t.Fatalf("WriteCaptureJSON() error = %v", err)
	}
	if !bytes.Contains(got.Bytes(), []byte(`"id":"frc_00000000000000000000000000000000"`)) {
		t.Errorf("WriteCaptureJSON() = %q, want migrated ID", got.String())
	}
}

func TestWriteCapturesJSONPreservesOrder(t *testing.T) {
	captures := []domain.Capture{
		testCapture(t, domain.CaptureTypeAction),
		testCapture(t, domain.CaptureTypeFriction),
	}
	var got bytes.Buffer
	if err := WriteCapturesJSON(&got, captures); err != nil {
		t.Fatalf("WriteCapturesJSON() error = %v", err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "captures.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Errorf("JSON mismatch\ngot:  %s\nwant: %s", got.Bytes(), want)
	}
}

func TestWriteCapturesJSONEmitsEmptyArray(t *testing.T) {
	for _, captures := range [][]domain.Capture{nil, {}} {
		var got bytes.Buffer
		if err := WriteCapturesJSON(&got, captures); err != nil {
			t.Fatalf("WriteCapturesJSON() error = %v", err)
		}
		if got.String() != "[]\n" {
			t.Errorf("WriteCapturesJSON() = %q, want %q", got.String(), "[]\n")
		}
	}
}

func TestCaptureJSONValidatesBeforeWriting(t *testing.T) {
	invalid := testCapture(t, domain.CaptureTypeDecision)
	invalid.Description = " invalid "

	var single bytes.Buffer
	err := WriteCaptureJSON(&single, invalid)
	var invalidValue *domain.InvalidValueError
	if !errors.As(err, &invalidValue) || single.Len() != 0 {
		t.Fatalf("WriteCaptureJSON() error/output = %v/%q, want validation and no output", err, single.String())
	}

	var list bytes.Buffer
	err = WriteCapturesJSON(&list, []domain.Capture{testCapture(t, domain.CaptureTypeAction), invalid})
	if !errors.As(err, &invalidValue) || list.Len() != 0 {
		t.Fatalf("WriteCapturesJSON() error/output = %v/%q, want validation and no output", err, list.String())
	}
}

func TestCaptureJSONPropagatesWriterFailures(t *testing.T) {
	wantErr := errors.New("writer failed")
	if err := WriteCaptureJSON(errorWriter{err: wantErr}, testCapture(t, domain.CaptureTypeAction)); !errors.Is(err, wantErr) {
		t.Fatalf("WriteCaptureJSON() error = %v, want %v", err, wantErr)
	}
	if err := WriteCapturesJSON(errorWriter{err: wantErr}, []domain.Capture{testCapture(t, domain.CaptureTypeAction)}); !errors.Is(err, wantErr) {
		t.Fatalf("WriteCapturesJSON() error = %v, want %v", err, wantErr)
	}
}

func testCapture(t *testing.T, captureType domain.CaptureType) domain.Capture {
	t.Helper()
	description := map[domain.CaptureType]string{
		domain.CaptureTypeFriction: "Releases require manual checks",
		domain.CaptureTypeAction:   "Ship the release",
		domain.CaptureTypeFollowUp: "Chase the vendor",
		domain.CaptureTypeDecision: "Choose the database",
	}[captureType]
	input := domain.ProposedCaptureInput{Type: captureType, Description: description}
	switch captureType {
	case domain.CaptureTypeFriction:
		input.Details.Friction = &domain.FrictionCaptureInput{
			Project: "forge", Frequency: domain.FrequencyWeekly,
			Impact: domain.ImpactHigh, Category: domain.CategoryVerification,
		}
	case domain.CaptureTypeAction:
		input.Details.Action = &domain.ActionCaptureDetails{}
	case domain.CaptureTypeFollowUp:
		input.Details.FollowUp = &domain.FollowUpCaptureDetails{}
	case domain.CaptureTypeDecision:
		input.Details.Decision = &domain.DecisionCaptureDetails{}
	default:
		t.Fatalf("unsupported capture type %q", captureType)
	}
	proposed, err := domain.NewProposedCapture(input)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := domain.NewPersistedCapture(
		proposed,
		time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC),
		bytes.NewReader(make([]byte, 16)),
	)
	if err != nil {
		t.Fatal(err)
	}
	return capture
}
