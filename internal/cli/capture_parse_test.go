package cli

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

func TestParseCaptureRequestFinalizesEveryQuickType(t *testing.T) {
	for _, captureType := range domain.CaptureTypes() {
		t.Run(captureType.String(), func(t *testing.T) {
			got, err := parseCaptureRequest([]string{
				"--quick", "--type", captureType.String(), "  description  ", "--json",
			})
			if err != nil {
				t.Fatalf("parseCaptureRequest() error = %v", err)
			}
			if !got.quick || !got.json || got.description != "description" || got.proposed == nil {
				t.Fatalf("request = %#v", got)
			}
			if got.proposed.Type != captureType || got.proposed.Description != "description" {
				t.Errorf("proposal = %#v", got.proposed)
			}
			if err := got.proposed.Validate(); err != nil {
				t.Errorf("proposal validation error = %v", err)
			}
		})
	}
}

func TestParseCaptureRequestAppliesAndOverridesFrictionDefaults(t *testing.T) {
	defaults, err := parseCaptureRequest([]string{"--quick", "--type=friction", "problem"})
	if err != nil {
		t.Fatalf("default parse error = %v", err)
	}
	details := defaults.proposed.Details.Friction
	if details.Frequency != domain.FrequencyUnknown || details.Impact != domain.ImpactUnknown ||
		details.Category != domain.CategoryOther || details.Project != nil || details.CurrentWorkaround != nil {
		t.Errorf("default friction details = %#v", details)
	}

	overrides, err := parseCaptureRequest([]string{
		"--category=verification", "--quick", "--impact", "high",
		"--project", "  forge  ", "--type", "friction",
		"--frequency=weekly", "--current-workaround", "  Use a checklist  ", "problem",
	})
	if err != nil {
		t.Fatalf("override parse error = %v", err)
	}
	details = overrides.proposed.Details.Friction
	if details.Project == nil || *details.Project != "forge" ||
		details.Frequency != domain.FrequencyWeekly || details.Impact != domain.ImpactHigh ||
		details.Category != domain.CategoryVerification || details.CurrentWorkaround == nil ||
		*details.CurrentWorkaround != "Use a checklist" {
		t.Errorf("overridden friction details = %#v", details)
	}
}

func TestParseCaptureRequestNormalizesEmptyOptionalFrictionText(t *testing.T) {
	got, err := parseCaptureRequest([]string{
		"--quick", "--type", "friction", "--project=", "--current-workaround=", "problem",
	})
	if err != nil {
		t.Fatalf("parseCaptureRequest() error = %v", err)
	}
	if got.proposed.Details.Friction.Project != nil || got.proposed.Details.Friction.CurrentWorkaround != nil {
		t.Errorf("optional friction text = %#v", got.proposed.Details.Friction)
	}
}

func TestParseCaptureRequestInteractiveIntent(t *testing.T) {
	got, err := parseCaptureRequest([]string{"--json", "  choose later  "})
	if err != nil {
		t.Fatalf("parseCaptureRequest() error = %v", err)
	}
	if got.quick || !got.json || got.description != "choose later" || got.proposed != nil {
		t.Errorf("interactive request = %#v", got)
	}
}

func TestParseCaptureRequestOptionTerminator(t *testing.T) {
	got, err := parseCaptureRequest([]string{"--quick", "--type", "action", "--", "- investigate"})
	if err != nil {
		t.Fatalf("parseCaptureRequest() error = %v", err)
	}
	if got.proposed == nil || got.proposed.Description != "- investigate" {
		t.Errorf("proposal = %#v", got.proposed)
	}
}

func TestParseCaptureRequestRejectsModeAndTypeConflicts(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "quick missing type", args: []string{"--quick", "description"}, want: "--quick requires --type"},
		{name: "interactive type", args: []string{"--type", "action", "description"}, want: "--type requires --quick"},
		{name: "interactive friction option", args: []string{"--project", "forge", "description"}, want: "friction options require --quick --type friction"},
		{name: "action friction option", args: []string{"--quick", "--type", "action", "--impact", "high", "description"}, want: "friction options require --type friction"},
		{name: "follow-up friction option", args: []string{"--quick", "--type", "follow-up", "--project", "forge", "description"}, want: "friction options require --type friction"},
		{name: "decision friction option", args: []string{"--quick", "--type", "decision", "--category", "other", "description"}, want: "friction options require --type friction"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCaptureRequest(tt.args)
			var usage *UsageError
			if !errors.As(err, &usage) || err.Error() != tt.want {
				t.Fatalf("error = %T %v, want usage %q", err, err, tt.want)
			}
		})
	}
}

func TestParseCaptureRequestRejectsDuplicateFlags(t *testing.T) {
	flags := []struct {
		flag, value string
	}{
		{flag: "--quick"},
		{flag: "--json"},
		{flag: "--type", value: "action"},
		{flag: "--project", value: "forge"},
		{flag: "--frequency", value: "weekly"},
		{flag: "--impact", value: "high"},
		{flag: "--category", value: "other"},
		{flag: "--current-workaround", value: "manual"},
	}
	for _, tt := range flags {
		t.Run(tt.flag, func(t *testing.T) {
			args := []string{tt.flag}
			if tt.value != "" {
				args = append(args, tt.value)
			}
			args = append(args, tt.flag)
			if tt.value != "" {
				args = append(args, tt.value)
			}
			args = append(args, "description")
			_, err := parseCaptureRequest(args)
			if err == nil || err.Error() != tt.flag+" may only be specified once" {
				t.Fatalf("error = %v, want duplicate %s", err, tt.flag)
			}
		})
	}
}

func TestParseCaptureRequestRejectsUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing description", args: []string{"--quick", "--type", "action"}, want: "capture requires a description"},
		{name: "extra description", args: []string{"--quick", "--type", "action", "one", "two"}, want: `unexpected argument "two"`},
		{name: "unknown flag", args: []string{"--quick", "--type", "action", "--unknown", "description"}, want: `unknown argument "--unknown"`},
	}
	valueFlags := []string{"--type", "--frequency", "--impact", "--category"}
	for _, flag := range valueFlags {
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
			}{name: flag + " empty", args: []string{flag + "=", "description"}, want: flag + " requires a value"},
		)
	}
	for _, flag := range []string{"--project", "--current-workaround"} {
		tests = append(tests, struct {
			name string
			args []string
			want string
		}{name: flag + " missing", args: []string{flag}, want: flag + " requires a value"})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCaptureRequest(tt.args)
			var usage *UsageError
			if !errors.As(err, &usage) || err.Error() != tt.want {
				t.Fatalf("error = %T %v, want usage %q", err, err, tt.want)
			}
		})
	}
}

func TestParseCaptureRequestRejectsInvalidDomainValues(t *testing.T) {
	tests := []struct {
		name, flag, value, field string
	}{
		{name: "type", flag: "--type", value: "thought", field: "capture type"},
		{name: "frequency", flag: "--frequency", value: "often", field: "frequency"},
		{name: "impact", flag: "--impact", value: "severe", field: "impact"},
		{name: "category", flag: "--category", value: "process", field: "category"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"--quick", "--type", "friction", tt.flag, tt.value, "description"}
			if tt.flag == "--type" {
				args = []string{"--quick", tt.flag, tt.value, "description"}
			}
			_, err := parseCaptureRequest(args)
			var invalid *domain.InvalidValueError
			if !errors.As(err, &invalid) || invalid.Field != tt.field || invalid.Value != tt.value {
				t.Fatalf("error = %#v, want invalid %s %q", invalid, tt.field, tt.value)
			}
		})
	}
	_, err := parseCaptureRequest([]string{"--quick", "--type", "action", " \t "})
	var invalid *domain.InvalidValueError
	if !errors.As(err, &invalid) || invalid.Field != "description" {
		t.Fatalf("blank description error = %v, want invalid description", err)
	}
}

func TestParseCaptureRequestIsPure(t *testing.T) {
	args := []string{"--quick", "--type", "action", "description"}
	first, err := parseCaptureRequest(args)
	if err != nil {
		t.Fatal(err)
	}
	second, err := parseCaptureRequest(args)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || strings.Join(args, "|") != "--quick|--type|action|description" {
		t.Errorf("parser mutated input or returned nondeterministic results")
	}
}
