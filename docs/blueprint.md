# Forge CLI: Go Implementation Blueprint

## Purpose

Build Forge as a fast, native Go command-line application for capturing ideas and
friction in day-to-day work. The command design and user workflow are the product;
the implementation should be small, direct, secure, and easy to evolve.

An earlier Python implementation reached version 2.1.0 and is now archived. It is
useful as historical context for the concepts behind Forge, but it is not a
behavioral specification, compatibility oracle, test dependency, or release target.
The Go project may deliberately improve commands, output, errors, storage, and
interaction design.

## Goals

1. Make common commands feel immediate.
2. Preserve the useful product concepts: captures, friction, projects, tags,
   classification, review, and lifecycle status.
3. Define Go behavior explicitly through this document and tests.
4. Keep local data durable, private, inspectable, and easy to back up.
5. Produce native Linux AMD64 and macOS ARM64 executables without a runtime.
6. Keep the codebase small, explicit, and testable.

Initial performance budgets are warm wall-clock medians over at least 30 runs:

- `forge --version` and `forge --help`: at most 30 ms
- commands against a small local database: at most 50 ms

These are targets, not permission to weaken validation, durability, or security.

## Non-goals

- Reproducing Python 2.1.0 byte-for-byte or quirk-for-quirk.
- Maintaining cross-implementation golden tests.
- Keeping old output, prompts, errors, JSON, or exit codes merely because the
  archived implementation used them.
- Importing or invoking Python from Go.
- Adding a daemon solely to hide startup time.
- Adding a web UI, synchronization, plugins, AI features, or a configuration
  framework during the initial implementation.
- Introducing an ORM, dependency-injection framework, or broad CLI framework
  without demonstrated need.
- Optimizing hypothetical workloads before measuring them.

## Sources of truth

Use these sources in order:

1. Approved product contracts, beginning with `docs/cli-contract.md`.
2. The current product and command decisions in this document.
3. Tests encoding approved Go behavior.
4. Current user-facing documentation.
5. The archived Python implementation, as historical context only.

When Python differs, do not assume either behavior is correct. Choose the clearest
design for the Go product, document material decisions, and add focused tests.
Historical behavior becomes a requirement only when explicitly adopted here.

## Initial command design

This is the starting design, not a frozen compatibility contract:

```text
forge
forge --help
forge --version

forge capture [--quick] [--project PROJECT] [--kind KIND] [--tags TAGS]
              [--json] DESCRIPTION

forge friction [--quick] [--project PROJECT] [--frequency FREQUENCY]
               [--impact IMPACT] [--category CATEGORY]
               [--current-workaround TEXT] [--json] DESCRIPTION

forge list [--limit N] [--type TYPE] [--project PROJECT] [--status STATUS]
           [--json]
forge show RECORD_ID [--json]
forge update [--json] RECORD_ID --status STATUS
forge review [--json]
```

Before implementing each command group, confirm that its names, flags, defaults,
and interaction model express the intended workflow. Improve them deliberately
rather than inheriting historical details accidentally.

### Initial vocabulary

```text
record types: capture, friction
statuses: captured, reviewing, candidate, automated, dismissed
capture kinds: thought, idea, observation, question, decision, seed
frequencies: daily, weekly, monthly, occasional, unknown
impacts: low, medium, high, unknown
categories: information-finding, repeated-action, context-switching, remembering,
            verification, waiting, other
```

This is a starting vocabulary. Once exposed through persistent data or a
machine-readable interface, changes require an explicit compatibility decision.

## Intended behavior

### Input

- Descriptions contain non-whitespace text. Trim surrounding whitespace and
  preserve internal whitespace.
- Optional text becomes absent when empty after trimming.
- Tags are comma-separated, trimmed, lowercased, deduplicated, and retain
  first-seen order.
- `--quick` avoids prompts and supplies defaults for omitted classification fields.
- Explicit flags override defaults.
- Prompt only when useful information is missing and stdin is a terminal.
- Declining confirmation or cancelling writes nothing.
- Cancellation restores terminal state.
- Help, version, and argument errors do not create a data directory.

Initial quick defaults:

```text
capture kind: thought
friction frequency: unknown
friction impact: unknown
friction category: other
status: captured
```

### Queries

- List filters compose with AND semantics.
- Limits are positive integers.
- Newest records appear first, with deterministic tie-breaking.
- Review initially includes friction in `captured`, `reviewing`, and `candidate`.
- Review is read-only.
- Updating a missing record is an error.

### Output and errors

- Human output is concise and machine output is valid, stable JSON.
- JSON mode emits no additional human success text on stdout.
- Lists and review emit arrays, including `[]` when empty.
- Human output visibly escapes terminal controls and bidirectional controls.
- Normal errors are concise and never include stack traces.
- Exit-code categories and JSON schemas are defined in `docs/cli-contract.md` and
  `docs/record-contract.md` and must be tested before release.

## Storage design

Use SQLite for local persistence. The Go-owned schema is defined in
`docs/storage-contract.md`: common and typed record fields live in one `records`
table, while ordered capture tags use a normalized `record_tags` table. Opaque
metadata JSON is not used. After release, migrations are append-only and existing
Go-created databases remain supported.

Default locations:

```text
Linux: $XDG_DATA_HOME/forge/forge.db
       falling back to ~/.local/share/forge/forge.db
macOS: ~/Library/Application Support/forge/forge.db
override: $FORGE_DATA_DIR/forge.db
```

Configured base directories must be absolute. A relative `FORGE_DATA_DIR`,
`XDG_DATA_HOME`, or `HOME` value is an error rather than an implicit path beneath
the process working directory. Resolution is pure and does not inspect or create
the resulting path.

Importing an archived Python database is optional future work. If needed, implement
an explicit, tested import or migration path; do not constrain the core model
silently for compatibility.

### Filesystem security

- Resolve the data directory centrally and create it lazily with mode `0700`.
- Require the directory and database to be owned by the effective user.
- Reject symbolic links at data-directory and database paths.
- Require a regular database file and create or correct mode `0600`.
- Use no-follow/open-and-inspect facilities where supported to avoid races.
- Keep platform operations behind a small internal boundary.
- Never weaken these rules to meet a latency budget.

## Go technology decisions

- Use and pin the newest stable Go release supported by CI.
- Prefer the standard library and custom command dispatch.
- Use `crypto/rand`, `encoding/json`, and `time` for standard concerns.
- Evaluate SQLite drivers with a focused spike before selecting one.
- Keep prompt dependencies behind a small interface and out of non-interactive
  startup paths.
- Record each dependency's purpose, version, alternatives, startup cost, and binary
  size impact.

The SQLite spike must cover native cross-builds, startup, size, CRUD, migrations,
locking, durability, and maintenance. Pure Go is desirable but not pre-approved.
The initial accepted driver and measured tradeoffs are recorded in
`docs/decisions/001-sqlite-driver.md`.
The accepted interactive prompt library, adapter constraints, and measured costs
are recorded in `docs/decisions/002-prompt-library.md`.

## Architecture

```text
main -> cli/app -> domain + prompt + output + repository -> config/platform
```

- `main` constructs dependencies, runs the app, and maps errors to an exit code.
  Do not call `os.Exit` below `main`.
- Parsing returns typed intent and does not open the database.
- Domain code validates without SQL or terminal knowledge.
- Prompt code collects input and never persists records.
- Repository code owns SQL, migrations, transactions, and decoding.
- Output code owns human and JSON representations.
- Inject streams, environment, clock, randomness, terminal detection, and data
  location through narrow dependencies.
- Use `context.Context` at application and repository boundaries.
- Prefer concrete internal types and small interfaces at real seams.

Suggested runner:

```go
type Runtime struct {
    Stdin  io.Reader
    Stdout io.Writer
    Stderr io.Writer
    Env    func(string) string
    Now    func() time.Time
    Random io.Reader
    IsTTY  func() bool
}

func Run(ctx context.Context, args []string, rt Runtime) error
```

## Performance design

- Handle help and version before database or prompt construction.
- Parse only the selected command.
- Embed the version at link time with a deterministic development fallback.
- Embed migrations and avoid unnecessary parsing.
- Open at most one logical database handle per command.
- Push filters, ordering, and limits into SQL.
- Add no caches, background processes, goroutines, or pools without end-to-end
  evidence.
- Measure release binaries, not just package microbenchmarks.

Track median, p90, binary size, and peak RSS for representative commands against
empty, small, and generated large databases according to
`docs/performance-contract.md`. Use a stable host for release results.

## Storage implementation

- Parameterize values and allow-list dynamic SQL fragments.
- Apply migrations in order inside transactions and record only successful ones.
- Make initialization idempotent and reject unknown newer migrations.
- Use a finite busy timeout and make lock failures actionable.
- Do not tune journal or synchronous modes without durability measurements.
- Preserve fields an operation does not intentionally update.
- Test initialization, migrations, round trips, malformed data, filtering,
  ordering, locking, rollback, permissions, ownership, and symlink rejection using
  temporary databases only.

Expected repository operations:

```text
Create(ctx, record)
Get(ctx, id)
List(ctx, filters)
UpdateStatus(ctx, id, status, updatedAt)
ListFrictionForReview(ctx)
```

## Interactive prompting

Define a small interface before selecting a terminal adapter:

```go
type Prompt interface {
    Select(ctx context.Context, label string, choices []string, defaultValue string) (string, error)
    Text(ctx context.Context, label string) (string, error)
    Confirm(ctx context.Context, label string, defaultValue bool) (bool, error)
}
```

- Keep prompt output away from JSON stdout.
- Distinguish cancellation, EOF, and ordinary failure.
- Restore terminal state after signals and errors.
- Open no database before prompts complete and are confirmed.
- Use fake prompts in app tests and a small pseudo-terminal integration suite.
- Quick and fully supplied paths must not initialize terminal UI code.

## Testing

Tests define the intended Go product, not parity with an archived executable.

- Unit-test domain values, normalization, IDs, time, escaping, parsing, errors, and
  deterministic formatting.
- Application-test every command's output streams, errors, state changes,
  cancellation, and read-free behavior.
- Repository-test real temporary SQLite files; never use a user's data directory.
- Use goldens selectively for intentionally designed Go help and output. Review
  changes as product changes and never regenerate them silently.
- Do not derive fixtures from Python 2.1.0.

Required checks:

```bash
gofmt -w <changed-go-files>
go vet ./...
go test ./...
go test -race ./...
go test -cover ./...
```

Add vulnerability scanning when dependencies exist and cross-build checks when
platform code or dependencies are introduced.

## Release design

- Build Linux AMD64 and macOS ARM64 artifacts first.
- Prefer `CGO_ENABLED=0` only if the storage driver passes all requirements.
- Inject the version at link time and use reproducible flags where practical.
- Package Bash, Zsh, and Fish completions.
- Publish checksums, provenance, tag validation, and smoke tests.
- Install the executable as `forge`.

The archived Python release process and artifacts are not part of this release.

## Sequential implementation plan

Complete and verify one phase before starting the next unless explicitly asked to
continue.

Current implementation status:

- Phases 0 through 3 are complete: product contracts, dependency risk spikes, the
  performance harness, data-free CLI shell, domain model, and presentation
  foundations are established.
- Phase 4, secure storage, is in progress; centralized read-only path resolution
  and secure data-directory and database-file opening are implemented. SQLite
  connection setup with required pragmas is implemented; transactional migrations
  and transactional capture and friction insertion are also implemented.
  Repository lookup by record ID and filtered newest-first listing are
  implemented; repository status updates are next.
- Phases 5 through 8 remain planned and unimplemented.
- The top-level help previews the approved command surface; it does not mean the
  data-backed commands are implemented.

### Phase 0: Product specification

- Confirm the command surface, terminology, defaults, output principles, exit-code
  categories, and JSON stability policy.
- Validate the approved typed-column schema and indexes before migration 001.
- Maintain the reproducible, user-data-free measurement contract and harness in
  `docs/performance-contract.md` and `scripts/benchmark.sh`.

Acceptance: open decisions are explicit, intended behavior is testable, and no step
depends on running Python.

### Phase 1: Risk spikes

- Evaluate SQLite and interactive prompt options outside production packages.
- Measure cross-builds, startup, size, CRUD, concurrency, cancellation, and terminal
  cleanup as applicable.
- Record decisions and remove spike-only code.

### Phase 2: Fast CLI shell

- Add the module, `cmd/forge`, runtime, exit handling, version injection,
  no-argument help, `--help`, and `--version`.
- Add no database or prompt code.
- Verify intentional goldens, no filesystem writes, and the 30 ms budget.

### Phase 3: Domain and presentation

- Implement approved values, validation, normalization, IDs, timestamps,
  terminal-safe output, and JSON schemas.
- Make clock and randomness failures testable.

### Phase 4: Secure storage

- Commit the first Go-owned schema.
- Implement paths, secure file handling, SQLite, migrations, and repositories.
- Verify failure safety and at most one logical handle per command.

### Phase 5: Non-interactive commands

Build small vertical slices in the order that best delivers a useful workflow.
Each slice covers human output, JSON where supported, errors, database state, and
latency. Capture-first may be more useful than repository-operation order.

### Phase 6: Interactive workflows

- Add prompting after non-interactive behavior is stable.
- Converge prompted and flag-driven inputs on the same domain functions.
- Test confirmation, cancellation, EOF, non-TTY behavior, and restoration.

### Phase 7: Packaging and release

- Add target builds, completions, checksums, provenance, smoke tests, and benchmark
  reporting.
- Validate archives on Linux AMD64 and macOS ARM64.

### Phase 8: Product hardening

- Review actual usage, simplify rough edges, stabilize intended public interfaces,
  and document future Go migration policy.
- Consider an explicit Python database importer only if users need it.

## Agent protocol

For each implementation task:

1. Read this blueprint and inspect the repository and worktree.
2. Preserve unrelated user changes.
3. State the single phase and coherent slice.
4. Identify product decisions, security invariants, and latency budgets affected.
5. Make the smallest coherent change without opportunistic refactors.
6. Add tests before claiming completion.
7. Run targeted and relevant full checks.
8. Benchmark startup, parsing, dependency, storage, or packaging changes.
9. Report commands, results, risks, and changed files.
10. Stop at the phase boundary unless asked to continue.

Do not use Python as a test oracle, touch real user data, treat historical behavior
as an implicit requirement, add unmeasured dependencies, weaken security, claim
end-to-end gains from microbenchmarks, combine unrelated changes, or silently
regenerate goldens.

## Definition of done

- Approved Forge workflows are implemented and documented.
- Tests express intended Go behavior across output, errors, and state.
- Local data is protected by tested filesystem and SQLite invariants.
- Quick, flag-driven, interactive, and cancellation paths are covered.
- Linux AMD64 and macOS ARM64 artifacts are built, checksummed, attested, and smoke
  tested.
- Warm help/version is at most 30 ms and small database commands at most 50 ms on
  the benchmark host.
- The project builds, tests, and releases without Python.
- The implementation remains small enough to understand in one sitting.

## Architectural principle

```text
intentional product design
-> thin startup path
-> explicit domain rules
-> secure local storage
-> deterministic output
-> measured performance
```

Forge succeeds when capturing useful thoughts and friction is faster than losing
them—and when the implementation is free to become the best version of that idea.
