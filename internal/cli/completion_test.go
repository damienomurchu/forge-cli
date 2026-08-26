package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCompletionScripts(t *testing.T) {
	tests := []struct {
		shell string
		want  []string
	}{
		{shell: "bash", want: []string{"complete -F _forge_completion forge", "friction action follow-up decision", "--frequency"}},
		{shell: "fish", want: []string{"complete -c forge", "friction action follow-up decision", "current-workaround"}},
		{shell: "zsh", want: []string{"#compdef forge", "compdef _forge forge", "information-finding"}},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			var stdout bytes.Buffer
			rt := Runtime{
				Stdout: &stdout,
				Env: func(string) string {
					t.Fatal("completion inspected environment")
					return ""
				},
			}
			if err := Run(context.Background(), []string{"completion", tt.shell}, rt, "dev"); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if !strings.HasSuffix(stdout.String(), "\n") {
				t.Errorf("script does not end in newline")
			}
			for _, want := range tt.want {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("script missing %q", want)
				}
			}
		})
	}
}

func TestCompletionRejectsInvalidArguments(t *testing.T) {
	tests := [][]string{
		{"completion"},
		{"completion", "powershell"},
		{"completion", "bash", "extra"},
	}
	for _, args := range tests {
		var stdout bytes.Buffer
		err := Run(context.Background(), args, Runtime{Stdout: &stdout}, "dev")
		var usage *UsageError
		if !errors.As(err, &usage) || stdout.Len() != 0 {
			t.Errorf("Run(%q) error/stdout = %T %v/%q, want usage error/empty", args, err, err, stdout.String())
		}
	}
}
