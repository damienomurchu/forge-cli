package prompt

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestConfirmUsesInjectedStreams(t *testing.T) {
	var output bytes.Buffer
	confirmed, err := New(strings.NewReader("n\r"), &output).
		Confirm(context.Background(), "Create capture?", true)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed {
		t.Fatal("Confirm() = true, want false")
	}
	if output.Len() == 0 {
		t.Fatal("Confirm() wrote no prompt output")
	}
}

func TestSelectNavigation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "j", input: "j\r", want: "idea"},
		{name: "down arrow", input: "\x1b[B\r", want: "idea"},
		{name: "j then k", input: "jk\r", want: "thought"},
		{name: "down then up", input: "\x1b[B\x1b[A\r", want: "thought"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			got, err := New(strings.NewReader(tt.input), &output).
				Select(context.Background(), "Kind", []string{"thought", "idea"}, "thought")
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("Select() = %q, want %q", got, tt.want)
			}
			if output.Len() == 0 {
				t.Fatal("Select() wrote no prompt output")
			}
		})
	}
}

func TestCancellationIsClassified(t *testing.T) {
	_, err := New(bytes.NewReader([]byte{3}), &bytes.Buffer{}).
		Confirm(context.Background(), "Create capture?", true)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("Confirm() error = %v, want ErrCancelled", err)
	}
}

func TestImmediateEOFDoesNotHang(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := New(strings.NewReader(""), &bytes.Buffer{}).
		Confirm(ctx, "Create capture?", true)
	if !errors.Is(err, ErrEOF) {
		t.Fatalf("Confirm() error = %v, want ErrEOF", err)
	}
}

func TestBoundaryValidation(t *testing.T) {
	adapter := New(strings.NewReader(""), &bytes.Buffer{})
	if _, err := adapter.Select(context.Background(), "Kind", nil, ""); err == nil {
		t.Fatal("Select() accepted empty choices")
	}
	if _, err := adapter.Select(context.Background(), "Kind", []string{"thought"}, "idea"); err == nil {
		t.Fatal("Select() accepted invalid default")
	}
	if _, err := (*Adapter)(nil).Text(context.Background(), "Text"); err == nil {
		t.Fatal("Text() accepted missing streams")
	}
}
