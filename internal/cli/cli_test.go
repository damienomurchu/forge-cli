package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestHelp(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "help.golden"))
	if err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{nil, {"-h"}, {"--help"}} {
		t.Run(argumentName(args), func(t *testing.T) {
			var stdout bytes.Buffer
			err := Run(context.Background(), args, Runtime{Stdout: &stdout}, "dev")
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != string(want) {
				t.Errorf("help output mismatch\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"--version"}, Runtime{Stdout: &stdout}, "0.1.0")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "forge 0.1.0\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestUsageError(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"unknown"}, Runtime{Stdout: &stdout}, "dev")
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}

	var usageErr *UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("Run() error = %T %v, want *UsageError", err, err)
	}

	var stderr bytes.Buffer
	if got, want := WriteError(&stderr, err), 2; got != want {
		t.Errorf("exit status = %d, want %d", got, want)
	}
	if got, want := stderr.String(), "forge: unknown argument \"unknown\"\nTry 'forge --help' for usage.\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestDataFreeInvocationsDoNotUseEnvironmentOrFilesystem(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	for _, args := range [][]string{nil, {"-h"}, {"--help"}, {"--version"}, {"unknown"}} {
		t.Run(argumentName(args), func(t *testing.T) {
			var stdout bytes.Buffer
			rt := Runtime{
				Stdout: &stdout,
				Env: func(string) string {
					t.Fatal("data-free invocation inspected the environment")
					return dataDir
				},
			}
			_ = Run(context.Background(), args, rt, "dev")
			if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
				t.Fatalf("data directory state error = %v, want not exist", err)
			}
		})
	}
}

func TestWriteErrorMapsOperationalFailure(t *testing.T) {
	var stderr bytes.Buffer
	if got, want := WriteError(&stderr, errors.New("write failed")), 1; got != want {
		t.Errorf("exit status = %d, want %d", got, want)
	}
	if got, want := stderr.String(), "forge: write failed\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func argumentName(args []string) string {
	if len(args) == 0 {
		return "no arguments"
	}
	return args[0]
}
