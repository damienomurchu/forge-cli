// Package cli implements Forge's command-line interface.
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/damienomurchu/forge-cli/internal/config"
	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/output"
	promptui "github.com/damienomurchu/forge-cli/internal/prompt"
	"github.com/damienomurchu/forge-cli/internal/repository"
	"github.com/damienomurchu/forge-cli/internal/storage"
)

const help = `Forge captures work that deserves attention.

Usage:
  forge
  forge [command] [flags]

Commands:
  capture    Capture friction, an action, a follow-up, or a decision
  completion Generate shell completion scripts
  list       List captures
  show       Show a capture

Flags:
  -h, --help      Show help
      --version   Show version
`

const captureHelp = `Capture friction, an action, a follow-up, or a decision.

Usage:
  forge capture [--json] DESCRIPTION
  forge capture --quick --type TYPE [type-specific options] [--json] DESCRIPTION

Flags:
  -h, --help                     Show help
      --category CATEGORY        Set friction category (default: other)
      --current-workaround TEXT  Record the current friction workaround
      --frequency FREQUENCY      Set friction frequency (default: unknown)
      --impact IMPACT            Set friction impact (default: unknown)
      --json                     Write the created capture as JSON
      --project PROJECT          Associate friction with a project
      --quick                    Capture without prompting
      --type TYPE                Set friction, action, follow-up, or decision
`

const listHelp = `List captures newest first.

Usage:
  forge list [--limit N] [--type TYPE] [--project PROJECT] [--json]

Output:
  <id>  <capture-type>  <description>

Missing storage and empty results produce no output and succeed.

Flags:
  -h, --help            Show help
      --json             Write records as a JSON array
      --limit N          Return at most N records
      --project PROJECT  Filter by project
      --type TYPE        Filter by capture type
`

const showHelp = `Show one complete capture.

Usage:
  forge show [--json] RECORD_ID

Flags:
  -h, --help  Show help
      --json   Write the record as JSON
`

const completionHelp = `Generate a shell completion script.

Usage:
  forge completion SHELL

Supported shells:
  bash
  fish
  zsh

Flags:
  -h, --help  Show help
`

// Runtime contains process facilities used by the CLI.
type Runtime struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Env    func(string) string
	Now    func() time.Time
	Random io.Reader
	IsTTY  func() bool
	Prompt func() Prompt
	GOOS   string
	EUID   int
}

// Prompt is the interactive boundary used after parsing, validation, and TTY
// detection. Implementations must route rendering away from command stdout.
type Prompt interface {
	Select(context.Context, string, []string, string) (string, error)
	Text(context.Context, string) (string, error)
	Confirm(context.Context, string, bool) (bool, error)
}

type UsageError struct {
	Argument string
	Message  string
}

type InterruptedError struct {
	Message string
	Cause   error
}

func (e *InterruptedError) Error() string { return e.Message }
func (e *InterruptedError) Unwrap() error { return e.Cause }

func (e *UsageError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("unknown argument %q", e.Argument)
}

func Run(ctx context.Context, args []string, rt Runtime, version string) error {
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
	case args[0] == "capture":
		return runCapture(ctx, args[1:], rt)
	case args[0] == "completion":
		return runCompletion(args[1:], rt)
	case args[0] == "list":
		return runList(ctx, args[1:], rt)
	case args[0] == "show":
		return runShow(ctx, args[1:], rt)
	default:
		return &UsageError{Argument: args[0]}
	}
}

func runCapture(ctx context.Context, args []string, rt Runtime) error {
	if commandHelpRequested(args) {
		_, err := io.WriteString(rt.Stdout, captureHelp)
		return err
	}
	request, err := parseCaptureRequest(args)
	if err != nil {
		return err
	}
	var proposed domain.ProposedCapture
	if request.quick {
		proposed = *request.proposed
	} else {
		if rt.IsTTY == nil || !rt.IsTTY() {
			return &domain.InvalidValueError{Field: "interaction", Value: "stdin is not a terminal"}
		}
		if rt.Prompt == nil {
			return errors.New("capture prompt is not configured")
		}
		prompter := rt.Prompt()
		if prompter == nil {
			return errors.New("capture prompt is not configured")
		}
		if rt.Stderr == nil {
			return errors.New("capture summary writer is not configured")
		}
		var confirmed bool
		proposed, confirmed, err = collectCapture(ctx, request, prompter, rt.Stderr)
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}

	databasePath, err := config.ResolveDatabasePath(rt.GOOS, rt.Env)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	session, err := storage.OpenForCreation(ctx, databasePath, rt.EUID)
	if err != nil {
		return fmt.Errorf("open storage for capture: %w", err)
	}
	repo, err := repository.New(session.Database())
	if err != nil {
		return errors.Join(err, session.Close())
	}
	var rendered bytes.Buffer
	if _, err := persistCapture(
		ctx, proposed, request.json, rt.Now(), rt.Random, repo, &rendered,
	); err != nil {
		return errors.Join(err, session.Close())
	}
	return closeAndPublish(session, &rendered, rt.Stdout, "capture")
}

func runList(ctx context.Context, args []string, rt Runtime) error {
	if commandHelpRequested(args) {
		_, err := io.WriteString(rt.Stdout, listHelp)
		return err
	}
	request, err := parseListRequest(args)
	if err != nil {
		return err
	}
	databasePath, err := config.ResolveDatabasePath(rt.GOOS, rt.Env)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	session, err := storage.OpenExisting(ctx, databasePath, rt.EUID, storage.DatabaseReadOnly)
	if errors.Is(err, storage.ErrStorageNotFound) {
		var rendered bytes.Buffer
		if request.json {
			err = output.WriteCapturesJSON(&rendered, []domain.Capture{})
		} else {
			err = output.WriteCaptureList(&rendered, []domain.Capture{})
		}
		if err != nil {
			return fmt.Errorf("render list result: %w", err)
		}
		return publish(&rendered, rt.Stdout, "list")
	}
	if err != nil {
		return fmt.Errorf("open storage for list: %w", err)
	}
	repo, err := repository.New(session.Database())
	if err != nil {
		return errors.Join(err, session.Close())
	}
	var rendered bytes.Buffer
	if err := executeList(ctx, request, repo, &rendered); err != nil {
		return errors.Join(err, session.Close())
	}
	return closeAndPublish(session, &rendered, rt.Stdout, "list")
}

func runShow(ctx context.Context, args []string, rt Runtime) error {
	if commandHelpRequested(args) {
		_, err := io.WriteString(rt.Stdout, showHelp)
		return err
	}
	id, jsonOutput, err := parseShow(args)
	if err != nil {
		return err
	}
	databasePath, err := config.ResolveDatabasePath(rt.GOOS, rt.Env)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	session, err := storage.OpenExisting(ctx, databasePath, rt.EUID, storage.DatabaseReadOnly)
	if errors.Is(err, storage.ErrStorageNotFound) {
		return fmt.Errorf("record %q not found", id.String())
	}
	if err != nil {
		return fmt.Errorf("open storage for show: %w", err)
	}
	repo, err := repository.New(session.Database())
	if err != nil {
		return errors.Join(err, session.Close())
	}
	var rendered bytes.Buffer
	err = executeShow(ctx, id, jsonOutput, repo, &rendered)
	if errors.Is(err, repository.ErrRecordNotFound) {
		return errors.Join(fmt.Errorf("record %q not found", id.String()), session.Close())
	}
	if err != nil {
		return errors.Join(err, session.Close())
	}
	return closeAndPublish(session, &rendered, rt.Stdout, "show")
}

func parseShow(args []string) (domain.ID, bool, error) {
	var id domain.ID
	idSet, jsonOutput, jsonSet := false, false, false
	for _, argument := range args {
		switch {
		case argument == "--json":
			if jsonSet {
				return "", false, &UsageError{Message: "--json may only be specified once"}
			}
			jsonOutput, jsonSet = true, true
		case strings.HasPrefix(argument, "-"):
			return "", false, &UsageError{Argument: argument}
		case idSet:
			return "", false, &UsageError{Argument: argument}
		default:
			id, idSet = domain.ID(argument), true
		}
	}
	if !idSet {
		return "", false, &UsageError{Message: "record ID is required"}
	}
	if err := validateLookupID(id); err != nil {
		return "", false, err
	}
	return id, jsonOutput, nil
}

// closeAndPublish ensures a successful command result is not exposed until its
// storage session has closed successfully.
func closeAndPublish(session *storage.Session, rendered *bytes.Buffer, stdout io.Writer, command string) error {
	if err := session.Close(); err != nil {
		return fmt.Errorf("close storage after %s: %w", command, err)
	}
	return publish(rendered, stdout, command)
}

func publish(rendered *bytes.Buffer, stdout io.Writer, command string) error {
	if _, err := io.Copy(stdout, rendered); err != nil {
		return fmt.Errorf("write %s result: %w", command, err)
	}
	return nil
}

func validateLookupID(id domain.ID) error {
	value := id.String()
	if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
		return &domain.InvalidValueError{Field: "record ID", Value: value}
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return &domain.InvalidValueError{Field: "record ID", Value: value}
		}
	}
	return nil
}

func capturePromptError(action string, err error) error {
	if errors.Is(err, promptui.ErrCancelled) {
		return &InterruptedError{Message: "capture cancelled", Cause: err}
	}
	return fmt.Errorf("%s capture: %w", action, err)
}

func commandHelpRequested(args []string) bool {
	for _, argument := range args {
		if argument == "--" {
			return false
		}
		if argument == "-h" || argument == "--help" {
			return true
		}
	}
	return false
}

func WriteError(w io.Writer, err error) int {
	var interrupted *InterruptedError
	if errors.As(err, &interrupted) {
		fmt.Fprintf(w, "forge: %s\n", interrupted)
		return 130
	}
	if usageErr, ok := err.(*UsageError); ok {
		fmt.Fprintf(w, "forge: %s\nTry 'forge --help' for usage.\n", usageErr)
		return 2
	}
	fmt.Fprintf(w, "forge: %s\n", err)
	return 1
}
