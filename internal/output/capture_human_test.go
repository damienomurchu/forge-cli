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

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteCaptureCreatedMatchesGolden(t *testing.T) {
	for _, captureType := range domain.CaptureTypes() {
		t.Run(captureType.String(), func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", ""+captureType.String()+"-created.golden"))
			if err != nil {
				t.Fatal(err)
			}
			var got bytes.Buffer
			if err := WriteCaptureCreated(&got, testCapture(t, captureType)); err != nil {
				t.Fatalf("WriteCaptureCreated() error = %v", err)
			}
			if got.String() != string(want) {
				t.Errorf("WriteCaptureCreated() = %q, want %q", got.String(), want)
			}
		})
	}
}

func TestWriteCaptureMatchesGolden(t *testing.T) {
	for _, captureType := range domain.CaptureTypes() {
		t.Run(captureType.String(), func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", ""+captureType.String()+"-show.golden"))
			if err != nil {
				t.Fatal(err)
			}
			var got bytes.Buffer
			if err := WriteCapture(&got, testCapture(t, captureType)); err != nil {
				t.Fatalf("WriteCapture() error = %v", err)
			}
			if got.String() != string(want) {
				t.Errorf("WriteCapture() = %q, want %q", got.String(), want)
			}
		})
	}
}

func TestWriteCaptureShowsAbsentFrictionText(t *testing.T) {
	capture := testCapture(t, domain.CaptureTypeFriction)
	capture.Details.Friction.Project = nil
	capture.Details.Friction.CurrentWorkaround = nil
	var got bytes.Buffer
	if err := WriteCapture(&got, capture); err != nil {
		t.Fatalf("WriteCapture() error = %v", err)
	}
	for _, field := range []string{"Project: (none)\n", "Current workaround: (none)\n"} {
		if !strings.Contains(got.String(), field) {
			t.Errorf("WriteCapture() = %q, want %q", got.String(), field)
		}
	}
}

func TestWriteCaptureEscapesUserText(t *testing.T) {
	capture := testCapture(t, domain.CaptureTypeFriction)
	capture.Description = "manual\nchecks\t\u202e"
	project := "forge\r\u2066"
	workaround := "line one\\line two"
	capture.Details.Friction.Project = &project
	capture.Details.Friction.CurrentWorkaround = &workaround
	var got bytes.Buffer
	if err := WriteCapture(&got, capture); err != nil {
		t.Fatalf("WriteCapture() error = %v", err)
	}
	for _, escaped := range []string{
		`Description: manual\nchecks\t\u202e`,
		`Project: forge\r\u2066`,
		`Current workaround: line one\\line two`,
	} {
		if !strings.Contains(got.String(), escaped) {
			t.Errorf("WriteCapture() = %q, want escaped %q", got.String(), escaped)
		}
	}
}

func TestWriteCaptureListMatchesGoldenAndPreservesOrder(t *testing.T) {
	captures := []domain.Capture{
		testCapture(t, domain.CaptureTypeDecision),
		testCapture(t, domain.CaptureTypeFriction),
		testCapture(t, domain.CaptureTypeAction),
		testCapture(t, domain.CaptureTypeFollowUp),
	}
	captures[1].ID = "frc_00000000000000000000000000000000"
	want, err := os.ReadFile(filepath.Join("testdata", "list.golden"))
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := WriteCaptureList(&got, captures); err != nil {
		t.Fatalf("WriteCaptureList() error = %v", err)
	}
	if got.String() != string(want) {
		t.Errorf("WriteCaptureList() = %q, want %q", got.String(), want)
	}
}

func TestWriteCaptureListEscapesDescriptions(t *testing.T) {
	capture := testCapture(t, domain.CaptureTypeAction)
	capture.Description = "line one\nline two\t\u202e"
	var got bytes.Buffer
	if err := WriteCaptureList(&got, []domain.Capture{capture}); err != nil {
		t.Fatalf("WriteCaptureList() error = %v", err)
	}
	want := capture.ID.String() + "  action  line one\\nline two\\t\\u202e\n"
	if got.String() != want {
		t.Errorf("WriteCaptureList() = %q, want %q", got.String(), want)
	}
}

func TestWriteCaptureListEmpty(t *testing.T) {
	for _, captures := range [][]domain.Capture{nil, {}} {
		var got bytes.Buffer
		if err := WriteCaptureList(&got, captures); err != nil {
			t.Fatalf("WriteCaptureList() error = %v", err)
		}
		if got.Len() != 0 {
			t.Errorf("WriteCaptureList() = %q, want empty", got.String())
		}
	}
}

func TestCaptureHumanOutputValidatesBeforeWriting(t *testing.T) {
	invalid := testCapture(t, domain.CaptureTypeAction)
	invalid.Description = " invalid "

	tests := []struct {
		name  string
		write func(*bytes.Buffer) error
	}{
		{name: "created", write: func(w *bytes.Buffer) error { return WriteCaptureCreated(w, invalid) }},
		{name: "show", write: func(w *bytes.Buffer) error { return WriteCapture(w, invalid) }},
		{name: "list", write: func(w *bytes.Buffer) error {
			return WriteCaptureList(w, []domain.Capture{testCapture(t, domain.CaptureTypeDecision), invalid})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bytes.Buffer
			err := tt.write(&got)
			var invalidValue *domain.InvalidValueError
			if !errors.As(err, &invalidValue) || got.Len() != 0 {
				t.Fatalf("write error/output = %v/%q, want validation and no output", err, got.String())
			}
		})
	}
}

func TestCaptureHumanOutputPropagatesWriterFailures(t *testing.T) {
	wantErr := errors.New("writer failed")
	capture := testCapture(t, domain.CaptureTypeAction)
	tests := []struct {
		name  string
		write func() error
	}{
		{name: "created", write: func() error { return WriteCaptureCreated(errorWriter{err: wantErr}, capture) }},
		{name: "show", write: func() error { return WriteCapture(errorWriter{err: wantErr}, capture) }},
		{name: "list", write: func() error {
			return WriteCaptureList(errorWriter{err: wantErr}, []domain.Capture{capture})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.write(); !errors.Is(err, wantErr) {
				t.Fatalf("write error = %v, want %v", err, wantErr)
			}
		})
	}
}
