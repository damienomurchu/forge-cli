package domain

import (
	"errors"
	"testing"
)

func TestNormalizeDescription(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "unchanged", input: "Measure startup time", want: "Measure startup time"},
		{name: "surrounding ASCII whitespace", input: " \tMeasure startup time\r\n", want: "Measure startup time"},
		{name: "surrounding Unicode whitespace", input: "\u00a0Measure startup time\u2003", want: "Measure startup time"},
		{name: "internal whitespace preserved", input: "  Measure\tstartup\n\ntime  ", want: "Measure\tstartup\n\ntime"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeDescription(tt.input)
			if err != nil {
				t.Fatalf("NormalizeDescription(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("NormalizeDescription(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeDescriptionRejectsBlankText(t *testing.T) {
	for _, input := range []string{"", " \t\r\n", "\u00a0\u2003"} {
		t.Run(input, func(t *testing.T) {
			got, err := NormalizeDescription(input)
			var invalid *InvalidValueError
			if !errors.As(err, &invalid) {
				t.Fatalf("NormalizeDescription(%q) error = %T %v, want *InvalidValueError", input, err, err)
			}
			if invalid.Field != "description" || invalid.Value != input {
				t.Errorf("error = %#v, want description field and original value %q", invalid, input)
			}
			if got != "" {
				t.Errorf("NormalizeDescription(%q) = %q, want empty", input, got)
			}
		})
	}
}

func TestNormalizeOptionalText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *string
	}{
		{name: "empty", input: "", want: nil},
		{name: "ASCII whitespace", input: " \t\r\n", want: nil},
		{name: "Unicode whitespace", input: "\u00a0\u2003", want: nil},
		{name: "unchanged", input: "forge", want: stringPointer("forge")},
		{name: "surrounding whitespace", input: " \tforge\u2003", want: stringPointer("forge")},
		{name: "internal whitespace preserved", input: "  current\tworkaround\ntext  ", want: stringPointer("current\tworkaround\ntext")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeOptionalText(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("NormalizeOptionalText(%q) = %q, want nil", tt.input, *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("NormalizeOptionalText(%q) = nil, want %q", tt.input, *tt.want)
			}
			if *got != *tt.want {
				t.Errorf("NormalizeOptionalText(%q) = %q, want %q", tt.input, *got, *tt.want)
			}
		})
	}
}

func stringPointer(value string) *string {
	return &value
}
