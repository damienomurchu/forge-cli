package cli

import (
	"errors"
	"testing"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestParseShowAcceptsJSONBeforeOrAfterID(t *testing.T) {
	for _, args := range [][]string{{"--json", "capture-id"}, {"capture-id", "--json"}} {
		id, jsonOutput, err := parseShow(args)
		if err != nil {
			t.Fatalf("parseShow(%q) error = %v", args, err)
		}
		if id != "capture-id" || !jsonOutput {
			t.Errorf("parseShow(%q) = %q/%t", args, id, jsonOutput)
		}
	}
}

func TestParseShowRejectsDuplicateJSON(t *testing.T) {
	_, _, err := parseShow([]string{"--json", "capture-id", "--json"})
	var usage *UsageError
	if !errors.As(err, &usage) || err.Error() != "--json may only be specified once" {
		t.Fatalf("error = %T %v, want duplicate --json usage error", err, err)
	}
}

func TestParseShowRejectsInvalidSyntaxAndIDs(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		usage bool
	}{
		{name: "missing ID", usage: true},
		{name: "extra ID", args: []string{"one", "two"}, usage: true},
		{name: "JSON value", args: []string{"capture-id", "--json=true"}, usage: true},
		{name: "blank ID", args: []string{"  "}},
		{name: "control ID", args: []string{"capture\n-id"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseShow(tt.args)
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
