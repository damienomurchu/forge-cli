package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/repository"
)

func TestExecuteListForwardsFiltersAndPreservesOrder(t *testing.T) {
	frictionType, project, limit := domain.CaptureTypeFriction, "forge", 2
	request := listRequest{filters: repository.CaptureFilters{
		Type: &frictionType, Project: &project, Limit: &limit,
	}}
	newer := persistedCaptureForList(t, domain.CaptureTypeFriction, 2, 15)
	older := persistedCaptureForList(t, domain.CaptureTypeAction, 1, 14)
	lister := &recordingCaptureLister{captures: []domain.Capture{newer, older}}
	var stdout bytes.Buffer
	if err := executeList(context.Background(), request, lister, &stdout); err != nil {
		t.Fatalf("executeList() error = %v", err)
	}
	if !reflect.DeepEqual(lister.filters, []repository.CaptureFilters{request.filters}) {
		t.Errorf("filters = %#v, want %#v", lister.filters, request.filters)
	}
	want := newer.ID.String() + "  friction  description\n" +
		older.ID.String() + "  action  description\n"
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestExecuteListRendersEveryTypeAsJSON(t *testing.T) {
	captures := make([]domain.Capture, 0, len(domain.CaptureTypes()))
	for index, captureType := range domain.CaptureTypes() {
		captures = append(captures, persistedCaptureForList(t, captureType, byte(index+1), 14-index))
	}
	lister := &recordingCaptureLister{captures: captures}
	var stdout bytes.Buffer
	if err := executeList(
		context.Background(), listRequest{json: true}, lister, &stdout,
	); err != nil {
		t.Fatalf("executeList() error = %v", err)
	}
	for _, captureType := range domain.CaptureTypes() {
		if !strings.Contains(stdout.String(), `"capture_type":"`+captureType.String()+`"`) {
			t.Errorf("JSON output missing %s: %q", captureType, stdout.String())
		}
	}
	if !strings.HasPrefix(stdout.String(), "[") || !strings.HasSuffix(stdout.String(), "]\n") {
		t.Errorf("stdout = %q, want one JSON array", stdout.String())
	}
}

func TestExecuteListEmptyResults(t *testing.T) {
	for _, tt := range []struct {
		name string
		json bool
		want string
	}{
		{name: "human", want: ""},
		{name: "JSON", json: true, want: "[]\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			err := executeList(
				context.Background(), listRequest{json: tt.json},
				&recordingCaptureLister{captures: []domain.Capture{}}, &stdout,
			)
			if err != nil || stdout.String() != tt.want {
				t.Fatalf("error/stdout = %v/%q, want nil/%q", err, stdout.String(), tt.want)
			}
		})
	}
}

func TestExecuteListRepositoryFailureWritesNothing(t *testing.T) {
	wantErr := errors.New("query failed")
	var stdout bytes.Buffer
	err := executeList(
		context.Background(), listRequest{},
		&recordingCaptureLister{err: wantErr}, &stdout,
	)
	if !errors.Is(err, wantErr) || stdout.Len() != 0 {
		t.Fatalf("error/stdout = %v/%q", err, stdout.String())
	}
}

func TestExecuteListPreservesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := executeList(ctx, listRequest{}, contextCaptureLister{}, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestExecuteListInvalidResultWritesNothing(t *testing.T) {
	var stdout bytes.Buffer
	err := executeList(
		context.Background(), listRequest{},
		&recordingCaptureLister{captures: []domain.Capture{{}}}, &stdout,
	)
	var invalid *domain.InvalidValueError
	if !errors.As(err, &invalid) || stdout.Len() != 0 {
		t.Fatalf("error/stdout = %T %v/%q", err, err, stdout.String())
	}
}

func TestExecuteListWriterFailureOccursAfterQuery(t *testing.T) {
	wantErr := errors.New("write failed")
	lister := &recordingCaptureLister{captures: []domain.Capture{}}
	err := executeList(
		context.Background(), listRequest{json: true}, lister,
		failingCaptureWriter{err: wantErr},
	)
	if !errors.Is(err, wantErr) || len(lister.filters) != 1 {
		t.Fatalf("error/query count = %v/%d", err, len(lister.filters))
	}
}

func TestExecuteListRejectsMissingBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		lister captureLister
		stdout io.Writer
		want   string
	}{
		{name: "repository", stdout: io.Discard, want: "repository is required"},
		{name: "writer", lister: &recordingCaptureLister{}, want: "writer is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executeList(context.Background(), listRequest{}, tt.lister, tt.stdout)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

type recordingCaptureLister struct {
	filters  []repository.CaptureFilters
	captures []domain.Capture
	err      error
}

func (l *recordingCaptureLister) List(
	_ context.Context, filters repository.CaptureFilters,
) ([]domain.Capture, error) {
	l.filters = append(l.filters, filters)
	return l.captures, l.err
}

type contextCaptureLister struct{}

func (contextCaptureLister) List(
	ctx context.Context, _ repository.CaptureFilters,
) ([]domain.Capture, error) {
	return nil, ctx.Err()
}

func persistedCaptureForList(
	t *testing.T, captureType domain.CaptureType, idByte byte, hour int,
) domain.Capture {
	t.Helper()
	capture, err := domain.NewPersistedCapture(
		proposalForPersistence(t, captureType),
		time.Date(2026, time.August, 26, hour, 0, 0, 0, time.UTC),
		bytes.NewReader(bytes.Repeat([]byte{idByte}, 16)),
	)
	if err != nil {
		t.Fatalf("NewPersistedCapture() error = %v", err)
	}
	return capture
}
