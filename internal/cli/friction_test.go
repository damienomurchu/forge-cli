package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/repository"
	"github.com/damienomurchu/forge-cli/internal/storage"
)

func TestFrictionHelp(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "friction-help.golden"))
	if err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"friction", "-h"},
		{"friction", "--help"},
		{"friction", "--quick", "--unknown", "--help"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			var stdout bytes.Buffer
			rt := Runtime{
				Stdout: &stdout,
				Env: func(string) string {
					t.Fatal("friction help inspected the environment")
					return ""
				},
			}
			if err := Run(context.Background(), args, rt, "dev"); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != string(want) {
				t.Errorf("friction help mismatch\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestFrictionHelpAfterOptionTerminatorIsDescription(t *testing.T) {
	description, err := parseQuickFriction([]string{"--quick", "--", "--help"})
	if err != nil {
		t.Fatalf("parseQuickFriction() error = %v", err)
	}
	if description != "--help" {
		t.Errorf("description = %q, want --help", description)
	}
}

func TestQuickFrictionCreatesRecordWithDefaults(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"friction", "--quick", "  CI failures require manual log searching  "},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	const id = "frc_00000000000000000000000000000000"
	if got, want := stdout.String(), "Created friction "+id+"\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	record := readQuickFriction(t, dataDirectory)
	details := record.Details.Friction
	if record.Description != "CI failures require manual log searching" ||
		record.Project != nil ||
		details.Frequency != domain.FrequencyUnknown ||
		details.Impact != domain.ImpactUnknown ||
		details.Category != domain.CategoryOther ||
		details.CurrentWorkaround != nil {
		t.Errorf("stored record = %+v, want normalized quick-friction defaults", record)
	}
}

func TestQuickFrictionAcceptsOptionTerminator(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"friction", "--quick", "--", "- repeated manual release"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := readQuickFriction(t, dataDirectory).Description; got != "- repeated manual release" {
		t.Errorf("stored description = %q, want leading-hyphen description", got)
	}
}

func TestQuickFrictionUsageErrorsDoNotInspectEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing description", args: []string{"friction", "--quick"}, wantErr: "friction requires a description"},
		{name: "interactive not implemented", args: []string{"friction", "description"}, wantErr: "friction currently requires --quick"},
		{name: "unknown flag", args: []string{"friction", "--quick", "--impact", "high", "description"}, wantErr: `unknown argument "--impact"`},
		{name: "extra description", args: []string{"friction", "--quick", "one", "two"}, wantErr: `unexpected argument "two"`},
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

func TestQuickFrictionValidationHappensBeforeStorage(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	rt := quickCaptureRuntime(dataDirectory, &stdout)
	rt.Env = func(string) string {
		t.Fatal("invalid friction inspected environment")
		return ""
	}
	err := Run(context.Background(), []string{"friction", "--quick", " \t "}, rt, "dev")
	if err == nil || !strings.Contains(err.Error(), "invalid description") {
		t.Fatalf("Run() error = %v, want invalid-description error", err)
	}
	if _, statErr := os.Stat(dataDirectory); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("data directory state error = %v, want not exist", statErr)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestQuickFrictionStorageFailureProducesNoOutput(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"friction", "--quick", "description"},
		quickCaptureRuntime("relative", &stdout),
		"dev",
	)
	if err == nil || !strings.Contains(err.Error(), "FORGE_DATA_DIR must be an absolute path") {
		t.Fatalf("Run() error = %v, want path error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func readQuickFriction(t *testing.T, dataDirectory string) domain.Record {
	t.Helper()
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
	record, err := repo.FindByID(context.Background(), domain.ID("frc_00000000000000000000000000000000"))
	closeErr := session.Close()
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if closeErr != nil {
		t.Fatalf("Session.Close() error = %v", closeErr)
	}
	return record
}
