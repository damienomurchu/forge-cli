package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestPersistUnifiedCaptureStoresAndWritesEveryType(t *testing.T) {
	now := time.Date(2026, time.August, 26, 14, 15, 16, 123456000, time.UTC)
	for index, captureType := range domain.CaptureTypes() {
		captureType := captureType
		t.Run(captureType.String(), func(t *testing.T) {
			proposal := unifiedProposalForPersistence(t, captureType)
			creator := &recordingUnifiedCaptureCreator{}
			var stdout bytes.Buffer
			capture, err := persistUnifiedCapture(
				context.Background(), proposal, false, now,
				bytes.NewReader(bytes.Repeat([]byte{byte(index + 1)}, 16)), creator, &stdout,
			)
			if err != nil {
				t.Fatalf("persistUnifiedCapture() error = %v", err)
			}
			if len(creator.captures) != 1 || creator.captures[0] != capture {
				t.Fatalf("stored captures = %#v, returned %#v", creator.captures, capture)
			}
			if capture.Type != captureType || capture.Description != proposal.Description {
				t.Errorf("capture = %#v", capture)
			}
			if !capture.CreatedAt.Time().Equal(now) || capture.UpdatedAt != capture.CreatedAt {
				t.Errorf("timestamps = %s/%s, want %s", capture.CreatedAt, capture.UpdatedAt, now)
			}
			want := "Created " + captureType.String() + " capture " + capture.ID.String() + "\n"
			if stdout.String() != want {
				t.Errorf("stdout = %q, want %q", stdout.String(), want)
			}
		})
	}
}

func TestPersistUnifiedCaptureWritesJSON(t *testing.T) {
	creator := &recordingUnifiedCaptureCreator{}
	var stdout bytes.Buffer
	capture, err := persistUnifiedCapture(
		context.Background(), unifiedProposalForPersistence(t, domain.CaptureTypeAction), true,
		time.Date(2026, time.August, 26, 14, 0, 0, 0, time.UTC), bytes.NewReader(make([]byte, 16)),
		creator, &stdout,
	)
	if err != nil {
		t.Fatalf("persistUnifiedCapture() error = %v", err)
	}
	want := `{"id":"` + capture.ID.String() + `","capture_type":"action","description":"description","details":{},"created_at":"2026-08-26T14:00:00.000000Z","updated_at":"2026-08-26T14:00:00.000000Z"}` + "\n"
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestPersistUnifiedCaptureFailsBeforePersistenceForInvalidProposalOrMetadata(t *testing.T) {
	wantRandomErr := errors.New("random failed")
	tests := []struct {
		name     string
		proposal domain.ProposedCapture
		random   io.Reader
		wantErr  error
	}{
		{name: "invalid proposal", proposal: domain.ProposedCapture{}, random: panicCaptureReader{}},
		{name: "randomness", proposal: unifiedProposalForPersistence(t, domain.CaptureTypeDecision), random: failingCaptureReader{err: wantRandomErr}, wantErr: wantRandomErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creator := &recordingUnifiedCaptureCreator{}
			var stdout bytes.Buffer
			capture, err := persistUnifiedCapture(
				context.Background(), tt.proposal, false, time.Now(), tt.random, creator, &stdout,
			)
			if err == nil || (tt.wantErr != nil && !errors.Is(err, tt.wantErr)) {
				t.Fatalf("error = %v, want failure wrapping %v", err, tt.wantErr)
			}
			if capture != (domain.Capture{}) || len(creator.captures) != 0 || stdout.Len() != 0 {
				t.Errorf("capture/stored/stdout = %#v/%#v/%q", capture, creator.captures, stdout.String())
			}
		})
	}
}

func TestPersistUnifiedCaptureRepositoryFailureWritesNothing(t *testing.T) {
	wantErr := errors.New("insert failed")
	creator := &recordingUnifiedCaptureCreator{err: wantErr}
	var stdout bytes.Buffer
	capture, err := persistUnifiedCapture(
		context.Background(), unifiedProposalForPersistence(t, domain.CaptureTypeFollowUp), false,
		time.Now(), bytes.NewReader(make([]byte, 16)), creator, &stdout,
	)
	if !errors.Is(err, wantErr) || capture != (domain.Capture{}) || stdout.Len() != 0 {
		t.Fatalf("capture/error/stdout = %#v/%v/%q", capture, err, stdout.String())
	}
}

func TestPersistUnifiedCaptureWriterFailureOccursAfterPersistence(t *testing.T) {
	wantErr := errors.New("write failed")
	creator := &recordingUnifiedCaptureCreator{}
	capture, err := persistUnifiedCapture(
		context.Background(), unifiedProposalForPersistence(t, domain.CaptureTypeAction), false,
		time.Now(), bytes.NewReader(make([]byte, 16)), creator, failingCaptureWriter{err: wantErr},
	)
	if !errors.Is(err, wantErr) || capture != (domain.Capture{}) {
		t.Fatalf("capture/error = %#v/%v", capture, err)
	}
	if len(creator.captures) != 1 {
		t.Fatalf("stored captures = %#v, want one", creator.captures)
	}
}

func TestPersistUnifiedCaptureRejectsMissingBoundaries(t *testing.T) {
	proposal := unifiedProposalForPersistence(t, domain.CaptureTypeAction)
	tests := []struct {
		name    string
		creator unifiedCaptureCreator
		stdout  io.Writer
		want    string
	}{
		{name: "repository", stdout: io.Discard, want: "repository is required"},
		{name: "writer", creator: &recordingUnifiedCaptureCreator{}, want: "writer is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture, err := persistUnifiedCapture(
				context.Background(), proposal, false, time.Now(), bytes.NewReader(make([]byte, 16)), tt.creator, tt.stdout,
			)
			if err == nil || !strings.Contains(err.Error(), tt.want) || capture != (domain.Capture{}) {
				t.Fatalf("capture/error = %#v/%v, want %q", capture, err, tt.want)
			}
		})
	}
}

type recordingUnifiedCaptureCreator struct {
	captures []domain.Capture
	err      error
}

func (c *recordingUnifiedCaptureCreator) CreateUnifiedCapture(_ context.Context, capture domain.Capture) error {
	c.captures = append(c.captures, capture)
	return c.err
}

func unifiedProposalForPersistence(t *testing.T, captureType domain.CaptureType) domain.ProposedCapture {
	t.Helper()
	input := domain.ProposedCaptureInput{Type: captureType, Description: "description"}
	switch captureType {
	case domain.CaptureTypeFriction:
		input.Details.Friction = &domain.FrictionCaptureInput{
			Frequency: domain.FrequencyUnknown,
			Impact:    domain.ImpactUnknown,
			Category:  domain.CategoryOther,
		}
	case domain.CaptureTypeAction:
		input.Details.Action = &domain.ActionCaptureDetails{}
	case domain.CaptureTypeFollowUp:
		input.Details.FollowUp = &domain.FollowUpCaptureDetails{}
	case domain.CaptureTypeDecision:
		input.Details.Decision = &domain.DecisionCaptureDetails{}
	}
	proposal, err := domain.NewProposedCapture(input)
	if err != nil {
		t.Fatalf("NewProposedCapture() error = %v", err)
	}
	return proposal
}

type failingCaptureReader struct{ err error }

func (r failingCaptureReader) Read([]byte) (int, error) { return 0, r.err }

type panicCaptureReader struct{}

func (panicCaptureReader) Read([]byte) (int, error) { panic("randomness read for invalid proposal") }
