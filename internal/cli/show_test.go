package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/damienomurchu/forge-cli/internal/storage"
)

func TestShowHelpIsDataFree(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "show-help.golden"))
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"show", "-h"},
		{"show", "--help"},
		{"show", "extra", "--unknown", "--help"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			var stdout bytes.Buffer
			rt := Runtime{
				Stdout: &stdout,
				Env: func(string) string {
					t.Fatal("show help inspected the environment")
					return ""
				},
			}
			if err := Run(context.Background(), args, rt, "dev"); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != string(want) {
				t.Errorf("show help mismatch\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestShowWritesCompleteHumanCapture(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	if err := Run(
		context.Background(),
		[]string{"capture", "--quick", "--project", "forge", "--kind", "observation", "--tags", "performance,cli", "Measure startup"},
		quickCaptureRuntime(dataDirectory, &bytes.Buffer{}),
		"dev",
	); err != nil {
		t.Fatalf("create capture error = %v", err)
	}

	var stdout bytes.Buffer
	if err := Run(
		context.Background(),
		[]string{"show", "cap_00000000000000000000000000000000"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "ID: cap_00000000000000000000000000000000\n" +
		"Type: capture\n" +
		"Description: Measure startup\n" +
		"Project: forge\n" +
		"Status: captured\n" +
		"Kind: observation\n" +
		"Tags: performance, cli\n" +
		"Created: 2026-08-25T12:00:00.000000Z\n" +
		"Updated: 2026-08-25T12:00:00.000000Z\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestShowWritesCompleteFrictionJSONWithFlagBeforeID(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	if err := Run(
		context.Background(),
		[]string{"friction", "--quick", "--frequency", "weekly", "--impact", "high", "--category", "verification", "--current-workaround", "Search logs", "Manual checks"},
		quickCaptureRuntime(dataDirectory, &bytes.Buffer{}),
		"dev",
	); err != nil {
		t.Fatalf("create friction error = %v", err)
	}

	var stdout bytes.Buffer
	if err := Run(
		context.Background(),
		[]string{"show", "--json", "frc_00000000000000000000000000000000"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "{\"id\":\"frc_00000000000000000000000000000000\",\"type\":\"friction\",\"description\":\"Manual checks\",\"project\":null,\"status\":\"captured\",\"details\":{\"frequency\":\"weekly\",\"impact\":\"high\",\"category\":\"verification\",\"current_workaround\":\"Search logs\"},\"created_at\":\"2026-08-25T12:00:00.000000Z\",\"updated_at\":\"2026-08-25T12:00:00.000000Z\"}\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestShowMissingStorageIsNotFoundAndReadOnly(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"show", "opaque-id"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	)
	if err == nil || err.Error() != `record "opaque-id" not found` {
		t.Fatalf("Run() error = %v, want not-found error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if _, statErr := os.Stat(dataDirectory); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("data directory state error = %v, want not exist", statErr)
	}
}

func TestShowMissingRecordIsNotFound(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	if err := Run(
		context.Background(),
		[]string{"capture", "--quick", "Stored capture"},
		quickCaptureRuntime(dataDirectory, &bytes.Buffer{}),
		"dev",
	); err != nil {
		t.Fatalf("create capture error = %v", err)
	}
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"show", "opaque-id"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	)
	if err == nil || err.Error() != `record "opaque-id" not found` {
		t.Fatalf("Run() error = %v, want not-found error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestShowUsageErrorsDoNotInspectEnvironment(t *testing.T) {
	for _, args := range [][]string{
		{"show"},
		{"show", "--json"},
		{"show", "one", "two"},
		{"show", "--unknown", "one"},
		{"show", "--json=value", "one"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			rt := quickCaptureRuntime(t.TempDir(), &bytes.Buffer{})
			rt.Env = func(string) string {
				t.Fatal("show usage error inspected environment")
				return ""
			}
			err := Run(context.Background(), args, rt, "dev")
			var usageErr *UsageError
			if !errors.As(err, &usageErr) {
				t.Fatalf("Run() error = %T %v, want *UsageError", err, err)
			}
		})
	}
}

func TestShowInvalidIDDoesNotInspectEnvironment(t *testing.T) {
	for _, id := range []string{"", "   ", " surrounding ", "control\ncharacter", "control\x00character"} {
		t.Run(id, func(t *testing.T) {
			var stdout bytes.Buffer
			rt := quickCaptureRuntime(t.TempDir(), &stdout)
			rt.Env = func(string) string {
				t.Fatal("invalid show ID inspected environment")
				return ""
			}
			err := Run(context.Background(), []string{"show", id}, rt, "dev")
			if err == nil || !strings.Contains(err.Error(), "invalid record ID") {
				t.Fatalf("Run() error = %v, want invalid-record-ID error", err)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestShowRejectsIncompatibleSchemaWithoutOutput(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createArchivedMigrationDatabase(t, dataDirectory)
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"show", "opaque-id"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	)
	if !errors.Is(err, storage.ErrIncompatibleSchema) {
		t.Fatalf("Run() error = %v, want ErrIncompatibleSchema", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}
