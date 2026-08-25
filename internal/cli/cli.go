// Package cli implements Forge's command-line interface.
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
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
  forge capture [--quick] [--project PROJECT] [--kind KIND]
                [--tags TAGS] [--json] DESCRIPTION

Flags:
  -h, --help             Show help
      --kind KIND        Set capture kind (default: thought)
      --project PROJECT  Associate the capture with a project
      --quick            Capture without confirmation
      --tags TAGS        Add comma-separated tags
      --json             Write the created record as JSON
`

const frictionHelp = `Record recurring friction.

Usage:
  forge friction [--quick] [--project PROJECT] [--frequency FREQUENCY]
                 [--impact IMPACT] [--category CATEGORY]
                 [--current-workaround TEXT] [--json] DESCRIPTION

Defaults in quick mode:
  frequency  unknown
  impact     unknown
  category   other

Flags:
  -h, --help                     Show help
      --category CATEGORY        Set friction category (default: other)
      --current-workaround TEXT  Record the current workaround
      --frequency FREQUENCY      Set occurrence frequency (default: unknown)
      --impact IMPACT            Set severity (default: unknown)
      --json                     Write the created record as JSON
      --project PROJECT          Associate the friction with a project
      --quick                    Record without prompting
`

const listHelp = `List records newest first.

Usage:
  forge list [--limit N] [--type TYPE] [--project PROJECT] [--status STATUS]
             [--json]

Output:
  <id>  <type>  <status>  <description>

Missing storage and empty results produce no output and succeed.

Flags:
  -h, --help            Show help
      --json             Write records as a JSON array
      --limit N          Return at most N records
      --project PROJECT  Filter by project
      --status STATUS    Filter by lifecycle status
      --type TYPE        Filter by capture or friction
`

const showHelp = `Show one complete record.

Usage:
  forge show [--json] RECORD_ID

Flags:
  -h, --help  Show help
      --json   Write the record as JSON
`

const updateHelp = `Update a record's lifecycle status.

Usage:
  forge update [--json] RECORD_ID --status STATUS

Flags:
  -h, --help          Show help
      --json           Write the resulting record as JSON
      --status STATUS  Set the lifecycle status
`

const reviewHelp = `Review actionable friction.

Usage:
  forge review [--json]

Includes friction in captured, reviewing, or candidate status.

Output:
  <id>  <status>  <frequency>  <impact>  <category>  <description>

Flags:
  -h, --help  Show help
      --json   Write records as a JSON array
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

// UsageError reports an invalid command line.
type UsageError struct {
	Argument string
	Message  string
}

// InterruptedError reports a user interruption with command-specific wording.
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
	case len(args) > 0 && args[0] == "list":
		return runList(ctx, args[1:], rt)
	case len(args) > 0 && args[0] == "show":
		return runShow(ctx, args[1:], rt)
	case len(args) > 0 && args[0] == "update":
		return runUpdate(ctx, args[1:], rt)
	case len(args) > 0 && args[0] == "review":
		return runReview(ctx, args[1:], rt)
	default:
		return &UsageError{Argument: args[0]}
	}
}

func runReview(ctx context.Context, args []string, rt Runtime) error {
	if commandHelpRequested(args) {
		_, err := io.WriteString(rt.Stdout, reviewHelp)
		return err
	}
	jsonOutput, err := parseReview(args)
	if err != nil {
		return err
	}
	databasePath, err := config.ResolveDatabasePath(rt.GOOS, rt.Env)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	session, err := storage.OpenExisting(ctx, databasePath, rt.EUID, storage.DatabaseReadOnly)
	if errors.Is(err, storage.ErrStorageNotFound) {
		if err := writeReviewResult(rt.Stdout, nil, jsonOutput); err != nil {
			return fmt.Errorf("write review result: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("open storage for review: %w", err)
	}
	repo, err := repository.New(session.Database())
	if err != nil {
		return errors.Join(err, session.Close())
	}
	records, err := repo.Review(ctx)
	if err != nil {
		return errors.Join(err, session.Close())
	}
	if err := session.Close(); err != nil {
		return fmt.Errorf("close storage after review: %w", err)
	}
	if err := writeReviewResult(rt.Stdout, records, jsonOutput); err != nil {
		return fmt.Errorf("write review result: %w", err)
	}
	return nil
}

func parseReview(args []string) (bool, error) {
	jsonOutput := false
	for _, arg := range args {
		if arg != "--json" {
			return false, &UsageError{Argument: arg}
		}
		if jsonOutput {
			return false, &UsageError{Message: "--json may only be specified once"}
		}
		jsonOutput = true
	}
	return jsonOutput, nil
}

func writeReviewResult(w io.Writer, records []domain.Record, jsonOutput bool) error {
	var rendered bytes.Buffer
	var err error
	if jsonOutput {
		err = output.WriteRecordsJSON(&rendered, records)
	} else {
		err = output.WriteReview(&rendered, records)
	}
	if err != nil {
		return err
	}
	_, err = io.Copy(w, &rendered)
	return err
}

type updateCommandOptions struct {
	id     domain.ID
	status domain.Status
	json   bool
}

func runUpdate(ctx context.Context, args []string, rt Runtime) error {
	if commandHelpRequested(args) {
		_, err := io.WriteString(rt.Stdout, updateHelp)
		return err
	}
	options, err := parseUpdate(args)
	if err != nil {
		return err
	}
	databasePath, err := config.ResolveDatabasePath(rt.GOOS, rt.Env)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	session, err := storage.OpenExisting(ctx, databasePath, rt.EUID, storage.DatabaseReadWrite)
	if errors.Is(err, storage.ErrStorageNotFound) {
		return fmt.Errorf("record %q not found", options.id.String())
	}
	if err != nil {
		return fmt.Errorf("open storage for update: %w", err)
	}
	repo, err := repository.New(session.Database())
	if err != nil {
		return errors.Join(err, session.Close())
	}
	record, _, err := repo.UpdateStatus(ctx, options.id, options.status, rt.Now())
	if errors.Is(err, repository.ErrRecordNotFound) {
		return errors.Join(fmt.Errorf("record %q not found", options.id.String()), session.Close())
	}
	if err != nil {
		return errors.Join(err, session.Close())
	}
	if err := session.Close(); err != nil {
		return fmt.Errorf("close storage after update: %w", err)
	}
	if err := writeUpdateResult(rt.Stdout, record, options.json); err != nil {
		return fmt.Errorf("write update result: %w", err)
	}
	return nil
}

func parseUpdate(args []string) (updateCommandOptions, error) {
	var options updateCommandOptions
	idSet := false
	statusSet := false
	jsonSet := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--json":
			if jsonSet {
				return updateCommandOptions{}, &UsageError{Message: "--json may only be specified once"}
			}
			jsonSet = true
			options.json = true
		case arg == "--status":
			if statusSet {
				return updateCommandOptions{}, &UsageError{Message: "--status may only be specified once"}
			}
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return updateCommandOptions{}, &UsageError{Message: "--status requires a value"}
			}
			index++
			status, err := domain.ParseStatus(args[index])
			if err != nil {
				return updateCommandOptions{}, err
			}
			statusSet = true
			options.status = status
		case strings.HasPrefix(arg, "--status="):
			if statusSet {
				return updateCommandOptions{}, &UsageError{Message: "--status may only be specified once"}
			}
			value := strings.TrimPrefix(arg, "--status=")
			if value == "" {
				return updateCommandOptions{}, &UsageError{Message: "--status requires a value"}
			}
			status, err := domain.ParseStatus(value)
			if err != nil {
				return updateCommandOptions{}, err
			}
			statusSet = true
			options.status = status
		case strings.HasPrefix(arg, "-"):
			return updateCommandOptions{}, &UsageError{Argument: arg}
		case idSet:
			return updateCommandOptions{}, &UsageError{Argument: arg}
		default:
			options.id = domain.ID(arg)
			idSet = true
		}
	}
	if !idSet {
		return updateCommandOptions{}, &UsageError{Message: "record ID is required"}
	}
	if !statusSet {
		return updateCommandOptions{}, &UsageError{Message: "--status is required"}
	}
	if err := validateLookupID(options.id); err != nil {
		return updateCommandOptions{}, err
	}
	return options, nil
}

func writeUpdateResult(w io.Writer, record domain.Record, jsonOutput bool) error {
	var rendered bytes.Buffer
	var err error
	if jsonOutput {
		err = output.WriteRecordJSON(&rendered, record)
	} else {
		err = output.WriteUpdated(&rendered, record)
	}
	if err != nil {
		return err
	}
	_, err = io.Copy(w, &rendered)
	return err
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
	record, err := repo.FindByID(ctx, id)
	if errors.Is(err, repository.ErrRecordNotFound) {
		return errors.Join(fmt.Errorf("record %q not found", id.String()), session.Close())
	}
	if err != nil {
		return errors.Join(err, session.Close())
	}
	if err := session.Close(); err != nil {
		return fmt.Errorf("close storage after show: %w", err)
	}
	if err := writeShowResult(rt.Stdout, record, jsonOutput); err != nil {
		return fmt.Errorf("write show result: %w", err)
	}
	return nil
}

func parseShow(args []string) (domain.ID, bool, error) {
	var id domain.ID
	idSet := false
	jsonOutput := false
	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOutput = true
		case strings.HasPrefix(arg, "-"):
			return "", false, &UsageError{Argument: arg}
		case idSet:
			return "", false, &UsageError{Argument: arg}
		default:
			id = domain.ID(arg)
			idSet = true
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

func writeShowResult(w io.Writer, record domain.Record, jsonOutput bool) error {
	var rendered bytes.Buffer
	var err error
	if jsonOutput {
		err = output.WriteRecordJSON(&rendered, record)
	} else {
		err = output.WriteRecord(&rendered, record)
	}
	if err != nil {
		return err
	}
	_, err = io.Copy(w, &rendered)
	return err
}

func runList(ctx context.Context, args []string, rt Runtime) error {
	if commandHelpRequested(args) {
		_, err := io.WriteString(rt.Stdout, listHelp)
		return err
	}
	options, err := parseList(args)
	if err != nil {
		return err
	}
	databasePath, err := config.ResolveDatabasePath(rt.GOOS, rt.Env)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	session, err := storage.OpenExisting(ctx, databasePath, rt.EUID, storage.DatabaseReadOnly)
	if errors.Is(err, storage.ErrStorageNotFound) {
		if err := writeListResult(rt.Stdout, nil, options.json); err != nil {
			return fmt.Errorf("write list result: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("open storage for list: %w", err)
	}
	repo, err := repository.New(session.Database())
	if err != nil {
		return errors.Join(err, session.Close())
	}
	records, err := repo.List(ctx, options.filters)
	if err != nil {
		return errors.Join(err, session.Close())
	}
	if err := session.Close(); err != nil {
		return fmt.Errorf("close storage after list: %w", err)
	}
	if err := writeListResult(rt.Stdout, records, options.json); err != nil {
		return fmt.Errorf("write list result: %w", err)
	}
	return nil
}

type listCommandOptions struct {
	filters repository.ListOptions
	json    bool
}

func parseList(args []string) (listCommandOptions, error) {
	options := listCommandOptions{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--json":
			options.json = true
		case arg == "--type":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return listCommandOptions{}, &UsageError{Message: "--type requires a value"}
			}
			index++
			recordType, err := domain.ParseRecordType(args[index])
			if err != nil {
				return listCommandOptions{}, err
			}
			options.filters.Type = &recordType
		case strings.HasPrefix(arg, "--type="):
			value := strings.TrimPrefix(arg, "--type=")
			if value == "" {
				return listCommandOptions{}, &UsageError{Message: "--type requires a value"}
			}
			recordType, err := domain.ParseRecordType(value)
			if err != nil {
				return listCommandOptions{}, err
			}
			options.filters.Type = &recordType
		case arg == "--project":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return listCommandOptions{}, &UsageError{Message: "--project requires a value"}
			}
			index++
			project := domain.NormalizeOptionalText(args[index])
			if project == nil {
				return listCommandOptions{}, &domain.InvalidValueError{Field: "project", Value: args[index]}
			}
			options.filters.Project = project
		case strings.HasPrefix(arg, "--project="):
			value := strings.TrimPrefix(arg, "--project=")
			if value == "" {
				return listCommandOptions{}, &UsageError{Message: "--project requires a value"}
			}
			project := domain.NormalizeOptionalText(value)
			if project == nil {
				return listCommandOptions{}, &domain.InvalidValueError{Field: "project", Value: value}
			}
			options.filters.Project = project
		case arg == "--status":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return listCommandOptions{}, &UsageError{Message: "--status requires a value"}
			}
			index++
			status, err := domain.ParseStatus(args[index])
			if err != nil {
				return listCommandOptions{}, err
			}
			options.filters.Status = &status
		case strings.HasPrefix(arg, "--status="):
			value := strings.TrimPrefix(arg, "--status=")
			if value == "" {
				return listCommandOptions{}, &UsageError{Message: "--status requires a value"}
			}
			status, err := domain.ParseStatus(value)
			if err != nil {
				return listCommandOptions{}, err
			}
			options.filters.Status = &status
		case arg == "--limit":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return listCommandOptions{}, &UsageError{Message: "--limit requires a value"}
			}
			index++
			limit, err := parseListLimit(args[index])
			if err != nil {
				return listCommandOptions{}, err
			}
			options.filters.Limit = &limit
		case strings.HasPrefix(arg, "--limit="):
			value := strings.TrimPrefix(arg, "--limit=")
			if value == "" {
				return listCommandOptions{}, &UsageError{Message: "--limit requires a value"}
			}
			limit, err := parseListLimit(value)
			if err != nil {
				return listCommandOptions{}, err
			}
			options.filters.Limit = &limit
		default:
			return listCommandOptions{}, &UsageError{Argument: arg}
		}
	}
	return options, nil
}

func parseListLimit(value string) (int, error) {
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, &domain.InvalidValueError{Field: "limit", Value: value}
		}
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return 0, &domain.InvalidValueError{Field: "limit", Value: value}
	}
	return limit, nil
}

func writeListResult(w io.Writer, records []domain.Record, jsonOutput bool) error {
	if !jsonOutput {
		return output.WriteRecordList(w, records)
	}
	var rendered bytes.Buffer
	if err := output.WriteRecordsJSON(&rendered, records); err != nil {
		return fmt.Errorf("render JSON: %w", err)
	}
	_, err := io.Copy(w, &rendered)
	return err
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
	if !options.quick {
		if _, err := domain.NormalizeDescription(options.description); err != nil {
			return err
		}
		if !options.frequencySet || !options.impactSet || !options.categorySet {
			return &UsageError{Message: "interactive friction currently requires explicit frequency, impact, and category"}
		}
		if rt.IsTTY == nil || !rt.IsTTY() {
			return &domain.InvalidValueError{Field: "interaction", Value: "stdin is not a terminal"}
		}
		if rt.Prompt == nil {
			return errors.New("friction prompt is not configured")
		}
		prompt := rt.Prompt()
		if prompt == nil {
			return errors.New("friction prompt is not configured")
		}
		confirmed, err := prompt.Confirm(ctx, "Create friction?", true)
		if err != nil {
			return frictionPromptError("confirm", err)
		}
		if !confirmed {
			return nil
		}
	}
	record, err := domain.NewFriction(domain.FrictionInput{
		Description:       options.description,
		Project:           options.project,
		Frequency:         options.frequency,
		Impact:            options.impact,
		Category:          options.category,
		CurrentWorkaround: options.currentWorkaround,
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
	if options.json {
		var rendered bytes.Buffer
		if err := output.WriteRecordJSON(&rendered, record); err != nil {
			return fmt.Errorf("render friction result: %w", err)
		}
		if _, err := io.Copy(rt.Stdout, &rendered); err != nil {
			return fmt.Errorf("write friction result: %w", err)
		}
		return nil
	}
	if err := output.WriteCreated(rt.Stdout, record); err != nil {
		return fmt.Errorf("write friction result: %w", err)
	}
	return nil
}

type quickFrictionOptions struct {
	description       string
	project           string
	frequency         domain.Frequency
	impact            domain.Impact
	category          domain.Category
	currentWorkaround string
	json              bool
	quick             bool
	frequencySet      bool
	impactSet         bool
	categorySet       bool
}

func parseQuickFriction(args []string) (quickFrictionOptions, error) {
	quick := false
	optionsEnded := false
	positionals := make([]string, 0, 1)
	frequency := domain.FrequencyUnknown
	frequencySet := false
	impact := domain.ImpactUnknown
	impactSet := false
	category := domain.CategoryOther
	categorySet := false
	project := ""
	currentWorkaround := ""
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
			frequencySet = true
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
			frequencySet = true
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
			impactSet = true
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
			impactSet = true
		case !optionsEnded && arg == "--category":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return quickFrictionOptions{}, &UsageError{Message: "--category requires a value"}
			}
			index++
			parsed, err := domain.ParseCategory(args[index])
			if err != nil {
				return quickFrictionOptions{}, err
			}
			category = parsed
			categorySet = true
		case !optionsEnded && strings.HasPrefix(arg, "--category="):
			value := strings.TrimPrefix(arg, "--category=")
			if value == "" {
				return quickFrictionOptions{}, &UsageError{Message: "--category requires a value"}
			}
			parsed, err := domain.ParseCategory(value)
			if err != nil {
				return quickFrictionOptions{}, err
			}
			category = parsed
			categorySet = true
		case !optionsEnded && arg == "--project":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return quickFrictionOptions{}, &UsageError{Message: "--project requires a value"}
			}
			index++
			project = args[index]
		case !optionsEnded && strings.HasPrefix(arg, "--project="):
			project = strings.TrimPrefix(arg, "--project=")
		case !optionsEnded && arg == "--current-workaround":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				return quickFrictionOptions{}, &UsageError{Message: "--current-workaround requires a value"}
			}
			index++
			currentWorkaround = args[index]
		case !optionsEnded && strings.HasPrefix(arg, "--current-workaround="):
			currentWorkaround = strings.TrimPrefix(arg, "--current-workaround=")
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
	return quickFrictionOptions{
		description:       positionals[0],
		project:           project,
		frequency:         frequency,
		impact:            impact,
		category:          category,
		currentWorkaround: currentWorkaround,
		json:              jsonOutput,
		quick:             quick,
		frequencySet:      frequencySet,
		impactSet:         impactSet,
		categorySet:       categorySet,
	}, nil
}

func frictionPromptError(action string, err error) error {
	if errors.Is(err, promptui.ErrCancelled) {
		return &InterruptedError{Message: "friction cancelled", Cause: err}
	}
	return fmt.Errorf("%s friction: %w", action, err)
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
	if !options.quick {
		if _, err := domain.NormalizeDescription(options.description); err != nil {
			return err
		}
		if rt.IsTTY == nil || !rt.IsTTY() {
			return &domain.InvalidValueError{Field: "interaction", Value: "stdin is not a terminal"}
		}
		if rt.Prompt == nil {
			return errors.New("capture prompt is not configured")
		}
		prompt := rt.Prompt()
		if prompt == nil {
			return errors.New("capture prompt is not configured")
		}
		if !options.kindSet {
			selected, err := prompt.Select(ctx, "Kind", captureKindChoices(), domain.CaptureKindThought.String())
			if err != nil {
				return capturePromptError("select kind for", err)
			}
			kind, err := domain.ParseCaptureKind(selected)
			if err != nil {
				return fmt.Errorf("validate selected capture kind: %w", err)
			}
			options.kind = kind
		}
		confirmed, err := prompt.Confirm(ctx, "Create capture?", true)
		if err != nil {
			return capturePromptError("confirm", err)
		}
		if !confirmed {
			return nil
		}
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

func captureKindChoices() []string {
	return []string{
		domain.CaptureKindThought.String(),
		domain.CaptureKindIdea.String(),
		domain.CaptureKindObservation.String(),
		domain.CaptureKindQuestion.String(),
		domain.CaptureKindDecision.String(),
		domain.CaptureKindSeed.String(),
	}
}

func capturePromptError(action string, err error) error {
	if errors.Is(err, promptui.ErrCancelled) {
		return &InterruptedError{Message: "capture cancelled", Cause: err}
	}
	return fmt.Errorf("%s capture: %w", action, err)
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
	quick       bool
	kindSet     bool
}

func parseQuickCapture(args []string) (quickCaptureOptions, error) {
	quick := false
	optionsEnded := false
	positionals := make([]string, 0, 1)
	kind := domain.CaptureKindThought
	kindSet := false
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
			kindSet = true
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
			kindSet = true
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
	return quickCaptureOptions{
		description: positionals[0],
		project:     project,
		kind:        kind,
		tags:        tags,
		json:        jsonOutput,
		quick:       quick,
		kindSet:     kindSet,
	}, nil
}

// WriteError writes a concise user-facing error and returns its exit status.
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
