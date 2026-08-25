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

	"github.com/damienomurchu/forge-cli/internal/storage"
)

func TestListHelp(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "list-help.golden"))
	if err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"list", "-h"},
		{"list", "--help"},
		{"list", "--unknown", "--help"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			var stdout bytes.Buffer
			rt := Runtime{
				Stdout: &stdout,
				Env: func(string) string {
					t.Fatal("list help inspected the environment")
					return ""
				},
			}
			if err := Run(context.Background(), args, rt, "dev"); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != string(want) {
				t.Errorf("list help mismatch\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestListMissingStorageIsSuccessfulAndEmpty(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"list"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if _, statErr := os.Stat(dataDirectory); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("data directory state error = %v, want not exist", statErr)
	}
}

func TestListJSONMissingStorageWritesEmptyArray(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"list", "--json"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "[]\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if _, statErr := os.Stat(dataDirectory); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("data directory state error = %v, want not exist", statErr)
	}
}

func TestListWritesRecordsNewestFirst(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createListRecords(t, dataDirectory)

	var stdout bytes.Buffer
	if err := Run(
		context.Background(),
		[]string{"list"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "frc_00000000000000000000000000000000  friction  captured  Newer friction\n" +
		"cap_00000000000000000000000000000000  capture  captured  Older capture\n"
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestListJSONWritesCompleteRecordsNewestFirst(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createListRecords(t, dataDirectory)

	var stdout bytes.Buffer
	if err := Run(
		context.Background(),
		[]string{"list", "--json"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := "[{\"id\":\"frc_00000000000000000000000000000000\",\"type\":\"friction\",\"description\":\"Newer friction\",\"project\":null,\"status\":\"captured\",\"details\":{\"frequency\":\"unknown\",\"impact\":\"unknown\",\"category\":\"other\",\"current_workaround\":null},\"created_at\":\"2026-08-25T12:00:00.000000Z\",\"updated_at\":\"2026-08-25T12:00:00.000000Z\"},{\"id\":\"cap_00000000000000000000000000000000\",\"type\":\"capture\",\"description\":\"Older capture\",\"project\":null,\"status\":\"captured\",\"details\":{\"kind\":\"thought\",\"tags\":[]},\"created_at\":\"2026-08-25T11:00:00.000000Z\",\"updated_at\":\"2026-08-25T11:00:00.000000Z\"}]\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestListFiltersByRecordType(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createListRecords(t, dataDirectory)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "spaced human",
			args: []string{"list", "--type", "capture"},
			want: "cap_00000000000000000000000000000000  capture  captured  Older capture\n",
		},
		{
			name: "equals JSON",
			args: []string{"list", "--type=friction", "--json"},
			want: "[{\"id\":\"frc_00000000000000000000000000000000\",\"type\":\"friction\",\"description\":\"Newer friction\",\"project\":null,\"status\":\"captured\",\"details\":{\"frequency\":\"unknown\",\"impact\":\"unknown\",\"category\":\"other\",\"current_workaround\":null},\"created_at\":\"2026-08-25T12:00:00.000000Z\",\"updated_at\":\"2026-08-25T12:00:00.000000Z\"}]\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := Run(
				context.Background(),
				tt.args,
				quickCaptureRuntime(dataDirectory, &stdout),
				"dev",
			); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListTypeFilterWithNoMatchesWritesEmptyJSONArray(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	if err := Run(
		context.Background(),
		[]string{"capture", "--quick", "Only capture"},
		quickCaptureRuntime(dataDirectory, &bytes.Buffer{}),
		"dev",
	); err != nil {
		t.Fatalf("create capture error = %v", err)
	}

	var stdout bytes.Buffer
	if err := Run(
		context.Background(),
		[]string{"list", "--type", "friction", "--json"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "[]\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestListFiltersByNormalizedProjectAndComposesWithType(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	for _, args := range [][]string{
		{"capture", "--quick", "--project", "forge", "Forge capture"},
		{"friction", "--quick", "--project", "forge", "Forge friction"},
	} {
		if err := Run(
			context.Background(),
			args,
			quickCaptureRuntime(dataDirectory, &bytes.Buffer{}),
			"dev",
		); err != nil {
			t.Fatalf("create record error = %v", err)
		}
	}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "spaced value is normalized",
			args: []string{"list", "--project", "  forge  "},
			want: "frc_00000000000000000000000000000000  friction  captured  Forge friction\n" +
				"cap_00000000000000000000000000000000  capture  captured  Forge capture\n",
		},
		{
			name: "equals value composes with type",
			args: []string{"list", "--project=forge", "--type", "friction"},
			want: "frc_00000000000000000000000000000000  friction  captured  Forge friction\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := Run(
				context.Background(),
				tt.args,
				quickCaptureRuntime(dataDirectory, &stdout),
				"dev",
			); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListProjectFilterWithNoMatchesWritesEmptyJSONArray(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	if err := Run(
		context.Background(),
		[]string{"capture", "--quick", "--project", "forge", "Forge capture"},
		quickCaptureRuntime(dataDirectory, &bytes.Buffer{}),
		"dev",
	); err != nil {
		t.Fatalf("create capture error = %v", err)
	}

	var stdout bytes.Buffer
	if err := Run(
		context.Background(),
		[]string{"list", "--project", "other", "--json"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := stdout.String(), "[]\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestListFiltersByStatusAndComposesWithOtherFilters(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	for _, args := range [][]string{
		{"capture", "--quick", "--project", "forge", "Captured forge capture"},
		{"friction", "--quick", "--project", "forge", "Reviewing forge friction"},
	} {
		if err := Run(
			context.Background(),
			args,
			quickCaptureRuntime(dataDirectory, &bytes.Buffer{}),
			"dev",
		); err != nil {
			t.Fatalf("create record error = %v", err)
		}
	}
	setListRecordStatus(t, dataDirectory, "frc_00000000000000000000000000000000", "reviewing")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "spaced human",
			args: []string{"list", "--status", "captured"},
			want: "cap_00000000000000000000000000000000  capture  captured  Captured forge capture\n",
		},
		{
			name: "equals JSON composes with type and project",
			args: []string{"list", "--status=reviewing", "--type", "friction", "--project", "forge", "--json"},
			want: "[{\"id\":\"frc_00000000000000000000000000000000\",\"type\":\"friction\",\"description\":\"Reviewing forge friction\",\"project\":\"forge\",\"status\":\"reviewing\",\"details\":{\"frequency\":\"unknown\",\"impact\":\"unknown\",\"category\":\"other\",\"current_workaround\":null},\"created_at\":\"2026-08-25T12:00:00.000000Z\",\"updated_at\":\"2026-08-25T12:00:00.000000Z\"}]\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := Run(
				context.Background(),
				tt.args,
				quickCaptureRuntime(dataDirectory, &stdout),
				"dev",
			); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListUsageErrorsDoNotInspectEnvironment(t *testing.T) {
	for _, args := range [][]string{
		{"list", "extra"},
		{"list", "--unknown"},
		{"list", "--type"},
		{"list", "--type="},
		{"list", "--project"},
		{"list", "--project="},
		{"list", "--status"},
		{"list", "--status="},
	} {
		t.Run(args[1], func(t *testing.T) {
			rt := quickCaptureRuntime(t.TempDir(), &bytes.Buffer{})
			rt.Env = func(string) string {
				t.Fatal("list usage error inspected environment")
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

func TestListEmptyProjectDoesNotInspectEnvironment(t *testing.T) {
	for _, args := range [][]string{
		{"list", "--project", "  \t"},
		{"list", "--project=  \t"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			var stdout bytes.Buffer
			rt := quickCaptureRuntime(t.TempDir(), &stdout)
			rt.Env = func(string) string {
				t.Fatal("invalid list project inspected environment")
				return ""
			}
			err := Run(context.Background(), args, rt, "dev")
			if err == nil || !strings.Contains(err.Error(), "invalid project") {
				t.Fatalf("Run() error = %v, want invalid-project error", err)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestListInvalidTypeDoesNotInspectEnvironment(t *testing.T) {
	var stdout bytes.Buffer
	rt := quickCaptureRuntime(t.TempDir(), &stdout)
	rt.Env = func(string) string {
		t.Fatal("invalid list type inspected environment")
		return ""
	}
	err := Run(context.Background(), []string{"list", "--type", "task"}, rt, "dev")
	if err == nil || err.Error() != `invalid record type "task"` {
		t.Fatalf("Run() error = %v, want invalid-record-type error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func TestListInvalidStatusDoesNotInspectEnvironment(t *testing.T) {
	var stdout bytes.Buffer
	rt := quickCaptureRuntime(t.TempDir(), &stdout)
	rt.Env = func(string) string {
		t.Fatal("invalid list status inspected environment")
		return ""
	}
	err := Run(context.Background(), []string{"list", "--status", "pending"}, rt, "dev")
	if err == nil || err.Error() != `invalid status "pending"` {
		t.Fatalf("Run() error = %v, want invalid-status error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func createListRecords(t *testing.T, dataDirectory string) {
	t.Helper()
	createRuntime := quickCaptureRuntime(dataDirectory, &bytes.Buffer{})
	createRuntime.Now = func() time.Time {
		return time.Date(2026, time.August, 25, 11, 0, 0, 0, time.UTC)
	}
	if err := Run(
		context.Background(),
		[]string{"capture", "--quick", "Older capture"},
		createRuntime,
		"dev",
	); err != nil {
		t.Fatalf("create capture error = %v", err)
	}
	createRuntime = quickCaptureRuntime(dataDirectory, &bytes.Buffer{})
	createRuntime.Now = func() time.Time {
		return time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	}
	if err := Run(
		context.Background(),
		[]string{"friction", "--quick", "Newer friction"},
		createRuntime,
		"dev",
	); err != nil {
		t.Fatalf("create friction error = %v", err)
	}
}

func setListRecordStatus(t *testing.T, dataDirectory, id, status string) {
	t.Helper()
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
		`UPDATE records SET status = ? WHERE id = ?`,
		status,
		id,
	); err != nil {
		_ = session.Close()
		t.Fatalf("update record status error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Session.Close() error = %v", err)
	}
}

func TestListRejectsIncompatibleSchemaWithoutOutput(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createArchivedMigrationDatabase(t, dataDirectory)
	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"list"},
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

func TestListRejectsMalformedRecordWithoutOutput(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	if err := Run(
		context.Background(),
		[]string{"capture", "--quick", "Valid capture"},
		quickCaptureRuntime(dataDirectory, &bytes.Buffer{}),
		"dev",
	); err != nil {
		t.Fatalf("create capture error = %v", err)
	}

	session, err := storage.OpenExisting(
		context.Background(),
		filepath.Join(dataDirectory, "forge.db"),
		os.Geteuid(),
		storage.DatabaseReadWrite,
	)
	if err != nil {
		t.Fatalf("OpenExisting() error = %v", err)
	}
	if _, err := session.Database().Exec(`UPDATE records SET description = ' malformed '`); err != nil {
		_ = session.Close()
		t.Fatalf("corrupt record error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Session.Close() error = %v", err)
	}

	var stdout bytes.Buffer
	err = Run(
		context.Background(),
		[]string{"list"},
		quickCaptureRuntime(dataDirectory, &stdout),
		"dev",
	)
	if err == nil || !strings.Contains(err.Error(), "invalid description") {
		t.Fatalf("Run() error = %v, want malformed-record error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
}

func createArchivedMigrationDatabase(t *testing.T, dataDirectory string) {
	t.Helper()
	if err := os.Mkdir(dataDirectory, 0o700); err != nil {
		t.Fatalf("create data directory error = %v", err)
	}
	directory, err := storage.OpenDataDirectory(dataDirectory, os.Geteuid())
	if err != nil {
		t.Fatalf("OpenDataDirectory() error = %v", err)
	}
	database, err := storage.OpenDatabaseFile(directory, storage.DatabaseCreate, os.Geteuid())
	if err != nil {
		_ = directory.Close()
		t.Fatalf("OpenDatabaseFile() error = %v", err)
	}
	db, err := storage.OpenSQLite(context.Background(), directory, database, storage.DatabaseCreate)
	if err != nil {
		_ = database.Close()
		_ = directory.Close()
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		_ = db.Close()
		_ = database.Close()
		_ = directory.Close()
		t.Fatalf("create archived schema error = %v", err)
	}
	if err := errors.Join(db.Close(), database.Close(), directory.Close()); err != nil {
		t.Fatalf("close archived database error = %v", err)
	}
}
