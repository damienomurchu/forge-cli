//go:build linux || darwin

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
)

func TestParseDeleteAcceptsOpaqueID(t *testing.T) {
	id, err := parseDelete([]string{"legacy-opaque-id"})
	if err != nil || id != "legacy-opaque-id" {
		t.Fatalf("parseDelete() = %q/%v", id, err)
	}
}

func TestParseDeleteRejectsInvalidSyntaxAndIDs(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		usage bool
	}{
		{name: "missing ID", usage: true},
		{name: "extra ID", args: []string{"one", "two"}, usage: true},
		{name: "unknown flag", args: []string{"--force"}, usage: true},
		{name: "blank ID", args: []string{"  "}},
		{name: "padded ID", args: []string{" capture-id"}},
		{name: "control ID", args: []string{"capture\n-id"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDelete(tt.args)
			if tt.usage {
				var usage *UsageError
				if !errors.As(err, &usage) {
					t.Fatalf("error = %T %v, want UsageError", err, err)
				}
				return
			}
			var invalid *domain.InvalidValueError
			if !errors.As(err, &invalid) || invalid.Field != "record ID" {
				t.Fatalf("error = %T %v, want invalid record ID", err, err)
			}
		})
	}
}

func TestDeleteCommandRemovesCapture(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "forge-data")
	var created bytes.Buffer
	if err := Run(context.Background(), []string{"capture", "--quick", "--type", "action", "remove me"}, commandRuntime(t, dataDirectory, &created, 70), "dev"); err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSpace(strings.TrimPrefix(created.String(), "Created action capture "))
	var deleted bytes.Buffer
	if err := Run(context.Background(), []string{"delete", id}, commandRuntime(t, dataDirectory, &deleted, 71), "dev"); err != nil {
		t.Fatalf("delete error = %v", err)
	}
	if deleted.String() != "Deleted capture "+id+"\n" {
		t.Errorf("delete stdout = %q", deleted.String())
	}
	var shown bytes.Buffer
	err := Run(context.Background(), []string{"show", id}, commandRuntime(t, dataDirectory, &shown, 72), "dev")
	if err == nil || !strings.Contains(err.Error(), "not found") || shown.Len() != 0 {
		t.Fatalf("show after delete error/stdout = %v/%q", err, shown.String())
	}
}

func TestDeleteMissingStorageDoesNotCreateIt(t *testing.T) {
	dataDirectory := filepath.Join(t.TempDir(), "missing")
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"delete", "missing"}, commandRuntime(t, dataDirectory, &stdout, 73), "dev")
	if err == nil || !strings.Contains(err.Error(), `record "missing" not found`) || stdout.Len() != 0 {
		t.Fatalf("error/stdout = %v/%q", err, stdout.String())
	}
	if _, err := os.Lstat(dataDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing storage state error = %v", err)
	}
}

type recordingCaptureDeleter struct {
	ids []domain.ID
	err error
}

func (d *recordingCaptureDeleter) DeleteByID(_ context.Context, id domain.ID) error {
	d.ids = append(d.ids, id)
	return d.err
}

func TestExecuteDeletePreservesRepositoryErrorsAndOutputBoundary(t *testing.T) {
	deleter := &recordingCaptureDeleter{err: repository.ErrRecordNotFound}
	var stdout bytes.Buffer
	err := executeDelete(context.Background(), "opaque", deleter, &stdout)
	if !errors.Is(err, repository.ErrRecordNotFound) || stdout.Len() != 0 || len(deleter.ids) != 1 {
		t.Fatalf("error/stdout/ids = %v/%q/%v", err, stdout.String(), deleter.ids)
	}
}
