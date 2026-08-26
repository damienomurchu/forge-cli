package cli

import (
	"errors"
	"reflect"
	"testing"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/repository"
)

func TestParseListRequestDefaults(t *testing.T) {
	got, err := parseListRequest(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, listRequest{}) {
		t.Errorf("request = %#v, want zero request", got)
	}
}

func TestParseListRequestParsesSpacedAndEqualsFlags(t *testing.T) {
	friction, project, limit := domain.CaptureTypeFriction, "forge", 12
	want := listRequest{
		filters: repository.CaptureFilters{Type: &friction, Project: &project, Limit: &limit},
		json:    true,
	}
	for _, args := range [][]string{
		{"--type", "friction", "--project", "  forge  ", "--limit", "12", "--json"},
		{"--json", "--limit=12", "--project=  forge  ", "--type=friction"},
	} {
		got, err := parseListRequest(args)
		if err != nil {
			t.Fatalf("parseListRequest(%q) error = %v", args, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("parseListRequest(%q) = %#v, want %#v", args, got, want)
		}
	}
}

func TestParseListRequestAcceptsEveryCaptureType(t *testing.T) {
	for _, captureType := range domain.CaptureTypes() {
		got, err := parseListRequest([]string{"--type", captureType.String()})
		if err != nil {
			t.Fatalf("parse type %s error = %v", captureType, err)
		}
		if got.filters.Type == nil || *got.filters.Type != captureType {
			t.Errorf("type = %#v, want %s", got.filters.Type, captureType)
		}
	}
}

func TestParseListRequestRejectsDuplicates(t *testing.T) {
	for _, tt := range []struct{ flag, value string }{
		{flag: "--json"},
		{flag: "--type", value: "action"},
		{flag: "--project", value: "forge"},
		{flag: "--limit", value: "1"},
	} {
		t.Run(tt.flag, func(t *testing.T) {
			args := []string{tt.flag}
			if tt.value != "" {
				args = append(args, tt.value)
			}
			args = append(args, tt.flag)
			if tt.value != "" {
				args = append(args, tt.value)
			}
			_, err := parseListRequest(args)
			var usage *UsageError
			want := tt.flag + " may only be specified once"
			if !errors.As(err, &usage) || err.Error() != want {
				t.Fatalf("error = %T %v, want usage %q", err, err, want)
			}
		})
	}
}

func TestParseListRequestRejectsUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "status", args: []string{"--status", "captured"}, want: `unknown argument "--status"`},
		{name: "unknown", args: []string{"--unknown"}, want: `unknown argument "--unknown"`},
		{name: "positional", args: []string{"anything"}, want: `unknown argument "anything"`},
		{name: "json value", args: []string{"--json=true"}, want: `unknown argument "--json=true"`},
		{name: "limit followed by flag", args: []string{"--limit", "--json"}, want: "--limit requires a value"},
	}
	for _, flag := range []string{"--type", "--project", "--limit"} {
		tests = append(tests,
			struct {
				name string
				args []string
				want string
			}{name: flag + " missing", args: []string{flag}, want: flag + " requires a value"},
			struct {
				name string
				args []string
				want string
			}{name: flag + " empty", args: []string{flag + "="}, want: flag + " requires a value"},
		)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseListRequest(tt.args)
			var usage *UsageError
			if !errors.As(err, &usage) || err.Error() != tt.want {
				t.Fatalf("error = %T %v, want usage %q", err, err, tt.want)
			}
		})
	}
}

func TestParseListRequestRejectsInvalidValues(t *testing.T) {
	for _, tt := range []struct{ name, flag, value, field string }{
		{name: "type", flag: "--type", value: "capture", field: "capture type"},
		{name: "blank project", flag: "--project", value: "  ", field: "project"},
		{name: "zero limit", flag: "--limit", value: "0", field: "limit"},
		{name: "negative limit", flag: "--limit", value: "-1", field: "limit"},
		{name: "nonnumeric limit", flag: "--limit", value: "many", field: "limit"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseListRequest([]string{tt.flag, tt.value})
			var invalid *domain.InvalidValueError
			if !errors.As(err, &invalid) || invalid.Field != tt.field || invalid.Value != tt.value {
				t.Fatalf("error = %T %v, want invalid %s %q", err, err, tt.field, tt.value)
			}
		})
	}
}

func TestParseListRequestIsPure(t *testing.T) {
	args := []string{"--type", "decision", "--project", " forge ", "--limit", "3", "--json"}
	first, err := parseListRequest(args)
	if err != nil {
		t.Fatal(err)
	}
	second, err := parseListRequest(args)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("requests differ: %#v != %#v", first, second)
	}
}
