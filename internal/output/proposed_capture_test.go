package output

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestWriteProposedCaptureSummaryMatchesGolden(t *testing.T) {
	tests := []struct {
		name    string
		capture domain.ProposedCapture
	}{
		{name: "friction", capture: proposedFriction(t, "forge", "Follow the checklist")},
		{name: "action", capture: proposedMinimalCapture(t, domain.CaptureTypeAction, "Ship the release")},
		{name: "follow-up", capture: proposedMinimalCapture(t, domain.CaptureTypeFollowUp, "Chase the vendor")},
		{name: "decision", capture: proposedMinimalCapture(t, domain.CaptureTypeDecision, "Choose the database")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", "proposed-"+tt.name+".golden"))
			if err != nil {
				t.Fatal(err)
			}
			var got bytes.Buffer
			if err := WriteProposedCaptureSummary(&got, tt.capture); err != nil {
				t.Fatalf("WriteProposedCaptureSummary() error = %v", err)
			}
			if got.String() != string(want) {
				t.Errorf("WriteProposedCaptureSummary() = %q, want %q", got.String(), want)
			}
		})
	}
}

func TestWriteProposedCaptureSummaryShowsAbsentFrictionText(t *testing.T) {
	capture := proposedFriction(t, "", "")
	var got bytes.Buffer
	if err := WriteProposedCaptureSummary(&got, capture); err != nil {
		t.Fatalf("WriteProposedCaptureSummary() error = %v", err)
	}
	for _, field := range []string{"Project: (none)\n", "Current workaround: (none)\n"} {
		if !strings.Contains(got.String(), field) {
			t.Errorf("summary = %q, want %q", got.String(), field)
		}
	}
}

func TestWriteProposedCaptureSummaryEscapesUserText(t *testing.T) {
	capture := proposedFriction(t, "forge\t\u202e", "line one\nline two\\done")
	capture.Description = "manual\rchecks\u2066"
	var got bytes.Buffer
	if err := WriteProposedCaptureSummary(&got, capture); err != nil {
		t.Fatalf("WriteProposedCaptureSummary() error = %v", err)
	}
	want := []string{
		`Description: manual\rchecks\u2066`,
		`Project: forge\t\u202e`,
		`Current workaround: line one\nline two\\done`,
	}
	for _, escaped := range want {
		if !strings.Contains(got.String(), escaped) {
			t.Errorf("summary = %q, want escaped %q", got.String(), escaped)
		}
	}
	if strings.ContainsRune(got.String(), '\u202e') || strings.ContainsRune(got.String(), '\u2066') ||
		strings.ContainsRune(got.String(), '\t') || strings.ContainsRune(got.String(), '\r') {
		t.Errorf("summary contains unsafe terminal text: %q", got.String())
	}
}

func TestWriteProposedCaptureSummaryValidatesBeforeWriting(t *testing.T) {
	capture := proposedMinimalCapture(t, domain.CaptureTypeAction, "Take action")
	capture.Details.Decision = &domain.DecisionCaptureDetails{}
	var got bytes.Buffer
	err := WriteProposedCaptureSummary(&got, capture)
	var invalid *domain.InvalidValueError
	if !errors.As(err, &invalid) {
		t.Fatalf("WriteProposedCaptureSummary() error = %T %v, want *domain.InvalidValueError", err, err)
	}
	if got.Len() != 0 {
		t.Errorf("WriteProposedCaptureSummary() wrote %q before validation failed", got.String())
	}
}

func TestWriteProposedCaptureSummaryPropagatesWriterFailure(t *testing.T) {
	wantErr := errors.New("writer failed")
	err := WriteProposedCaptureSummary(
		errorWriter{err: wantErr},
		proposedMinimalCapture(t, domain.CaptureTypeDecision, "Choose"),
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("WriteProposedCaptureSummary() error = %v, want %v", err, wantErr)
	}
}

func proposedFriction(t *testing.T, project, workaround string) domain.ProposedCapture {
	t.Helper()
	capture, err := domain.NewProposedCapture(domain.ProposedCaptureInput{
		Type:        domain.CaptureTypeFriction,
		Description: "Releases require manual checks",
		Details: domain.ProposedCaptureDetailsInput{Friction: &domain.FrictionCaptureInput{
			Project:           project,
			Frequency:         domain.FrequencyWeekly,
			Impact:            domain.ImpactHigh,
			Category:          domain.CategoryVerification,
			CurrentWorkaround: workaround,
		}},
	})
	if err != nil {
		t.Fatalf("NewProposedCapture() error = %v", err)
	}
	return capture
}

func proposedMinimalCapture(t *testing.T, captureType domain.CaptureType, description string) domain.ProposedCapture {
	t.Helper()
	details := domain.ProposedCaptureDetailsInput{}
	switch captureType {
	case domain.CaptureTypeAction:
		details.Action = &domain.ActionCaptureDetails{}
	case domain.CaptureTypeFollowUp:
		details.FollowUp = &domain.FollowUpCaptureDetails{}
	case domain.CaptureTypeDecision:
		details.Decision = &domain.DecisionCaptureDetails{}
	default:
		t.Fatalf("unsupported minimal capture type %q", captureType)
	}
	capture, err := domain.NewProposedCapture(domain.ProposedCaptureInput{
		Type:        captureType,
		Description: description,
		Details:     details,
	})
	if err != nil {
		t.Fatalf("NewProposedCapture() error = %v", err)
	}
	return capture
}
