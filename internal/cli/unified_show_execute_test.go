package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/repository"
)

func TestExecuteUnifiedShowRendersEveryTypeForHumans(t *testing.T) {
	for index, captureType := range domain.CaptureTypes() {
		t.Run(captureType.String(), func(t *testing.T) {
			capture := persistedCaptureForUnifiedList(t, captureType, byte(index+1), 14)
			finder := &recordingUnifiedCaptureFinder{capture: capture}
			var stdout bytes.Buffer
			if err := executeUnifiedShow(
				context.Background(), capture.ID, false, finder, &stdout,
			); err != nil {
				t.Fatalf("executeUnifiedShow() error = %v", err)
			}
			if len(finder.ids) != 1 || finder.ids[0] != capture.ID {
				t.Errorf("lookup IDs = %#v, want %s", finder.ids, capture.ID)
			}
			for _, want := range []string{
				"ID: " + capture.ID.String() + "\n",
				"Type: " + captureType.String() + "\n",
				"Description: description\n",
			} {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("stdout = %q, missing %q", stdout.String(), want)
				}
			}
		})
	}
}

func TestExecuteUnifiedShowWritesJSON(t *testing.T) {
	capture := persistedCaptureForUnifiedList(t, domain.CaptureTypeDecision, 7, 14)
	var stdout bytes.Buffer
	err := executeUnifiedShow(
		context.Background(), capture.ID, true,
		&recordingUnifiedCaptureFinder{capture: capture}, &stdout,
	)
	if err != nil {
		t.Fatalf("executeUnifiedShow() error = %v", err)
	}
	if !strings.HasPrefix(stdout.String(), `{"id":"`+capture.ID.String()+`"`) ||
		!strings.Contains(stdout.String(), `"capture_type":"decision"`) ||
		!strings.HasSuffix(stdout.String(), "}\n") {
		t.Errorf("stdout = %q, want one decision JSON object", stdout.String())
	}
}

func TestExecuteUnifiedShowPassesOpaqueMigratedFrictionID(t *testing.T) {
	capture := persistedCaptureForUnifiedList(t, domain.CaptureTypeFriction, 8, 14)
	capture.ID = "frc_08080808080808080808080808080808"
	if err := capture.Validate(); err != nil {
		t.Fatalf("migrated friction capture validation error = %v", err)
	}
	finder := &recordingUnifiedCaptureFinder{capture: capture}
	if err := executeUnifiedShow(context.Background(), capture.ID, false, finder, io.Discard); err != nil {
		t.Fatalf("executeUnifiedShow() error = %v", err)
	}
	if len(finder.ids) != 1 || finder.ids[0] != capture.ID {
		t.Errorf("lookup IDs = %#v, want opaque migrated ID", finder.ids)
	}
}

func TestExecuteUnifiedShowPreservesRepositoryErrors(t *testing.T) {
	for _, wantErr := range []error{
		repository.ErrRecordNotFound,
		errors.New("stored data malformed"),
		context.Canceled,
	} {
		t.Run(wantErr.Error(), func(t *testing.T) {
			var stdout bytes.Buffer
			err := executeUnifiedShow(
				context.Background(), "capture-id", false,
				&recordingUnifiedCaptureFinder{err: wantErr}, &stdout,
			)
			if !errors.Is(err, wantErr) || stdout.Len() != 0 {
				t.Fatalf("error/stdout = %v/%q, want wrapped %v and no output", err, stdout.String(), wantErr)
			}
		})
	}
}

func TestExecuteUnifiedShowInvalidResultWritesNothing(t *testing.T) {
	var stdout bytes.Buffer
	err := executeUnifiedShow(
		context.Background(), "capture-id", false,
		&recordingUnifiedCaptureFinder{capture: domain.Capture{}}, &stdout,
	)
	var invalid *domain.InvalidValueError
	if !errors.As(err, &invalid) || stdout.Len() != 0 {
		t.Fatalf("error/stdout = %T %v/%q", err, err, stdout.String())
	}
}

func TestExecuteUnifiedShowWriterFailureOccursAfterLookup(t *testing.T) {
	wantErr := errors.New("write failed")
	capture := persistedCaptureForUnifiedList(t, domain.CaptureTypeAction, 9, 14)
	finder := &recordingUnifiedCaptureFinder{capture: capture}
	err := executeUnifiedShow(
		context.Background(), capture.ID, false, finder,
		failingCaptureWriter{err: wantErr},
	)
	if !errors.Is(err, wantErr) || len(finder.ids) != 1 {
		t.Fatalf("error/lookup count = %v/%d", err, len(finder.ids))
	}
}

func TestExecuteUnifiedShowRejectsMissingBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		finder unifiedCaptureFinder
		stdout io.Writer
		want   string
	}{
		{name: "repository", stdout: io.Discard, want: "repository is required"},
		{name: "writer", finder: &recordingUnifiedCaptureFinder{}, want: "writer is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executeUnifiedShow(context.Background(), "capture-id", false, tt.finder, tt.stdout)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

type recordingUnifiedCaptureFinder struct {
	ids     []domain.ID
	capture domain.Capture
	err     error
}

func (f *recordingUnifiedCaptureFinder) FindUnifiedCaptureByID(
	_ context.Context, id domain.ID,
) (domain.Capture, error) {
	f.ids = append(f.ids, id)
	return f.capture, f.err
}
