package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/prompt"
)

type scriptedCapturePrompt struct {
	selectValues []string
	textValues   []string
	confirmed    bool
	failAt       int
	failErr      error
	calls        int
	events       []capturePromptEvent
}

type capturePromptEvent struct {
	kind         string
	label        string
	choices      []string
	defaultValue any
}

func (p *scriptedCapturePrompt) Select(_ context.Context, label string, choices []string, defaultValue string) (string, error) {
	p.events = append(p.events, capturePromptEvent{
		kind: "select", label: label, choices: append([]string(nil), choices...), defaultValue: defaultValue,
	})
	if err := p.nextError(); err != nil {
		return "", err
	}
	if len(p.selectValues) == 0 {
		return "", errors.New("unexpected select prompt")
	}
	value := p.selectValues[0]
	p.selectValues = p.selectValues[1:]
	return value, nil
}

func (p *scriptedCapturePrompt) Text(_ context.Context, label string) (string, error) {
	p.events = append(p.events, capturePromptEvent{kind: "text", label: label})
	if err := p.nextError(); err != nil {
		return "", err
	}
	if len(p.textValues) == 0 {
		return "", errors.New("unexpected text prompt")
	}
	value := p.textValues[0]
	p.textValues = p.textValues[1:]
	return value, nil
}

func (p *scriptedCapturePrompt) Confirm(_ context.Context, label string, defaultValue bool) (bool, error) {
	p.events = append(p.events, capturePromptEvent{kind: "confirm", label: label, defaultValue: defaultValue})
	if err := p.nextError(); err != nil {
		return false, err
	}
	return p.confirmed, nil
}

func (p *scriptedCapturePrompt) nextError() error {
	p.calls++
	if p.failAt == p.calls {
		return p.failErr
	}
	return nil
}

func TestCollectCaptureCollectsEveryType(t *testing.T) {
	for _, captureType := range domain.CaptureTypes() {
		t.Run(captureType.String(), func(t *testing.T) {
			prompter := &scriptedCapturePrompt{selectValues: []string{captureType.String()}, confirmed: true}
			if captureType == domain.CaptureTypeFriction {
				prompter.selectValues = append(prompter.selectValues, "weekly", "high", "verification")
				prompter.textValues = []string{"  forge  ", "  Use a checklist  "}
			}
			var summary bytes.Buffer
			got, confirmed, err := collectCapture(
				context.Background(), interactiveCaptureRequest(), prompter, &summary,
			)
			if err != nil {
				t.Fatalf("collectCapture() error = %v", err)
			}
			if !confirmed || got.Type != captureType || got.Description != "description" {
				t.Fatalf("capture/confirmed = %#v/%t", got, confirmed)
			}
			if err := got.Validate(); err != nil {
				t.Errorf("capture validation error = %v", err)
			}
			if !strings.Contains(summary.String(), "Capture summary\nType: "+captureType.String()+"\n") {
				t.Errorf("summary = %q", summary.String())
			}
			if captureType == domain.CaptureTypeFriction {
				details := got.Details.Friction
				if details.Project == nil || *details.Project != "forge" ||
					details.CurrentWorkaround == nil || *details.CurrentWorkaround != "Use a checklist" {
					t.Errorf("friction details = %#v", details)
				}
			} else if len(prompter.events) != 2 {
				t.Errorf("minimal capture events = %#v, want type and confirmation only", prompter.events)
			}
		})
	}
}

func TestCollectCaptureFrictionPromptOrderAndDefaults(t *testing.T) {
	prompter := &scriptedCapturePrompt{
		selectValues: []string{"friction", "unknown", "unknown", "other"},
		textValues:   []string{"", ""},
		confirmed:    true,
	}
	var summary bytes.Buffer
	_, _, err := collectCapture(context.Background(), interactiveCaptureRequest(), prompter, &summary)
	if err != nil {
		t.Fatalf("collectCapture() error = %v", err)
	}
	want := []capturePromptEvent{
		{kind: "select", label: "Type", choices: []string{"friction", "action", "follow-up", "decision"}, defaultValue: "friction"},
		{kind: "text", label: "Project (optional)"},
		{kind: "select", label: "Frequency", choices: []string{"daily", "weekly", "monthly", "occasional", "unknown"}, defaultValue: "unknown"},
		{kind: "select", label: "Impact", choices: []string{"low", "medium", "high", "unknown"}, defaultValue: "unknown"},
		{kind: "select", label: "Category", choices: []string{"information-finding", "repeated-action", "context-switching", "remembering", "verification", "waiting", "other"}, defaultValue: "other"},
		{kind: "text", label: "Current workaround (optional)"},
		{kind: "confirm", label: "Create capture?", defaultValue: true},
	}
	if !reflect.DeepEqual(prompter.events, want) {
		t.Errorf("prompt events = %#v, want %#v", prompter.events, want)
	}
	if !strings.Contains(summary.String(), "Project: (none)\n") ||
		!strings.Contains(summary.String(), "Current workaround: (none)\n") {
		t.Errorf("summary = %q, want absent optional text", summary.String())
	}
}

func TestCollectCaptureDeclineReturnsNoProposal(t *testing.T) {
	prompter := &scriptedCapturePrompt{selectValues: []string{"action"}, confirmed: false}
	var summary bytes.Buffer
	got, confirmed, err := collectCapture(
		context.Background(), interactiveCaptureRequest(), prompter, &summary,
	)
	if err != nil {
		t.Fatalf("collectCapture() error = %v", err)
	}
	if confirmed || got != (domain.ProposedCapture{}) {
		t.Errorf("capture/confirmed = %#v/%t, want zero/false", got, confirmed)
	}
	if summary.Len() == 0 {
		t.Error("declined capture did not render summary")
	}
}

func TestCollectCapturePreservesCancellationAndEOFAtEveryStage(t *testing.T) {
	for _, promptErr := range []error{prompt.ErrCancelled, prompt.ErrEOF} {
		for stage := 1; stage <= 7; stage++ {
			t.Run(promptErr.Error()+" stage "+string(rune('0'+stage)), func(t *testing.T) {
				prompter := &scriptedCapturePrompt{
					selectValues: []string{"friction", "unknown", "unknown", "other"},
					textValues:   []string{"", ""},
					confirmed:    true,
					failAt:       stage,
					failErr:      promptErr,
				}
				got, confirmed, err := collectCapture(
					context.Background(), interactiveCaptureRequest(), prompter, io.Discard,
				)
				if got != (domain.ProposedCapture{}) || confirmed {
					t.Errorf("capture/confirmed = %#v/%t, want zero/false", got, confirmed)
				}
				if !errors.Is(err, promptErr) {
					t.Fatalf("error = %v, want %v", err, promptErr)
				}
				if errors.Is(promptErr, prompt.ErrCancelled) {
					var interrupted *InterruptedError
					if !errors.As(err, &interrupted) {
						t.Fatalf("cancellation error = %T %v, want InterruptedError", err, err)
					}
				}
			})
		}
	}
}

func TestCollectCaptureRejectsInvalidPromptSelections(t *testing.T) {
	tests := []struct {
		name         string
		selectValues []string
		textValues   []string
		wantField    string
	}{
		{name: "type", selectValues: []string{"invalid"}, wantField: "capture type"},
		{name: "frequency", selectValues: []string{"friction", "invalid"}, textValues: []string{""}, wantField: "frequency"},
		{name: "impact", selectValues: []string{"friction", "unknown", "invalid"}, textValues: []string{""}, wantField: "impact"},
		{name: "category", selectValues: []string{"friction", "unknown", "unknown", "invalid"}, textValues: []string{""}, wantField: "category"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompter := &scriptedCapturePrompt{selectValues: tt.selectValues, textValues: tt.textValues, confirmed: true}
			got, confirmed, err := collectCapture(
				context.Background(), interactiveCaptureRequest(), prompter, io.Discard,
			)
			var invalid *domain.InvalidValueError
			if !errors.As(err, &invalid) || invalid.Field != tt.wantField {
				t.Fatalf("error = %T %v, want invalid %s", err, err, tt.wantField)
			}
			if got != (domain.ProposedCapture{}) || confirmed {
				t.Errorf("capture/confirmed = %#v/%t, want zero/false", got, confirmed)
			}
			for _, event := range prompter.events {
				if event.kind == "confirm" {
					t.Error("invalid selection reached confirmation")
				}
			}
		})
	}
}

func TestCollectCapturePropagatesSummaryWriterFailureBeforeConfirmation(t *testing.T) {
	wantErr := errors.New("writer failed")
	prompter := &scriptedCapturePrompt{selectValues: []string{"action"}, confirmed: true}
	got, confirmed, err := collectCapture(
		context.Background(), interactiveCaptureRequest(), prompter, failingCaptureWriter{err: wantErr},
	)
	if !errors.Is(err, wantErr) || got != (domain.ProposedCapture{}) || confirmed {
		t.Fatalf("capture/confirmed/error = %#v/%t/%v", got, confirmed, err)
	}
	for _, event := range prompter.events {
		if event.kind == "confirm" {
			t.Error("summary failure reached confirmation")
		}
	}
}

func TestCollectCaptureRejectsNonInteractiveRequestAndMissingBoundaries(t *testing.T) {
	quick, err := parseCaptureRequest([]string{"--quick", "--type", "action", "description"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		request   captureRequest
		prompter  Prompt
		writer    io.Writer
		wantError string
	}{
		{name: "quick request", request: quick, prompter: &scriptedCapturePrompt{}, writer: io.Discard, wantError: "interactive capture request is required"},
		{name: "missing prompt", request: interactiveCaptureRequest(), writer: io.Discard, wantError: "capture prompt is required"},
		{name: "missing writer", request: interactiveCaptureRequest(), prompter: &scriptedCapturePrompt{}, wantError: "capture summary writer is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := collectCapture(context.Background(), tt.request, tt.prompter, tt.writer)
			if err == nil || err.Error() != tt.wantError {
				t.Fatalf("error = %v, want %q", err, tt.wantError)
			}
		})
	}
}

func TestCollectCaptureValidatesDescriptionBeforePrompting(t *testing.T) {
	prompter := &scriptedCapturePrompt{}
	_, confirmed, err := collectCapture(
		context.Background(), captureRequest{description: "   "}, prompter, io.Discard,
	)
	var validation *domain.InvalidValueError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %T %v, want InvalidValueError", err, err)
	}
	if confirmed {
		t.Error("invalid description was confirmed")
	}
	if len(prompter.events) != 0 {
		t.Errorf("prompt events = %#v, want none", prompter.events)
	}
}

func interactiveCaptureRequest() captureRequest {
	return captureRequest{description: "description"}
}

type failingCaptureWriter struct{ err error }

func (w failingCaptureWriter) Write([]byte) (int, error) { return 0, w.err }
