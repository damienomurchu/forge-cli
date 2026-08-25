//go:build linux || darwin

package prompt

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func TestCtrlCRestoresTerminalState(t *testing.T) {
	referenceMaster, referenceTerminal, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	before, err := term.GetState(int(referenceTerminal.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	_ = referenceTerminal.Close()
	_ = referenceMaster.Close()

	command := exec.Command(os.Args[0], "-test.run=TestPromptHelperProcess")
	command.Env = append(os.Environ(), "FORGE_PROMPT_HELPER=1")
	master, err := pty.StartWithSize(command, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()

	deadline := time.Now().Add(2 * time.Second)
	type readResult struct {
		data []byte
		err  error
	}
	reads := make(chan readResult, 1)
	go func() {
		buffer := make([]byte, 1024)
		for {
			count, readErr := master.Read(buffer)
			chunk := append([]byte(nil), buffer[:count]...)
			reads <- readResult{data: chunk, err: readErr}
			if readErr != nil {
				return
			}
		}
	}()
	var output []byte
	answeredBackground := false
	answeredCursor := false
	for !bytes.Contains(output, []byte("Create capture?")) {
		select {
		case read := <-reads:
			output = append(output, read.data...)
			if !answeredBackground && bytes.Contains(output, []byte("\x1b]11;?\x1b\\")) {
				if _, err := master.Write([]byte("\x1b]11;rgb:0000/0000/0000\x1b\\")); err != nil {
					t.Fatal(err)
				}
				answeredBackground = true
			}
			if !answeredCursor && bytes.Contains(output, []byte("\x1b[6n")) {
				if _, err := master.Write([]byte("\x1b[1;1R")); err != nil {
					t.Fatal(err)
				}
				answeredCursor = true
			}
			if read.err != nil {
				_ = command.Process.Kill()
				t.Fatalf("wait for prompt rendering: %v; output: %q", read.err, output)
			}
		case waitErr := <-done:
			t.Fatalf("prompt helper exited before rendering: %v; output: %q", waitErr, output)
		case <-time.After(time.Until(deadline)):
			_ = command.Process.Kill()
			t.Fatalf("prompt did not render; output: %q", output)
		}
	}
	if _, err := master.Write([]byte{3}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		var exitErr *exec.ExitError
		if err == nil || !errors.As(err, &exitErr) || exitErr.ExitCode() != 130 {
			t.Fatalf("helper exit = %v, want 130", err)
		}
	case <-time.After(3 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("prompt did not exit after Ctrl-C")
	}

	after, err := term.GetState(int(master.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("terminal state was not restored after Ctrl-C")
	}
}

func TestPromptHelperProcess(t *testing.T) {
	if os.Getenv("FORGE_PROMPT_HELPER") != "1" {
		return
	}
	_, err := New(os.Stdin, os.Stderr).Confirm(context.Background(), "Create capture?", true)
	if errors.Is(err, ErrCancelled) {
		os.Exit(130)
	}
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
