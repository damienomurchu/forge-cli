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

func TestExecuteUnifiedListForwardsFiltersAndPreservesOrder(t *testing.T) {
	frictionType, project, limit := domain.CaptureTypeFriction, "forge", 2
	request := unifiedListRequest{filters: repository.CaptureFilters{
		Type: &frictionType, Project: &project, Limit: &limit,
	}}
	newer := persistedCaptureForUnifiedList(t, domain.CaptureTypeFriction, 2, 15)
	older := persistedCaptureForUnifiedList(t, domain.CaptureTypeAction, 1, 14)
	lister := &recordingUnifiedCaptureLister{captures: []domain.Capture{newer, older}}
	var stdout bytes.Buffer
	if err := executeUnifiedList(context.Background(), request, lister, &stdout); err != nil {
		t.Fatalf("executeUnifiedList() error = %v", err)
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

func TestExecuteUnifiedListRendersEveryTypeAsJSON(t *testing.T) {
	captures := make([]domain.Capture, 0, len(domain.CaptureTypes()))
	for index, captureType := range domain.CaptureTypes() {
		captures = append(captures, persistedCaptureForUnifiedList(t, captureType, byte(index+1), 14-index))
	}
	lister := &recordingUnifiedCaptureLister{captures: captures}
	var stdout bytes.Buffer
	if err := executeUnifiedList(
		context.Background(), unifiedListRequest{json: true}, lister, &stdout,
	); err != nil {
		t.Fatalf("executeUnifiedList() error = %v", err)
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

func TestExecuteUnifiedListEmptyResults(t *testing.T) {
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
			err := executeUnifiedList(
				context.Background(), unifiedListRequest{json: tt.json},
				&recordingUnifiedCaptureLister{captures: []domain.Capture{}}, &stdout,
			)
			if err != nil || stdout.String() != tt.want {
				t.Fatalf("error/stdout = %v/%q, want nil/%q", err, stdout.String(), tt.want)
			}
		})
	}
}

func TestExecuteUnifiedListRepositoryFailureWritesNothing(t *testing.T) {
	wantErr := errors.New("query failed")
	var stdout bytes.Buffer
	err := executeUnifiedList(
		context.Background(), unifiedListRequest{},
		&recordingUnifiedCaptureLister{err: wantErr}, &stdout,
	)
	if !errors.Is(err, wantErr) || stdout.Len() != 0 {
		t.Fatalf("error/stdout = %v/%q", err, stdout.String())
	}
}

func TestExecuteUnifiedListPreservesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := executeUnifiedList(ctx, unifiedListRequest{}, contextUnifiedCaptureLister{}, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestExecuteUnifiedListInvalidResultWritesNothing(t *testing.T) {
	var stdout bytes.Buffer
	err := executeUnifiedList(
		context.Background(), unifiedListRequest{},
		&recordingUnifiedCaptureLister{captures: []domain.Capture{{}}}, &stdout,
	)
	var invalid *domain.InvalidValueError
	if !errors.As(err, &invalid) || stdout.Len() != 0 {
		t.Fatalf("error/stdout = %T %v/%q", err, err, stdout.String())
	}
}

func TestExecuteUnifiedListWriterFailureOccursAfterQuery(t *testing.T) {
	wantErr := errors.New("write failed")
	lister := &recordingUnifiedCaptureLister{captures: []domain.Capture{}}
	err := executeUnifiedList(
		context.Background(), unifiedListRequest{json: true}, lister,
		failingCaptureWriter{err: wantErr},
	)
	if !errors.Is(err, wantErr) || len(lister.filters) != 1 {
		t.Fatalf("error/query count = %v/%d", err, len(lister.filters))
	}
}

func TestExecuteUnifiedListRejectsMissingBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		lister unifiedCaptureLister
		stdout io.Writer
		want   string
	}{
		{name: "repository", stdout: io.Discard, want: "repository is required"},
		{name: "writer", lister: &recordingUnifiedCaptureLister{}, want: "writer is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executeUnifiedList(context.Background(), unifiedListRequest{}, tt.lister, tt.stdout)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

type recordingUnifiedCaptureLister struct {
	filters  []repository.CaptureFilters
	captures []domain.Capture
	err      error
}

func (l *recordingUnifiedCaptureLister) List(
	_ context.Context, filters repository.CaptureFilters,
) ([]domain.Capture, error) {
	l.filters = append(l.filters, filters)
	return l.captures, l.err
}

type contextUnifiedCaptureLister struct{}

func (contextUnifiedCaptureLister) List(
	ctx context.Context, _ repository.CaptureFilters,
) ([]domain.Capture, error) {
	return nil, ctx.Err()
}

func persistedCaptureForUnifiedList(
	t *testing.T, captureType domain.CaptureType, idByte byte, hour int,
) domain.Capture {
	t.Helper()
	capture, err := domain.NewPersistedCapture(
		unifiedProposalForPersistence(t, captureType),
		time.Date(2026, time.August, 26, hour, 0, 0, 0, time.UTC),
		bytes.NewReader(bytes.Repeat([]byte{idByte}, 16)),
	)
	if err != nil {
		t.Fatalf("NewPersistedCapture() error = %v", err)
	}
	return capture
}
