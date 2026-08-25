package promptprobe

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSelectNavigation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"j", "j\r", "idea"},
		{"down arrow", "\x1b[B\r", "idea"},
		{"j then k", "jk\r", "thought"},
		{"down then up", "\x1b[B\x1b[A\r", "thought"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			prompt := Prompt{Input: strings.NewReader(test.input), Output: &stderr}
			got, err := prompt.Select(context.Background(), "Kind", []string{"thought", "idea"}, "thought")
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("Select() = %q, want %q", got, test.want)
			}
			if stderr.Len() == 0 {
				t.Fatal("prompt did not write to its configured output")
			}
		})
	}
}

func TestConfirmAndText(t *testing.T) {
	var stderr bytes.Buffer
	prompt := Prompt{Input: strings.NewReader("n\r"), Output: &stderr}
	confirmed, err := prompt.Confirm(context.Background(), "Save?", true)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed {
		t.Fatal("Confirm() = true, want false")
	}

	stderr.Reset()
	prompt.Input = strings.NewReader("manual workaround\r")
	text, err := prompt.Text(context.Background(), "Workaround")
	if err != nil {
		t.Fatal(err)
	}
	if text != "manual workaround" {
		t.Fatalf("Text() = %q", text)
	}
}

func TestCancellationIsClassifiedAndUsesOnlyPromptOutput(t *testing.T) {
	var promptOutput, jsonStdout bytes.Buffer
	jsonStdout.WriteString(`{"reserved":"json"}` + "\n")
	prompt := Prompt{Input: bytes.NewReader([]byte{3}), Output: &promptOutput}
	_, err := prompt.Confirm(context.Background(), "Save?", true)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("Confirm() error = %v, want ErrCancelled", err)
	}
	if got := jsonStdout.String(); got != "{\"reserved\":\"json\"}\n" {
		t.Fatalf("JSON stdout changed: %q", got)
	}
}

func TestEOFDoesNotHang(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	prompt := Prompt{Input: strings.NewReader(""), Output: &bytes.Buffer{}}
	_, err := prompt.Text(ctx, "Workaround")
	if !errors.Is(err, ErrEOF) {
		t.Fatalf("Text() error = %v, want ErrEOF", err)
	}
}

func TestBoundaryValidation(t *testing.T) {
	prompt := Prompt{Input: strings.NewReader(""), Output: &bytes.Buffer{}}
	if _, err := prompt.Select(context.Background(), "Kind", nil, ""); err == nil {
		t.Fatal("expected empty choices to fail")
	}
	if _, err := prompt.Select(context.Background(), "Kind", []string{"thought"}, "idea"); err == nil {
		t.Fatal("expected invalid default to fail")
	}
	if _, err := (Prompt{}).Text(context.Background(), "Text"); err == nil {
		t.Fatal("expected missing streams to fail")
	}
}
