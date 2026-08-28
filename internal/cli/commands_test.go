//go:build linux || darwin

package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/prompt"
	"github.com/damienomurchu/forge-cli/internal/storage"
)

func TestCommandHelpMatchesGoldenFiles(t *testing.T) {
	tests := []struct {
		name string
		args []string
		file string
	}{
		{name: "top level", args: nil, file: "help.golden"},
		{name: "capture", args: []string{"capture", "--help"}, file: "capture-help.golden"},
		{name: "completion", args: []string{"completion", "--help"}, file: "completion-help.golden"},
		{name: "delete", args: []string{"delete", "--help"}, file: "delete-help.golden"},
		{name: "list", args: []string{"list", "--help"}, file: "list-help.golden"},
		{name: "show", args: []string{"show", "--help"}, file: "show-help.golden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			var stdout bytes.Buffer
			rt := Runtime{Stdout: &stdout, Env: func(string) string {
				t.Fatal("help inspected environment")
				return ""
			}}
			if err := Run(context.Background(), tt.args, rt, "dev"); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if stdout.String() != string(want) {
				t.Errorf("help mismatch\ngot:\n%s\nwant:\n%s", stdout.String(), want)
			}
		})
	}
}

func TestRemovedCommandsAreUnknown(t *testing.T) {
	for _, command := range []string{"friction", "update", "review"} {
		t.Run(command, func(t *testing.T) {
			var stdout bytes.Buffer
			err := Run(context.Background(), []string{command}, Runtime{Stdout: &stdout}, "dev")
			var usage *UsageError
			if !errors.As(err, &usage) || usage.Argument != command || stdout.Len() != 0 {
				t.Fatalf("error/stdout = %T %v/%q", err, err, stdout.String())
			}
		})
	}
}

func TestVersionAndErrorPresentationRemainStable(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run(context.Background(), []string{"--version"}, Runtime{Stdout: &stdout}, "1.2.3"); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "forge 1.2.3\n" {
		t.Errorf("version output = %q", stdout.String())
	}

	for _, tt := range []struct {
		name string
		err  error
		code int
	}{
		{name: "usage", err: &UsageError{Argument: "old"}, code: 2},
		{name: "operational", err: errors.New("failed"), code: 1},
		{name: "interrupted", err: &InterruptedError{Message: "capture cancelled", Cause: prompt.ErrCancelled}, code: 130},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if code := WriteError(&stderr, tt.err); code != tt.code || stderr.Len() == 0 {
				t.Errorf("code/stderr = %d/%q, want %d/nonempty", code, stderr.String(), tt.code)
			}
		})
	}
}

func TestQuickCaptureListAndShowUseCaptureSchema(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	for index, captureType := range domain.CaptureTypes() {
		var stdout bytes.Buffer
		args := []string{"capture", "--quick", "--type", captureType.String()}
		if captureType == domain.CaptureTypeFriction {
			args = append(args, "--project", "forge", "--frequency", "weekly", "--impact", "high", "--category", "verification")
		}
		args = append(args, captureType.String()+" description")
		rt := commandRuntime(t, dataDirectory, &stdout, byte(index+1))
		rt.Prompt = func() Prompt { t.Fatal("quick capture constructed prompt"); return nil }
		if err := Run(context.Background(), args, rt, "dev"); err != nil {
			t.Fatalf("capture %s error = %v", captureType, err)
		}
		if !strings.HasPrefix(stdout.String(), "Created "+captureType.String()+" capture cap_") {
			t.Errorf("capture stdout = %q", stdout.String())
		}
	}

	var listJSON bytes.Buffer
	if err := Run(context.Background(), []string{"list", "--json"}, commandRuntime(t, dataDirectory, &listJSON, 20), "dev"); err != nil {
		t.Fatalf("list error = %v", err)
	}
	for _, captureType := range domain.CaptureTypes() {
		if !strings.Contains(listJSON.String(), `"capture_type":"`+captureType.String()+`"`) {
			t.Errorf("list JSON missing %s: %q", captureType, listJSON.String())
		}
	}

	var frictionList bytes.Buffer
	if err := Run(context.Background(), []string{"list", "--type", "friction", "--project", " forge "}, commandRuntime(t, dataDirectory, &frictionList, 21), "dev"); err != nil {
		t.Fatalf("filtered list error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(frictionList.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "  friction  friction description") {
		t.Errorf("filtered list = %q", frictionList.String())
	}
	id := strings.Fields(lines[0])[0]
	var shown bytes.Buffer
	if err := Run(context.Background(), []string{"show", "--json", id}, commandRuntime(t, dataDirectory, &shown, 22), "dev"); err != nil {
		t.Fatalf("show error = %v", err)
	}
	if !strings.Contains(shown.String(), `"capture_type":"friction"`) ||
		!strings.Contains(shown.String(), `"project":"forge"`) {
		t.Errorf("show JSON = %q", shown.String())
	}
}

func TestInteractiveCaptureDeclineDoesNotInspectEnvironmentOrMetadata(t *testing.T) {
	prompter := &scriptedCapturePrompt{selectValues: []string{"action"}, confirmed: false}
	var stdout, stderr bytes.Buffer
	rt := Runtime{
		Stdout: &stdout, Stderr: &stderr,
		IsTTY:  func() bool { return true },
		Prompt: func() Prompt { return prompter },
		Env:    func(string) string { t.Fatal("declined capture inspected environment"); return "" },
		Now:    func() time.Time { t.Fatal("declined capture generated timestamp"); return time.Time{} },
		Random: panicCaptureReader{},
	}
	if err := Run(context.Background(), []string{"capture", "decline this"}, rt, "dev"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Capture summary\nType: action\n") {
		t.Errorf("stdout/stderr = %q/%q", stdout.String(), stderr.String())
	}
}

func TestCaptureRejectsInvalidOrNonTerminalInputBeforeStorage(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		isTTY func() bool
	}{
		{name: "invalid quick description", args: []string{"capture", "--quick", "--type", "action", "  "}},
		{name: "non-terminal interactive", args: []string{"capture", "description"}, isTTY: func() bool { return false }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := Runtime{
				Stdout: io.Discard, Stderr: io.Discard, IsTTY: tt.isTTY,
				Env:    func(string) string { t.Fatal("rejected capture inspected environment"); return "" },
				Prompt: func() Prompt { t.Fatal("rejected capture constructed prompt"); return nil },
			}
			if err := Run(context.Background(), tt.args, rt, "dev"); err == nil {
				t.Fatal("Run() error = nil, want rejection")
			}
		})
	}
}

func TestInteractiveCaptureCancellationDoesNotInspectStorage(t *testing.T) {
	prompter := &scriptedCapturePrompt{failAt: 1, failErr: prompt.ErrCancelled}
	rt := Runtime{
		Stdout: io.Discard, Stderr: io.Discard,
		IsTTY: func() bool { return true }, Prompt: func() Prompt { return prompter },
		Env: func(string) string { t.Fatal("cancelled capture inspected environment"); return "" },
	}
	err := Run(context.Background(), []string{"capture", "cancel this"}, rt, "dev")
	var interrupted *InterruptedError
	if !errors.As(err, &interrupted) || !errors.Is(err, prompt.ErrCancelled) {
		t.Fatalf("error = %T %v, want interrupted cancellation", err, err)
	}
}

func TestInteractiveFrictionPersistsAfterConfirmation(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	prompter := &scriptedCapturePrompt{
		selectValues: []string{"friction", "weekly", "high", "verification"},
		textValues:   []string{"forge", "manual check"}, confirmed: true,
	}
	var stdout, stderr bytes.Buffer
	rt := commandRuntime(t, dataDirectory, &stdout, 30)
	rt.Stderr = &stderr
	rt.IsTTY = func() bool { return true }
	rt.Prompt = func() Prompt { return prompter }
	if err := Run(context.Background(), []string{"capture", "interactive friction"}, rt, "dev"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.HasPrefix(stdout.String(), "Created friction capture cap_") ||
		!strings.Contains(stderr.String(), "Current workaround: manual check\n") {
		t.Errorf("stdout/stderr = %q/%q", stdout.String(), stderr.String())
	}
}

func TestListMissingStorageDoesNotCreateIt(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		dataDirectory := filepath.Join(t.TempDir(), "missing")
		args := []string{"list"}
		want := ""
		if jsonOutput {
			args, want = append(args, "--json"), "[]\n"
		}
		var stdout bytes.Buffer
		if err := Run(context.Background(), args, commandRuntime(t, dataDirectory, &stdout, 40), "dev"); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if stdout.String() != want {
			t.Errorf("stdout = %q, want %q", stdout.String(), want)
		}
		if _, err := os.Lstat(dataDirectory); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("missing storage state error = %v", err)
		}
	}
}

func TestShowMissingCapturePreservesNotFoundPresentationBoundary(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	var discarded bytes.Buffer
	if err := Run(context.Background(), []string{"capture", "--quick", "--type", "action", "stored"}, commandRuntime(t, dataDirectory, &discarded, 50), "dev"); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"show", "missing"}, commandRuntime(t, dataDirectory, &stdout, 51), "dev")
	if err == nil || !strings.Contains(err.Error(), `record "missing" not found`) || stdout.Len() != 0 {
		t.Fatalf("error/stdout = %v/%q", err, stdout.String())
	}
}

func TestSchemaOneDatabaseRequiresMigrationForReadCommands(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	createSchemaOneDatabase(t, dataDirectory)
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"list"}, commandRuntime(t, dataDirectory, &stdout, 60), "dev")
	if !errors.Is(err, storage.ErrMigrationRequired) || stdout.Len() != 0 {
		t.Fatalf("error/stdout = %v/%q, want migration required", err, stdout.String())
	}
}

func commandRuntime(t *testing.T, dataDirectory string, stdout *bytes.Buffer, randomByte byte) Runtime {
	t.Helper()
	return Runtime{
		Stdout: stdout,
		Stderr: &bytes.Buffer{},
		Env: func(name string) string {
			if name == "FORGE_DATA_DIR" {
				return dataDirectory
			}
			return ""
		},
		Now:    func() time.Time { return time.Date(2026, time.August, 26, 14, 0, 0, 0, time.UTC) },
		Random: bytes.NewReader(bytes.Repeat([]byte{randomByte}, 16)),
		GOOS:   "linux",
		EUID:   os.Geteuid(),
	}
}

func createSchemaOneDatabase(t *testing.T, dataDirectory string) {
	t.Helper()
	directory, err := storage.PrepareDataDirectory(dataDirectory, os.Geteuid())
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	database, err := storage.OpenDatabaseFile(directory, storage.DatabaseCreate, os.Geteuid())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	db, err := storage.OpenSQLite(context.Background(), directory, database, storage.DatabaseCreate)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query, err := os.ReadFile(filepath.Join("..", "..", "migrations", "001_initial.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(query)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations(version, name, applied_at)
		VALUES (1, '001_initial.sql', '2026-08-25T12:00:00.000Z')`); err != nil {
		t.Fatal(err)
	}
}
