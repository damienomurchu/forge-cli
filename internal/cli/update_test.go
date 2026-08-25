package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/storage"
)

const updateCaptureID = "cap_00000000000000000000000000000000"

func TestUpdateHelpIsDataFree(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "update-help.golden"))
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"update", "-h"},
		{"update", "--help"},
		{"update", "extra", "--unknown", "--help"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			var stdout bytes.Buffer
			rt := Runtime{
				Stdout: &stdout,
				Env: func(string) string {
					t.Fatal("update help inspected the environment")
					return ""
				},
			}
			if err := Run(context.Background(), args, rt, "dev"); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != string(want) {
				t.Errorf("update help mismatch\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestUpdateChangesStatusAndTimestamp(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createUpdateCapture(t, dataDirectory)
	var stdout bytes.Buffer
	rt := quickCaptureRuntime(dataDirectory, &stdout)
	rt.Now = func() time.Time {
		return time.Date(2026, time.August, 26, 13, 14, 15, 123456789, time.UTC)
	}
	if err := Run(
		context.Background(),
		[]string{"update", updateCaptureID, "--status", "candidate"},
		rt,
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "Updated "+updateCaptureID+" to candidate\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	record := readQuickCapture(t, dataDirectory)
	if record.Status != domain.StatusCandidate {
		t.Errorf("status = %q, want candidate", record.Status)
	}
	if got, want := record.UpdatedAt.String(), "2026-08-26T13:14:15.123456Z"; got != want {
		t.Errorf("updated_at = %q, want %q", got, want)
	}
	if record.CreatedAt.String() != "2026-08-25T12:00:00.000000Z" {
		t.Errorf("created_at = %q, want unchanged", record.CreatedAt)
	}
}

func TestUpdateNoOpPreservesTimestampAndWritesJSON(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createUpdateCapture(t, dataDirectory)
	var stdout bytes.Buffer
	rt := quickCaptureRuntime(dataDirectory, &stdout)
	rt.Now = func() time.Time { return time.Time{} }
	if err := Run(
		context.Background(),
		[]string{"update", "--status=captured", updateCaptureID, "--json"},
		rt,
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "{\"id\":\"cap_00000000000000000000000000000000\",\"type\":\"capture\",\"description\":\"Update me\",\"project\":null,\"status\":\"captured\",\"details\":{\"kind\":\"thought\",\"tags\":[]},\"created_at\":\"2026-08-25T12:00:00.000000Z\",\"updated_at\":\"2026-08-25T12:00:00.000000Z\"}\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if got := readQuickCapture(t, dataDirectory).UpdatedAt.String(); got != "2026-08-25T12:00:00.000000Z" {
		t.Errorf("updated_at = %q, want unchanged", got)
	}
}

func TestUpdateMissingStorageIsNotFoundAndDoesNotCreate(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"update", updateCaptureID, "--status", "reviewing"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	)
	if err == nil || err.Error() != `record "`+updateCaptureID+`" not found` {
		t.Fatalf("Run() error = %v, want not-found error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if _, statErr := os.Stat(dataDirectory); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("data directory state error = %v, want not exist", statErr)
	}
}

func TestUpdateMissingRecordIsNotFound(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createUpdateCapture(t, dataDirectory)
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"update", "opaque-id", "--status", "reviewing"},
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

func TestUpdateUsageErrorsDoNotInspectEnvironment(t *testing.T) {
	for _, args := range [][]string{
		{"update"},
		{"update", updateCaptureID},
		{"update", "--status", "captured"},
		{"update", updateCaptureID, "--status"},
		{"update", updateCaptureID, "--status="},
		{"update", updateCaptureID, "--status", "captured", "extra"},
		{"update", updateCaptureID, "--unknown", "--status", "captured"},
		{"update", updateCaptureID, "--json", "--json", "--status", "captured"},
		{"update", updateCaptureID, "--status", "captured", "--status=reviewing"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			rt := quickCaptureRuntime(t.TempDir(), &bytes.Buffer{})
			rt.Env = func(string) string {
				t.Fatal("update usage error inspected environment")
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

func TestUpdateInvalidValuesDoNotInspectEnvironment(t *testing.T) {
	for _, args := range [][]string{
		{"update", " surrounding ", "--status", "captured"},
		{"update", "control\nID", "--status", "captured"},
		{"update", updateCaptureID, "--status", "pending"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			var stdout bytes.Buffer
			rt := quickCaptureRuntime(t.TempDir(), &stdout)
			rt.Env = func(string) string {
				t.Fatal("invalid update inspected environment")
				return ""
			}
			err := Run(context.Background(), args, rt, "dev")
			var invalid *domain.InvalidValueError
			if !errors.As(err, &invalid) {
				t.Fatalf("Run() error = %T %v, want *domain.InvalidValueError", err, err)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestUpdateRejectedTimestampLeavesRecordUnchanged(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createUpdateCapture(t, dataDirectory)
	var stdout bytes.Buffer
	rt := quickCaptureRuntime(dataDirectory, &stdout)
	rt.Now = func() time.Time {
		return time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	}
	err := Run(
		context.Background(),
		[]string{"update", updateCaptureID, "--status", "dismissed"},
		rt,
		"dev",
	)
	if err == nil || !strings.Contains(err.Error(), "invalid updated_at") {
		t.Fatalf("Run() error = %v, want timestamp validation error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	record := readQuickCapture(t, dataDirectory)
	if record.Status != domain.StatusCaptured || record.UpdatedAt != record.CreatedAt {
		t.Errorf("stored rejected update = %#v, want original", record)
	}
}

func TestUpdateRejectsIncompatibleSchemaWithoutOutput(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createArchivedMigrationDatabase(t, dataDirectory)
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"update", updateCaptureID, "--status", "reviewing"},
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

func createUpdateCapture(t *testing.T, dataDirectory string) {
	t.Helper()
	if err := Run(
		context.Background(),
		[]string{"capture", "--quick", "Update me"},
		quickCaptureRuntime(dataDirectory, &bytes.Buffer{}),
		"dev",
	); err != nil {
		t.Fatalf("create capture error = %v", err)
	}
}
