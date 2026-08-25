package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/repository"
	"github.com/damienomurchu/forge-cli/internal/storage"
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

func TestCaptureHelp(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "capture-help.golden"))
	if err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"capture", "-h"},
		{"capture", "--help"},
		{"capture", "--quick", "--unknown", "--help"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			var stdout bytes.Buffer
			rt := Runtime{
				Stdout: &stdout,
				Env: func(string) string {
					t.Fatal("capture help inspected the environment")
					return ""
				},
			}
			if err := Run(context.Background(), args, rt, "dev"); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != string(want) {
				t.Errorf("capture help mismatch\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestCaptureHelpAfterOptionTerminatorIsDescription(t *testing.T) {
	options, err := parseQuickCapture([]string{"--quick", "--", "--help"})
	if err != nil {
		t.Fatalf("parseQuickCapture() error = %v", err)
	}
	if options.description != "--help" {
		t.Errorf("description = %q, want --help", options.description)
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

func TestQuickCaptureCreatesRecord(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	var stdout bytes.Buffer
	rt := quickCaptureRuntime(dataDirectory, &stdout)

	err := Run(context.Background(), []string{"capture", "--quick", "  Investigate startup cost  "}, rt, "dev")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	const id = "cap_00000000000000000000000000000000"
	if got, want := stdout.String(), "Created capture "+id+"\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	session, err := storage.OpenExisting(
		context.Background(),
		filepath.Join(dataDirectory, "forge.db"),
		os.Geteuid(),
		storage.DatabaseReadOnly,
	)
	if err != nil {
		t.Fatalf("OpenExisting() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	repo, err := repository.New(session.Database())
	if err != nil {
		t.Fatalf("repository.New() error = %v", err)
	}
	record, err := repo.FindByID(context.Background(), domain.ID(id))
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if record.Description != "Investigate startup cost" ||
		record.Details.Capture.Kind != domain.CaptureKindThought ||
		len(record.Details.Capture.Tags) != 0 {
		t.Errorf("stored record = %+v, want normalized quick-capture defaults", record)
	}
}

func TestQuickCaptureAcceptsOptionTerminator(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"capture", "--quick", "--", "- investigate startup cost"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.HasPrefix(stdout.String(), "Created capture cap_") {
		t.Errorf("stdout = %q, want capture confirmation", stdout.String())
	}
}

func TestQuickCapturePersistsExplicitKind(t *testing.T) {
	for _, args := range [][]string{
		{"capture", "--quick", "--kind", "idea", "spaced kind"},
		{"capture", "--quick", "--kind=idea", "equals kind"},
	} {
		t.Run(args[2], func(t *testing.T) {
			dataDirectory := filepath.Join(t.TempDir(), "forge-data")
			var stdout bytes.Buffer
			err := Run(
				context.Background(),
				args,
				quickCaptureRuntime(dataDirectory, &stdout),
				"dev",
			)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			session, err := storage.OpenExisting(
				context.Background(),
				filepath.Join(dataDirectory, "forge.db"),
				os.Geteuid(),
				storage.DatabaseReadOnly,
			)
			if err != nil {
				t.Fatalf("OpenExisting() error = %v", err)
			}
			repo, err := repository.New(session.Database())
			if err != nil {
				_ = session.Close()
				t.Fatalf("repository.New() error = %v", err)
			}
			record, err := repo.FindByID(context.Background(), domain.ID("cap_00000000000000000000000000000000"))
			closeErr := session.Close()
			if err != nil {
				t.Fatalf("FindByID() error = %v", err)
			}
			if closeErr != nil {
				t.Fatalf("Session.Close() error = %v", closeErr)
			}
			if record.Details.Capture.Kind != domain.CaptureKindIdea {
				t.Errorf("stored kind = %q, want idea", record.Details.Capture.Kind)
			}
		})
	}
}

func TestQuickCaptureUsageErrorsDoNotInspectEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing description", args: []string{"capture", "--quick"}, wantErr: "capture requires a description"},
		{name: "interactive not implemented", args: []string{"capture", "description"}, wantErr: "capture currently requires --quick"},
		{name: "missing kind", args: []string{"capture", "--quick", "--kind"}, wantErr: `--kind requires a value`},
		{name: "empty kind", args: []string{"capture", "--quick", "--kind=", "description"}, wantErr: `--kind requires a value`},
		{name: "unknown flag", args: []string{"capture", "--quick", "--project", "forge", "description"}, wantErr: `unknown argument "--project"`},
		{name: "extra description", args: []string{"capture", "--quick", "one", "two"}, wantErr: `unexpected argument "two"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := quickCaptureRuntime(t.TempDir(), &bytes.Buffer{})
			rt.Env = func(string) string {
				t.Fatal("usage error inspected environment")
				return ""
			}
			err := Run(context.Background(), tt.args, rt, "dev")
			var usageErr *UsageError
			if !errors.As(err, &usageErr) {
				t.Fatalf("Run() error = %T %v, want *UsageError", err, err)
			}
			if err.Error() != tt.wantErr {
				t.Errorf("Run() error = %q, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestQuickCaptureValidationHappensBeforeStorage(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "description", args: []string{"capture", "--quick", " \t "}, wantErr: "invalid description"},
		{name: "kind", args: []string{"capture", "--quick", "--kind", "not-a-kind", "description"}, wantErr: `invalid capture kind "not-a-kind"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDirectory := filepath.Join(t.TempDir(), "missing")
			rt := quickCaptureRuntime(dataDirectory, &bytes.Buffer{})
			rt.Env = func(string) string {
				t.Fatal("invalid capture inspected environment")
				return ""
			}
			err := Run(context.Background(), tt.args, rt, "dev")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Run() error = %v, want error containing %q", err, tt.wantErr)
			}
			if _, statErr := os.Stat(dataDirectory); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("data directory state error = %v, want not exist", statErr)
			}
		})
	}
}

func TestQuickCaptureStorageFailureProducesNoOutput(t *testing.T) {
	var stdout bytes.Buffer
	rt := quickCaptureRuntime("relative", &stdout)
	err := Run(context.Background(), []string{"capture", "--quick", "description"}, rt, "dev")
	if err == nil || !strings.Contains(err.Error(), "FORGE_DATA_DIR must be an absolute path") {
		t.Fatalf("Run() error = %v, want path error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func quickCaptureRuntime(dataDirectory string, stdout *bytes.Buffer) Runtime {
	return Runtime{
		Stdout: stdout,
		Env: func(name string) string {
			if name == "FORGE_DATA_DIR" {
				return dataDirectory
			}
			return ""
		},
		Now:    func() time.Time { return time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC) },
		Random: bytes.NewReader(make([]byte, 16)),
		GOOS:   runtime.GOOS,
		EUID:   os.Geteuid(),
	}
}

func argumentName(args []string) string {
	if len(args) == 0 {
		return "no arguments"
	}
	return args[0]
}
