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
	options, err := parseQuickFriction([]string{"--quick", "--", "--help"})
	if err != nil {
		t.Fatalf("parseQuickFriction() error = %v", err)
	}
	if options.description != "--help" {
		t.Errorf("description = %q, want --help", options.description)
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

func TestQuickFrictionPersistsExplicitFrequency(t *testing.T) {
	for _, args := range [][]string{
		{"friction", "--quick", "--frequency", "weekly", "spaced frequency"},
		{"friction", "--quick", "--frequency=weekly", "equals frequency"},
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
			if got := readQuickFriction(t, dataDirectory).Details.Friction.Frequency; got != domain.FrequencyWeekly {
				t.Errorf("stored frequency = %q, want weekly", got)
			}
		})
	}
}

func TestQuickFrictionPersistsExplicitImpact(t *testing.T) {
	for _, args := range [][]string{
		{"friction", "--quick", "--impact", "high", "spaced impact"},
		{"friction", "--quick", "--impact=high", "equals impact"},
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
			if got := readQuickFriction(t, dataDirectory).Details.Friction.Impact; got != domain.ImpactHigh {
				t.Errorf("stored impact = %q, want high", got)
			}
		})
	}
}

func TestQuickFrictionPersistsExplicitCategory(t *testing.T) {
	for _, args := range [][]string{
		{"friction", "--quick", "--category", "verification", "spaced category"},
		{"friction", "--quick", "--category=verification", "equals category"},
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
			if got := readQuickFriction(t, dataDirectory).Details.Friction.Category; got != domain.CategoryVerification {
				t.Errorf("stored category = %q, want verification", got)
			}
		})
	}
}

func TestQuickFrictionNormalizesAndPersistsProject(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantProject *string
	}{
		{
			name:        "spaced flag",
			args:        []string{"friction", "--quick", "--project", "  forge  ", "spaced project"},
			wantProject: stringPointer("forge"),
		},
		{
			name:        "equals flag",
			args:        []string{"friction", "--quick", "--project=forge", "equals project"},
			wantProject: stringPointer("forge"),
		},
		{
			name:        "empty project",
			args:        []string{"friction", "--quick", "--project=  ", "empty project"},
			wantProject: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDirectory := filepath.Join(t.TempDir(), "forge-data")
			var stdout bytes.Buffer
			err := Run(
				context.Background(),
				tt.args,
				quickCaptureRuntime(dataDirectory, &stdout),
				"dev",
			)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			record := readQuickFriction(t, dataDirectory)
			if tt.wantProject == nil {
				if record.Project != nil {
					t.Errorf("stored project = %q, want nil", *record.Project)
				}
			} else if record.Project == nil || *record.Project != *tt.wantProject {
				t.Errorf("stored project = %v, want %q", record.Project, *tt.wantProject)
			}
		})
	}
}

func TestQuickFrictionNormalizesAndPersistsCurrentWorkaround(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantWorkaround *string
	}{
		{
			name:           "spaced flag",
			args:           []string{"friction", "--quick", "--current-workaround", "  Search logs manually  ", "spaced workaround"},
			wantWorkaround: stringPointer("Search logs manually"),
		},
		{
			name:           "equals flag",
			args:           []string{"friction", "--quick", "--current-workaround=Search logs manually", "equals workaround"},
			wantWorkaround: stringPointer("Search logs manually"),
		},
		{
			name:           "empty workaround",
			args:           []string{"friction", "--quick", "--current-workaround=  ", "empty workaround"},
			wantWorkaround: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDirectory := filepath.Join(t.TempDir(), "forge-data")
			var stdout bytes.Buffer
			err := Run(
				context.Background(),
				tt.args,
				quickCaptureRuntime(dataDirectory, &stdout),
				"dev",
			)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			workaround := readQuickFriction(t, dataDirectory).Details.Friction.CurrentWorkaround
			if tt.wantWorkaround == nil {
				if workaround != nil {
					t.Errorf("stored workaround = %q, want nil", *workaround)
				}
			} else if workaround == nil || *workaround != *tt.wantWorkaround {
				t.Errorf("stored workaround = %v, want %q", workaround, *tt.wantWorkaround)
			}
		})
	}
}

func TestQuickFrictionWritesJSONRecord(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{
			"friction", "--quick", "--json", "--project", "forge",
			"--frequency=weekly", "--impact", "high", "--category=verification",
			"--current-workaround", "Search logs manually",
			"CI failures require manual log searching",
		},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "{\"id\":\"frc_00000000000000000000000000000000\",\"type\":\"friction\",\"description\":\"CI failures require manual log searching\",\"project\":\"forge\",\"status\":\"captured\",\"details\":{\"frequency\":\"weekly\",\"impact\":\"high\",\"category\":\"verification\",\"current_workaround\":\"Search logs manually\"},\"created_at\":\"2026-08-25T12:00:00.000000Z\",\"updated_at\":\"2026-08-25T12:00:00.000000Z\"}\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if strings.Contains(stdout.String(), "Created friction") {
		t.Errorf("JSON stdout contains human confirmation: %q", stdout.String())
	}

	record := readQuickFriction(t, dataDirectory)
	if record.Description != "CI failures require manual log searching" {
		t.Errorf("stored description = %q, want JSON friction description", record.Description)
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
		{name: "missing frequency", args: []string{"friction", "--quick", "--frequency"}, wantErr: `--frequency requires a value`},
		{name: "empty frequency", args: []string{"friction", "--quick", "--frequency=", "description"}, wantErr: `--frequency requires a value`},
		{name: "missing impact", args: []string{"friction", "--quick", "--impact"}, wantErr: `--impact requires a value`},
		{name: "empty impact", args: []string{"friction", "--quick", "--impact=", "description"}, wantErr: `--impact requires a value`},
		{name: "missing category", args: []string{"friction", "--quick", "--category"}, wantErr: `--category requires a value`},
		{name: "empty category", args: []string{"friction", "--quick", "--category=", "description"}, wantErr: `--category requires a value`},
		{name: "missing project", args: []string{"friction", "--quick", "--project"}, wantErr: `--project requires a value`},
		{name: "missing current workaround", args: []string{"friction", "--quick", "--current-workaround"}, wantErr: `--current-workaround requires a value`},
		{name: "unknown flag", args: []string{"friction", "--quick", "--unknown", "description"}, wantErr: `unknown argument "--unknown"`},
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
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "description", args: []string{"friction", "--quick", "--json", " \t "}, wantErr: "invalid description"},
		{name: "frequency", args: []string{"friction", "--quick", "--json", "--frequency", "often", "description"}, wantErr: `invalid frequency "often"`},
		{name: "impact", args: []string{"friction", "--quick", "--json", "--impact", "severe", "description"}, wantErr: `invalid impact "severe"`},
		{name: "category", args: []string{"friction", "--quick", "--json", "--category", "process", "description"}, wantErr: `invalid category "process"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDirectory := filepath.Join(t.TempDir(), "missing")
			var stdout bytes.Buffer
			rt := quickCaptureRuntime(dataDirectory, &stdout)
			rt.Env = func(string) string {
				t.Fatal("invalid friction inspected environment")
				return ""
			}
			err := Run(context.Background(), tt.args, rt, "dev")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Run() error = %v, want error containing %q", err, tt.wantErr)
			}
			if _, statErr := os.Stat(dataDirectory); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("data directory state error = %v, want not exist", statErr)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestQuickFrictionStorageFailureProducesNoOutput(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"friction", "--quick", "--json", "description"},
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
