package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/damienomurchu/forge-cli/internal/prompt"
)

func TestInteractiveFrictionConfirmsBeforePersisting(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	var stdout bytes.Buffer
	prompter := &fakePrompt{confirmed: true}
	rt := interactiveCaptureRuntime(dataDirectory, &stdout, prompter)
	err := Run(context.Background(), []string{
		"friction", "--frequency", "weekly", "--impact=high",
		"--category", "verification", "Confirm friction",
	}, rt, "dev")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "Created friction frc_00000000000000000000000000000000\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if prompter.confirmCalls != 1 || prompter.confirmLabel != "Create friction?" || !prompter.confirmDefault {
		t.Errorf("confirmation = calls:%d label:%q default:%t", prompter.confirmCalls, prompter.confirmLabel, prompter.confirmDefault)
	}
	record := readQuickFriction(t, dataDirectory)
	if record.Description != "Confirm friction" || record.Details.Friction.Frequency.String() != "weekly" ||
		record.Details.Friction.Impact.String() != "high" || record.Details.Friction.Category.String() != "verification" {
		t.Errorf("stored friction = %#v", record)
	}
}

func TestInteractiveFrictionDeclineWritesNothingAndDoesNotInspectStorage(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	prompter := &fakePrompt{confirmed: false}
	rt := interactiveCaptureRuntime(dataDirectory, &stdout, prompter)
	rt.Now = nil
	rt.Random = nil
	rt.Env = func(string) string {
		t.Fatal("declined friction inspected the environment")
		return ""
	}
	err := Run(context.Background(), explicitFrictionArgs("Decline friction"), rt, "dev")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if _, err := os.Stat(dataDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("data directory state error = %v, want not exist", err)
	}
}

func TestInteractiveFrictionRequiresTTYBeforePromptOrStorage(t *testing.T) {
	var stdout bytes.Buffer
	rt := quickCaptureRuntime(t.TempDir(), &stdout)
	rt.IsTTY = func() bool { return false }
	rt.Prompt = func() Prompt {
		t.Fatal("non-TTY friction constructed prompt")
		return nil
	}
	rt.Env = func(string) string {
		t.Fatal("non-TTY friction inspected the environment")
		return ""
	}
	err := Run(context.Background(), explicitFrictionArgs("Non-TTY friction"), rt, "dev")
	if err == nil || !strings.Contains(err.Error(), "stdin is not a terminal") {
		t.Fatalf("Run() error = %v, want non-TTY validation error", err)
	}
}

func TestInteractiveFrictionCancellationMapsToInterruptedWithoutStorage(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	rt := interactiveCaptureRuntime(dataDirectory, &stdout, &fakePrompt{err: prompt.ErrCancelled})
	rt.Env = func(string) string {
		t.Fatal("cancelled friction inspected the environment")
		return ""
	}
	err := Run(context.Background(), explicitFrictionArgs("Cancel friction"), rt, "dev")
	var stderr bytes.Buffer
	if got, want := WriteError(&stderr, err), 130; got != want {
		t.Errorf("exit status = %d, want %d", got, want)
	}
	if got, want := stderr.String(), "forge: friction cancelled\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
	if _, statErr := os.Stat(dataDirectory); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("data directory state error = %v, want not exist", statErr)
	}
}

func TestInteractiveFrictionEOFIsDistinctFailure(t *testing.T) {
	rt := interactiveCaptureRuntime(t.TempDir(), &bytes.Buffer{}, &fakePrompt{err: prompt.ErrEOF})
	err := Run(context.Background(), explicitFrictionArgs("EOF friction"), rt, "dev")
	if !errors.Is(err, prompt.ErrEOF) {
		t.Fatalf("Run() error = %v, want ErrEOF", err)
	}
}

func TestQuickFrictionDoesNotConstructPrompt(t *testing.T) {
	rt := quickCaptureRuntime(t.TempDir(), &bytes.Buffer{})
	rt.IsTTY = func() bool {
		t.Fatal("quick friction checked TTY")
		return false
	}
	rt.Prompt = func() Prompt {
		t.Fatal("quick friction constructed prompt")
		return nil
	}
	if err := Run(context.Background(), []string{"friction", "--quick", "Quick friction"}, rt, "dev"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func explicitFrictionArgs(description string) []string {
	return []string{
		"friction", "--frequency", "weekly", "--impact", "high",
		"--category", "verification", description,
	}
}
