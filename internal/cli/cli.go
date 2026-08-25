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

	"github.com/damienomurchu/forge-cli/internal/config"
	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/output"
	"github.com/damienomurchu/forge-cli/internal/repository"
	"github.com/damienomurchu/forge-cli/internal/storage"
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

const captureHelp = `Capture a thought, idea, or observation.

Usage:
  forge capture --quick [--project PROJECT] [--kind KIND]
                [--tags TAGS] [--json] DESCRIPTION

Flags:
  -h, --help             Show help
      --kind KIND        Set capture kind (default: thought)
      --project PROJECT  Associate the capture with a project
      --quick            Capture without prompting (currently required)
      --tags TAGS        Add comma-separated tags
      --json             Write the created record as JSON
`

const frictionHelp = `Record recurring friction.

Usage:
  forge friction --quick [--frequency FREQUENCY]
                 [--impact IMPACT] DESCRIPTION

Defaults in quick mode:
  frequency  unknown
  impact     unknown
  category   other

Flags:
  -h, --help                 Show help
      --frequency FREQUENCY  Set occurrence frequency (default: unknown)
      --impact IMPACT        Set severity (default: unknown)
      --quick                Record without prompting (currently required)
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
	GOOS   string
	EUID   int
}

// UsageError reports an invalid command line.
type UsageError struct {
	Argument string
	Message  string
}

func (e *UsageError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("unknown argument %q", e.Argument)
}

// Run executes Forge for args. The caller owns error presentation and process
// exit so application code never terminates the process directly.
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
	case len(args) > 0 && args[0] == "capture":
		return runCapture(ctx, args[1:], rt)
	case len(args) > 0 && args[0] == "friction":
		return runFriction(ctx, args[1:], rt)
	default:
		return &UsageError{Argument: args[0]}
	}
}

func runFriction(ctx context.Context, args []string, rt Runtime) error {
	if commandHelpRequested(args) {
		_, err := io.WriteString(rt.Stdout, frictionHelp)
		return err
	}
	options, err := parseQuickFriction(args)
	if err != nil {
		return err
	}
	record, err := domain.NewFriction(domain.FrictionInput{
		Description: options.description,
		Frequency:   options.frequency,
		Impact:      options.impact,
		Category:    domain.CategoryOther,
	}, rt.Now(), rt.Random)
	if err != nil {
		return fmt.Errorf("create friction: %w", err)
	}

	databasePath, err := config.ResolveDatabasePath(rt.GOOS, rt.Env)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	session, err := storage.OpenForCreation(ctx, databasePath, rt.EUID)
	if err != nil {
		return fmt.Errorf("open storage for friction: %w", err)
	}
	repo, err := repository.New(session.Database())
	if err != nil {
		return errors.Join(err, session.Close())
	}
	if err := repo.CreateFriction(ctx, record); err != nil {
		return errors.Join(err, session.Close())
	}
	if err := session.Close(); err != nil {
		return fmt.Errorf("close storage after friction: %w", err)
	}
	if err := output.WriteCreated(rt.Stdout, record); err != nil {
		return fmt.Errorf("write friction result: %w", err)
	}
	return nil
}

type quickFrictionOptions struct {
	description string
	frequency   domain.Frequency
	impact      domain.Impact
}

func parseQuickFriction(args []string) (quickFrictionOptions, error) {
	quick := false
	optionsEnded := false
	positionals := make([]string, 0, 1)
	frequency := domain.FrequencyUnknown
	impact := domain.ImpactUnknown
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case !optionsEnded && arg == "--":
			optionsEnded = true
		case !optionsEnded && arg == "--quick":
			quick = true
		case !optionsEnded && arg == "--frequency":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return quickFrictionOptions{}, &UsageError{Message: "--frequency requires a value"}
			}
			index++
			parsed, err := domain.ParseFrequency(args[index])
			if err != nil {
				return quickFrictionOptions{}, err
			}
			frequency = parsed
		case !optionsEnded && strings.HasPrefix(arg, "--frequency="):
			value := strings.TrimPrefix(arg, "--frequency=")
			if value == "" {
				return quickFrictionOptions{}, &UsageError{Message: "--frequency requires a value"}
			}
			parsed, err := domain.ParseFrequency(value)
			if err != nil {
				return quickFrictionOptions{}, err
			}
			frequency = parsed
		case !optionsEnded && arg == "--impact":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return quickFrictionOptions{}, &UsageError{Message: "--impact requires a value"}
			}
			index++
			parsed, err := domain.ParseImpact(args[index])
			if err != nil {
				return quickFrictionOptions{}, err
			}
			impact = parsed
		case !optionsEnded && strings.HasPrefix(arg, "--impact="):
			value := strings.TrimPrefix(arg, "--impact=")
			if value == "" {
				return quickFrictionOptions{}, &UsageError{Message: "--impact requires a value"}
			}
			parsed, err := domain.ParseImpact(value)
			if err != nil {
				return quickFrictionOptions{}, err
			}
			impact = parsed
		case !optionsEnded && strings.HasPrefix(arg, "-"):
			return quickFrictionOptions{}, &UsageError{Argument: arg}
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) == 0 {
		return quickFrictionOptions{}, &UsageError{Message: "friction requires a description"}
	}
	if len(positionals) > 1 {
		return quickFrictionOptions{}, &UsageError{Message: fmt.Sprintf("unexpected argument %q", positionals[1])}
	}
	if !quick {
		return quickFrictionOptions{}, &UsageError{Message: "friction currently requires --quick"}
	}
	return quickFrictionOptions{
		description: positionals[0],
		frequency:   frequency,
		impact:      impact,
	}, nil
}

func runCapture(ctx context.Context, args []string, rt Runtime) error {
	if commandHelpRequested(args) {
		_, err := io.WriteString(rt.Stdout, captureHelp)
		return err
	}
	options, err := parseQuickCapture(args)
	if err != nil {
		return err
	}
	record, err := domain.NewCapture(domain.CaptureInput{
		Description: options.description,
		Project:     options.project,
		Kind:        options.kind,
		Tags:        options.tags,
	}, rt.Now(), rt.Random)
	if err != nil {
		return fmt.Errorf("create capture: %w", err)
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
	if err := repo.CreateCapture(ctx, record); err != nil {
		return errors.Join(err, session.Close())
	}
	if err := session.Close(); err != nil {
		return fmt.Errorf("close storage after capture: %w", err)
	}
	if options.json {
		var rendered bytes.Buffer
		if err := output.WriteRecordJSON(&rendered, record); err != nil {
			return fmt.Errorf("render capture result: %w", err)
		}
		if _, err := io.Copy(rt.Stdout, &rendered); err != nil {
			return fmt.Errorf("write capture result: %w", err)
		}
		return nil
	}
	if err := output.WriteCreated(rt.Stdout, record); err != nil {
		return fmt.Errorf("write capture result: %w", err)
	}
	return nil
}

func commandHelpRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

type quickCaptureOptions struct {
	description string
	project     string
	kind        domain.CaptureKind
	tags        string
	json        bool
}

func parseQuickCapture(args []string) (quickCaptureOptions, error) {
	quick := false
	optionsEnded := false
	positionals := make([]string, 0, 1)
	kind := domain.CaptureKindThought
	project := ""
	tags := ""
	jsonOutput := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case !optionsEnded && arg == "--":
			optionsEnded = true
		case !optionsEnded && arg == "--quick":
			quick = true
		case !optionsEnded && arg == "--json":
			jsonOutput = true
		case !optionsEnded && arg == "--kind":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return quickCaptureOptions{}, &UsageError{Message: "--kind requires a value"}
			}
			index++
			parsed, err := domain.ParseCaptureKind(args[index])
			if err != nil {
				return quickCaptureOptions{}, err
			}
			kind = parsed
		case !optionsEnded && strings.HasPrefix(arg, "--kind="):
			value := strings.TrimPrefix(arg, "--kind=")
			if value == "" {
				return quickCaptureOptions{}, &UsageError{Message: "--kind requires a value"}
			}
			parsed, err := domain.ParseCaptureKind(value)
			if err != nil {
				return quickCaptureOptions{}, err
			}
			kind = parsed
		case !optionsEnded && arg == "--project":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return quickCaptureOptions{}, &UsageError{Message: "--project requires a value"}
			}
			index++
			project = args[index]
		case !optionsEnded && strings.HasPrefix(arg, "--project="):
			project = strings.TrimPrefix(arg, "--project=")
		case !optionsEnded && arg == "--tags":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return quickCaptureOptions{}, &UsageError{Message: "--tags requires a value"}
			}
			index++
			tags = args[index]
		case !optionsEnded && strings.HasPrefix(arg, "--tags="):
			tags = strings.TrimPrefix(arg, "--tags=")
		case !optionsEnded && len(arg) > 0 && arg[0] == '-':
			return quickCaptureOptions{}, &UsageError{Argument: arg}
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) == 0 {
		return quickCaptureOptions{}, &UsageError{Message: "capture requires a description"}
	}
	if len(positionals) > 1 {
		return quickCaptureOptions{}, &UsageError{Message: fmt.Sprintf("unexpected argument %q", positionals[1])}
	}
	if !quick {
		return quickCaptureOptions{}, &UsageError{Message: "capture currently requires --quick"}
	}
	return quickCaptureOptions{
		description: positionals[0],
		project:     project,
		kind:        kind,
		tags:        tags,
		json:        jsonOutput,
	}, nil
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
