// Package cli implements Forge's command-line interface without initializing
// storage or interactive prompting.
package cli

import (
	"context"
	"fmt"
	"io"
	"time"
)

const help = `Forge captures ideas and friction in day-to-day work.

Usage:
  forge
  forge [command] [flags]

Commands:
  capture    Capture a thought, idea, or observation
  friction   Record recurring friction
  list       List records
  show       Show a record
  update     Update a record
  review     Review captured friction

Flags:
  -h, --help      Show help
      --version   Show version
`

// Runtime contains process facilities used by the CLI. Keeping them explicit
// makes command behavior deterministic in tests and keeps global process state
// out of application code.
type Runtime struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Env    func(string) string
	Now    func() time.Time
	Random io.Reader
	IsTTY  func() bool
}

// UsageError reports an invalid command line.
type UsageError struct {
	Argument string
}

func (e *UsageError) Error() string {
	return fmt.Sprintf("unknown argument %q", e.Argument)
}

// Run executes Forge for args. The caller owns error presentation and process
// exit so application code never terminates the process directly.
func Run(_ context.Context, args []string, rt Runtime, version string) error {
	switch {
	case len(args) == 0:
		_, err := io.WriteString(rt.Stdout, help)
		return err
	case len(args) == 1 && (args[0] == "-h" || args[0] == "--help"):
		_, err := io.WriteString(rt.Stdout, help)
		return err
	case len(args) == 1 && args[0] == "--version":
		_, err := fmt.Fprintf(rt.Stdout, "forge %s\n", version)
		return err
	default:
		return &UsageError{Argument: args[0]}
	}
}

// WriteError writes a concise user-facing error and returns its exit status.
func WriteError(w io.Writer, err error) int {
	if usageErr, ok := err.(*UsageError); ok {
		fmt.Fprintf(w, "forge: %s\nTry 'forge --help' for usage.\n", usageErr)
		return 2
	}

	fmt.Fprintf(w, "forge: %s\n", err)
	return 1
}
