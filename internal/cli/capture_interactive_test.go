package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/damienomurchu/forge-cli/internal/prompt"
)

type fakePrompt struct {
	selection      string
	selectErr      error
	selectCalls    int
	selectLabel    string
	selectChoices  []string
	selectDefault  string
	confirmed      bool
	err            error
	confirmCalls   int
	confirmLabel   string
	confirmDefault bool
}

func (p *fakePrompt) Select(_ context.Context, label string, choices []string, defaultValue string) (string, error) {
	p.selectCalls++
	p.selectLabel = label
	p.selectChoices = append([]string(nil), choices...)
	p.selectDefault = defaultValue
	return p.selection, p.selectErr
}

func TestInteractiveCaptureSelectsOmittedKindBeforeConfirmation(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	var stdout bytes.Buffer
	prompter := &fakePrompt{selection: "idea", confirmed: true}
	rt := interactiveCaptureRuntime(dataDirectory, &stdout, prompter)
	if err := Run(
		context.Background(),
		[]string{"capture", "Select a kind"},
		rt,
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	wantChoices := []string{"thought", "idea", "observation", "question", "decision", "seed"}
	if prompter.selectCalls != 1 || prompter.selectLabel != "Kind" ||
		prompter.selectDefault != "thought" || !slices.Equal(prompter.selectChoices, wantChoices) {
		t.Errorf("selection = calls:%d label:%q choices:%v default:%q", prompter.selectCalls, prompter.selectLabel, prompter.selectChoices, prompter.selectDefault)
	}
	if prompter.confirmCalls != 1 {
		t.Errorf("Confirm() calls = %d, want 1", prompter.confirmCalls)
	}
	record := readQuickCapture(t, dataDirectory)
	if record.Details.Capture.Kind.String() != "idea" {
		t.Errorf("stored kind = %q, want idea", record.Details.Capture.Kind)
	}
}

func (p *fakePrompt) Text(context.Context, string) (string, error) {
	return "", errors.New("unexpected text prompt")
}

func (p *fakePrompt) Confirm(_ context.Context, label string, defaultValue bool) (bool, error) {
	p.confirmCalls++
	p.confirmLabel = label
	p.confirmDefault = defaultValue
	return p.confirmed, p.err
}

func TestInteractiveCaptureConfirmsBeforePersisting(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	var stdout bytes.Buffer
	prompter := &fakePrompt{confirmed: true}
	rt := interactiveCaptureRuntime(dataDirectory, &stdout, prompter)
	if err := Run(
		context.Background(),
		[]string{"capture", "--kind", "observation", "--project", "forge", "--tags", "cli,prompt", "Confirm me"},
		rt,
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "Created capture cap_00000000000000000000000000000000\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if prompter.confirmCalls != 1 || prompter.confirmLabel != "Create capture?" || !prompter.confirmDefault {
		t.Errorf("confirmation = calls:%d label:%q default:%t", prompter.confirmCalls, prompter.confirmLabel, prompter.confirmDefault)
	}
	record := readQuickCapture(t, dataDirectory)
	if record.Description != "Confirm me" || record.Details.Capture.Kind.String() != "observation" {
		t.Errorf("stored capture = %#v", record)
	}
}

func TestInteractiveCaptureJSONRemainsExclusive(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	var stdout bytes.Buffer
	rt := interactiveCaptureRuntime(dataDirectory, &stdout, &fakePrompt{confirmed: true})
	if err := Run(
		context.Background(),
		[]string{"capture", "--kind=decision", "--json", "Confirmed JSON"},
		rt,
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "{\"id\":\"cap_00000000000000000000000000000000\",\"type\":\"capture\",\"description\":\"Confirmed JSON\",\"project\":null,\"status\":\"captured\",\"details\":{\"kind\":\"decision\",\"tags\":[]},\"created_at\":\"2026-08-25T12:00:00.000000Z\",\"updated_at\":\"2026-08-25T12:00:00.000000Z\"}\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if strings.Contains(stdout.String(), "Create capture?") || strings.Contains(stdout.String(), "Created capture") {
		t.Errorf("JSON stdout contains human output: %q", stdout.String())
	}
}

func TestInteractiveCaptureDeclineWritesNothingAndDoesNotInspectStorage(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	prompter := &fakePrompt{confirmed: false}
	rt := interactiveCaptureRuntime(dataDirectory, &stdout, prompter)
	rt.Now = nil
	rt.Random = nil
	rt.Env = func(string) string {
		t.Fatal("declined capture inspected the environment")
		return ""
	}
	if err := Run(
		context.Background(),
		[]string{"capture", "--kind=thought", "Decline me"},
		rt,
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if prompter.confirmCalls != 1 {
		t.Errorf("Confirm() calls = %d, want 1", prompter.confirmCalls)
	}
	if _, err := os.Stat(dataDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("data directory state error = %v, want not exist", err)
	}
}

func TestInteractiveCaptureRequiresTTYBeforeConstructingPromptOrStorage(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	rt := quickCaptureRuntime(dataDirectory, &stdout)
	rt.IsTTY = func() bool { return false }
	rt.Prompt = func() Prompt {
		t.Fatal("non-TTY capture constructed prompt")
		return nil
	}
	rt.Env = func(string) string {
		t.Fatal("non-TTY capture inspected the environment")
		return ""
	}
	err := Run(
		context.Background(),
		[]string{"capture", "--kind", "thought", "Non-TTY"},
		rt,
		"dev",
	)
	if err == nil || !strings.Contains(err.Error(), "stdin is not a terminal") {
		t.Fatalf("Run() error = %v, want non-TTY validation error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestInteractiveCaptureValidatesBeforeTTYDetection(t *testing.T) {
	rt := quickCaptureRuntime(t.TempDir(), &bytes.Buffer{})
	rt.IsTTY = func() bool {
		t.Fatal("invalid capture checked TTY")
		return false
	}
	err := Run(
		context.Background(),
		[]string{"capture", "--kind", "thought", " \t "},
		rt,
		"dev",
	)
	if err == nil || !strings.Contains(err.Error(), "invalid description") {
		t.Fatalf("Run() error = %v, want description validation error", err)
	}
}

func TestInteractiveCaptureCancellationMapsToInterruptedWithoutStorage(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	prompter := &fakePrompt{err: prompt.ErrCancelled}
	rt := interactiveCaptureRuntime(dataDirectory, &stdout, prompter)
	rt.Env = func(string) string {
		t.Fatal("cancelled capture inspected the environment")
		return ""
	}
	err := Run(
		context.Background(),
		[]string{"capture", "--kind", "thought", "Cancel me"},
		rt,
		"dev",
	)
	var stderr bytes.Buffer
	if got, want := WriteError(&stderr, err), 130; got != want {
		t.Errorf("exit status = %d, want %d", got, want)
	}
	if got, want := stderr.String(), "forge: capture cancelled\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if _, statErr := os.Stat(dataDirectory); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("data directory state error = %v, want not exist", statErr)
	}
}

func TestInteractiveCaptureSelectionCancellationSkipsConfirmationAndStorage(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	prompter := &fakePrompt{selectErr: prompt.ErrCancelled}
	rt := interactiveCaptureRuntime(dataDirectory, &stdout, prompter)
	rt.Now = nil
	rt.Random = nil
	rt.Env = func(string) string {
		t.Fatal("cancelled selection inspected the environment")
		return ""
	}
	err := Run(context.Background(), []string{"capture", "Cancel selection"}, rt, "dev")
	var interrupted *InterruptedError
	if !errors.As(err, &interrupted) || !errors.Is(err, prompt.ErrCancelled) {
		t.Fatalf("Run() error = %T %v, want interrupted cancellation", err, err)
	}
	if prompter.confirmCalls != 0 {
		t.Errorf("Confirm() calls = %d, want 0", prompter.confirmCalls)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if _, statErr := os.Stat(dataDirectory); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("data directory state error = %v, want not exist", statErr)
	}
}

func TestInteractiveCaptureRejectsInvalidSelectedKindBeforeConfirmationOrStorage(t *testing.T) {
	var stdout bytes.Buffer
	prompter := &fakePrompt{selection: "invalid"}
	rt := interactiveCaptureRuntime(t.TempDir(), &stdout, prompter)
	rt.Env = func(string) string {
		t.Fatal("invalid selection inspected the environment")
		return ""
	}
	err := Run(context.Background(), []string{"capture", "Invalid selection"}, rt, "dev")
	if err == nil || !strings.Contains(err.Error(), "invalid capture kind") {
		t.Fatalf("Run() error = %v, want selected-kind validation", err)
	}
	if prompter.confirmCalls != 0 {
		t.Errorf("Confirm() calls = %d, want 0", prompter.confirmCalls)
	}
}

func TestInteractiveCaptureEOFIsDistinctFailureWithoutStorage(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	prompter := &fakePrompt{err: prompt.ErrEOF}
	rt := interactiveCaptureRuntime(dataDirectory, &stdout, prompter)
	rt.Env = func(string) string {
		t.Fatal("EOF capture inspected the environment")
		return ""
	}
	err := Run(
		context.Background(),
		[]string{"capture", "--kind", "thought", "EOF"},
		rt,
		"dev",
	)
	if !errors.Is(err, prompt.ErrEOF) {
		t.Fatalf("Run() error = %v, want ErrEOF", err)
	}
	var stderr bytes.Buffer
	if got, want := WriteError(&stderr, err), 1; got != want {
		t.Errorf("exit status = %d, want %d", got, want)
	}
	if !strings.Contains(stderr.String(), "prompt input closed") {
		t.Errorf("stderr = %q, want EOF message", stderr.String())
	}
}

func TestQuickCaptureDoesNotConstructPrompt(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	rt := quickCaptureRuntime(dataDirectory, &bytes.Buffer{})
	rt.IsTTY = func() bool {
		t.Fatal("quick capture checked TTY")
		return false
	}
	rt.Prompt = func() Prompt {
		t.Fatal("quick capture constructed prompt")
		return nil
	}
	if err := Run(context.Background(), []string{"capture", "--quick", "Quick"}, rt, "dev"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func interactiveCaptureRuntime(dataDirectory string, stdout *bytes.Buffer, prompter Prompt) Runtime {
	rt := quickCaptureRuntime(dataDirectory, stdout)
	rt.IsTTY = func() bool { return true }
	rt.Prompt = func() Prompt { return prompter }
	return rt
}
