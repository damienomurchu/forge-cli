package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/storage"
)

const (
	reviewCapturedID  = "frc_01010101010101010101010101010101"
	reviewCandidateID = "frc_02020202020202020202020202020202"
)

func TestReviewHelpIsDataFree(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "review-help.golden"))
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"review", "-h"},
		{"review", "--help"},
		{"review", "friction", "--unknown", "--help"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			var stdout bytes.Buffer
			rt := Runtime{
				Stdout: &stdout,
				Env: func(string) string {
					t.Fatal("review help inspected the environment")
					return ""
				},
			}
			if err := Run(context.Background(), args, rt, "dev"); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != string(want) {
				t.Errorf("review help mismatch\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestReviewWritesActionableFrictionNewestFirst(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createReviewRecords(t, dataDirectory)
	var stdout bytes.Buffer
	if err := Run(
		context.Background(),
		[]string{"review"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := reviewCandidateID + "  candidate  unknown  unknown  other  Candidate friction\n" +
		reviewCapturedID + "  captured  unknown  unknown  other  Captured friction\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestReviewJSONWritesCompleteActionableFriction(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createReviewRecords(t, dataDirectory)
	var stdout bytes.Buffer
	if err := Run(
		context.Background(),
		[]string{"review", "--json"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "[{\"id\":\"frc_02020202020202020202020202020202\",\"type\":\"friction\",\"description\":\"Candidate friction\",\"project\":null,\"status\":\"candidate\",\"details\":{\"frequency\":\"unknown\",\"impact\":\"unknown\",\"category\":\"other\",\"current_workaround\":null},\"created_at\":\"2026-08-25T14:00:00.000000Z\",\"updated_at\":\"2026-08-25T14:00:00.000000Z\"},{\"id\":\"frc_01010101010101010101010101010101\",\"type\":\"friction\",\"description\":\"Captured friction\",\"project\":null,\"status\":\"captured\",\"details\":{\"frequency\":\"unknown\",\"impact\":\"unknown\",\"category\":\"other\",\"current_workaround\":null},\"created_at\":\"2026-08-25T12:00:00.000000Z\",\"updated_at\":\"2026-08-25T12:00:00.000000Z\"}]\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestReviewEmptyExistingStorageWritesHumanEmptyState(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	if err := Run(
		context.Background(),
		[]string{"capture", "--quick", "Excluded capture"},
		quickCaptureRuntime(dataDirectory, &bytes.Buffer{}),
		"dev",
	); err != nil {
		t.Fatalf("create capture error = %v", err)
	}
	var stdout bytes.Buffer
	if err := Run(
		context.Background(),
		[]string{"review"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "No actionable friction.\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestReviewMissingStorageWritesEmptyJSONArrayWithoutCreating(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	if err := Run(
		context.Background(),
		[]string{"review", "--json"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "[]\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if _, statErr := os.Stat(dataDirectory); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("data directory state error = %v, want not exist", statErr)
	}
}

func TestReviewUsageErrorsDoNotInspectEnvironment(t *testing.T) {
	for _, args := range [][]string{
		{"review", "friction"},
		{"review", "extra"},
		{"review", "--unknown"},
		{"review", "--json=value"},
		{"review", "--json", "--json"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			rt := quickCaptureRuntime(t.TempDir(), &bytes.Buffer{})
			rt.Env = func(string) string {
				t.Fatal("review usage error inspected environment")
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

func TestReviewMalformedIncludedRecordWritesNothing(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createReviewFriction(t, dataDirectory, 1, 12, "captured", "Malformed friction")
	session, err := storage.OpenExisting(
		context.Background(),
		filepath.Join(dataDirectory, "forge.db"),
		os.Geteuid(),
		storage.DatabaseReadWrite,
	)
	if err != nil {
		t.Fatalf("OpenExisting() error = %v", err)
	}
	if _, err := session.Database().Exec(
		`UPDATE records SET created_at = 'malformed' WHERE id = ?`,
		reviewCapturedID,
	); err != nil {
		_ = session.Close()
		t.Fatalf("corrupt record error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Session.Close() error = %v", err)
	}

	var stdout bytes.Buffer
	err = Run(
		context.Background(),
		[]string{"review"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	)
	if err == nil || !strings.Contains(err.Error(), "invalid timestamp") {
		t.Fatalf("Run() error = %v, want malformed-record error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestReviewRejectsIncompatibleSchemaWithoutOutput(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createArchivedMigrationDatabase(t, dataDirectory)
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"review"},
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

func createReviewRecords(t *testing.T, dataDirectory string) {
	t.Helper()
	createReviewFriction(t, dataDirectory, 1, 12, "captured", "Captured friction")
	createReviewFriction(t, dataDirectory, 2, 14, "candidate", "Candidate friction")
	createReviewFriction(t, dataDirectory, 3, 15, "automated", "Automated friction")
	rt := quickCaptureRuntime(dataDirectory, &bytes.Buffer{})
	rt.Now = func() time.Time {
		return time.Date(2026, time.August, 25, 16, 0, 0, 0, time.UTC)
	}
	rt.Random = bytes.NewReader(bytes.Repeat([]byte{4}, 16))
	if err := Run(context.Background(), []string{"capture", "--quick", "Excluded capture"}, rt, "dev"); err != nil {
		t.Fatalf("create excluded capture error = %v", err)
	}
}

func createReviewFriction(t *testing.T, dataDirectory string, idByte byte, hour int, status, description string) {
	t.Helper()
	rt := quickCaptureRuntime(dataDirectory, &bytes.Buffer{})
	rt.Now = func() time.Time {
		return time.Date(2026, time.August, 25, hour, 0, 0, 0, time.UTC)
	}
	rt.Random = bytes.NewReader(bytes.Repeat([]byte{idByte}, 16))
	if err := Run(context.Background(), []string{"friction", "--quick", description}, rt, "dev"); err != nil {
		t.Fatalf("create friction error = %v", err)
	}
	if status != "captured" {
		id := "frc_" + strings.Repeat(fmt.Sprintf("%02x", idByte), 16)
		setListRecordStatus(t, dataDirectory, id, status)
	}
}
