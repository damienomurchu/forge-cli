# Forge CLI: Go Rewrite Blueprint

## Purpose

Rewrite Forge in Go to reduce command latency while preserving the behavior, data,
security properties, and release experience of Forge 2.1.0.

This is an implementation rewrite, not a product redesign. The Go executable must
be a drop-in replacement for the Python executable and must operate safely on the
same SQLite database. New features, schema changes, and output redesigns are out of
scope until parity is complete.

The rewrite should optimize this loop:

```text
invoke command -> parse only what is needed -> open data only when needed
               -> perform one operation -> write output -> exit
```

## Primary goals

1. Preserve the current command-line and storage contracts.
2. Reduce warm `--version` and `--help` latency from roughly 240 ms to at most 30 ms.
3. Keep warm database-backed commands under 50 ms for a small local database.
4. Produce native Linux AMD64 and macOS ARM64 executables without requiring a
   Python runtime.
5. Keep the codebase small, explicit, testable, and easy for an AI agent to modify.
6. Replace the Python implementation only after automated parity checks pass.

Latency targets are measured wall-clock medians over at least 30 warm invocations on
an otherwise idle machine. They are budgets, not permission to weaken validation,
durability, or filesystem security.

## Non-goals

- Changing commands, flags, defaults, exit codes, or stored values.
- Changing the existing database schema during the parity phase.
- Importing Python code or invoking Python from Go.
- Adding a daemon solely to hide startup time.
- Adding a web UI, API, synchronization, plugins, AI features, or configuration
  framework.
- Introducing an ORM, dependency-injection framework, or general command framework.
- Supporting downgrade from a database migrated by a future Forge release.
- Optimizing hypothetical workloads before measuring them.

## Sources of truth and conflict resolution

Use these sources in this order:

1. Tests that encode released Forge 2.1.0 behavior.
2. The command and compatibility contracts in this document.
3. `docs/blueprint.md` and `README.md`.
4. The existing Python implementation.

If these disagree, stop and record the discrepancy before changing behavior. Do not
silently choose the easiest implementation. A compatibility fixture may preserve a
released quirk even when a cleaner design is possible.

## Confirmed compatibility contract

### Command surface

```text
forge
forge --help
forge -h
forge --version

forge capture [--quick] [--project PROJECT] [--kind KIND] [--tags TAGS]
              [--json] DESCRIPTION
forge capture list [--limit N] [--project PROJECT] [--status STATUS] [--json]

forge friction [--quick] [--project PROJECT] [--frequency FREQUENCY]
               [--impact IMPACT] [--category CATEGORY]
               [--current-workaround TEXT] [--json] DESCRIPTION
forge friction list [--limit N] [--project PROJECT] [--status STATUS] [--json]

forge show RECORD_ID [--json]
forge update RECORD_ID --status STATUS
forge review [friction] [--json]
```

`forge capture list` and `forge friction list` are reserved typed-list commands. To
capture the literal description `list`, the user writes `forge capture -- list` or
`forge friction -- list`.

### Values

```text
record types: capture, friction
statuses: captured, reviewing, candidate, automated, dismissed
capture kinds: thought, idea, observation, question, decision, seed
frequencies: daily, weekly, monthly, occasional, unknown
impacts: low, medium, high, unknown
categories: information-finding, repeated-action, context-switching, remembering,
            verification, waiting, other
```

Enum spellings are storage and integration contracts.

### Input behavior

- Descriptions must contain non-whitespace text. Trim surrounding whitespace but
  preserve internal whitespace.
- Optional text values become absent when empty after trimming.
- Tags are split on commas, trimmed, lowercased, deduplicated, and stored in
  first-seen order.
- `--quick` skips prompts and supplies defaults only for omitted fields.
- Explicit flags always override quick-mode defaults.
- Outside quick mode, prompt only for missing required classification values and
  then ask for confirmation.
- If prompting is required and stdin is not an interactive terminal, fail
  immediately with a usage error.
- Declining confirmation exits successfully and writes nothing.
- Ctrl-C during prompting prints cancellation to stderr, writes nothing, and exits
  130.
- Do not create the data directory for help, version, or usage errors.

Quick defaults:

```text
capture kind: thought
friction frequency: unknown
friction impact: unknown
friction category: other
optional fields: absent/empty
status: captured
```

### Query behavior

- List filters compose with AND semantics.
- `--limit` must be a positive integer.
- Results are ordered by `created_at DESC, id DESC`.
- Review includes friction in `captured`, `reviewing`, and `candidate` status.
- Review excludes `automated` and `dismissed` records.
- Review is read-only.
- Status updates validate the new status but do not enforce a transition graph.
- A missing record is a command failure, not an empty success.

### Output and exit behavior

```text
0   success, including declined confirmation
1   operational, validation, stored-data, or not-found failure
2   command-line usage error
130 user cancellation via Ctrl-C
```

- Human output goes to stdout.
- Concise errors and cancellation notices go to stderr.
- JSON mode emits no human success text.
- Capture, friction, and show emit one JSON object.
- List and review emit one JSON array, including `[]` for no results.
- JSON field names and value types must remain compatible with Forge 2.1.0.
- Human output visibly escapes C0/C1 terminal controls, newlines, carriage returns,
  tabs, and Unicode bidirectional controls.
- JSON output preserves stored content using valid JSON escaping.
- Never print stack traces during normal command failures.

Golden tests must capture exact help, success, empty-result, error, human-record, and
JSON output before the Python implementation is removed.

## Existing storage contract

Forge continues to use the existing SQLite database without export or conversion.

Default locations:

```text
Linux: $XDG_DATA_HOME/forge/forge.db
       falling back to ~/.local/share/forge/forge.db
macOS: ~/Library/Application Support/forge/forge.db
override: $FORGE_DATA_DIR/forge.db
```

Schema:

```sql
CREATE TABLE records (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    description TEXT NOT NULL,
    project TEXT,
    status TEXT NOT NULL DEFAULT 'captured',
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_records_type ON records(type);
CREATE INDEX idx_records_status ON records(status);
CREATE INDEX idx_records_project ON records(project);
CREATE INDEX idx_records_created_at ON records(created_at);
```

The existing `schema_migrations` table and `001_initial.sql` migration name must be
recognized. Do not rename, reapply, or replace them.

Capture metadata:

```json
{"kind":"thought","tags":["forge","dashboard"]}
```

Friction metadata:

```json
{
  "frequency":"weekly",
  "impact":"medium",
  "category":"information-finding",
  "current_workaround":"Search repositories manually"
}
```

Timestamps are UTC RFC 3339 strings with six fractional digits and a `Z` suffix,
for example `2026-08-24T20:34:11.123456Z`. Preserve this exact format rather than
using Go's default `time.RFC3339Nano` formatting.

IDs retain the current shape:

```text
ca-YYYYMMDD-HHMMSS-microseconds-6hex
fr-YYYYMMDD-HHMMSS-microseconds-6hex
```

Generate the suffix with a cryptographically secure random source. If randomness
fails, return an error; do not silently substitute predictable data.

### Filesystem security invariants

- Resolve the data directory centrally.
- Create it lazily with mode `0700`.
- Require it to be a real directory owned by the effective user.
- Reject a symbolic link at the data-directory path.
- Require the database to be a regular file owned by the effective user.
- Reject a symbolic link at the database path.
- Create or correct the database mode to `0600`.
- Use no-follow/open-and-inspect facilities where the operating system supports
  them; avoid check-then-open races.
- Never weaken these checks merely to meet a latency target.
- Keep platform-specific filesystem operations behind a small internal interface so
  Linux, macOS, and test implementations can differ explicitly.

## Go technology decisions

### Toolchain

- Use the newest stable Go release supported by CI when the rewrite begins.
- Pin the intended toolchain in `go.mod`/the `toolchain` directive and CI.
- Support the two most recent stable Go release families unless a dependency forces
  a narrower range.
- Commit `go.mod` and `go.sum`.
- Use `gofmt`, `go vet`, and `go test` as the baseline toolchain.

### Dependencies

Prefer the standard library. Every third-party module must solve a concrete problem,
be pinned, and be recorded in the dependency decision table below.

Initial candidates:

| Concern | Candidate | Decision gate |
|---|---|---|
| SQLite | `modernc.org/sqlite` through `database/sql` | Accept only after cross-build, binary-size, startup, CRUD, migration, and concurrency spikes pass |
| Interactive prompts | `github.com/charmbracelet/huh` | Keep behind `Prompt` interface; accept only if arrow keys, `j`/`k`, cancellation, streams, and startup budgets pass |
| CLI parsing | standard library/custom command dispatch | Do not add Cobra unless measured complexity justifies its startup and dependency cost |
| IDs | `crypto/rand`, `encoding/hex`, `time` | Standard library |
| JSON | `encoding/json` | Standard library |

The pure-Go SQLite candidate avoids CGO and simplifies Linux/macOS builds. It is not
pre-approved solely for that reason: its compile time, binary size, cold-start cost,
SQLite behavior, and transitive dependency set must be measured. If it fails the
spike, document and compare a CGO driver or another maintained pure-Go driver before
changing direction.

Do not let prompt dependencies enter non-interactive packages. A non-interactive
command must not initialize terminal UI state.

## Target repository layout

Keep the Go rewrite alongside the Python implementation until parity is proven:

```text
.
├── cmd/
│   └── forge/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── app.go
│   │   └── commands.go
│   ├── cli/
│   │   ├── parse.go
│   │   ├── help.go
│   │   └── exit.go
│   ├── config/
│   │   └── paths.go
│   ├── domain/
│   │   ├── record.go
│   │   ├── values.go
│   │   └── normalize.go
│   ├── prompt/
│   │   ├── prompt.go
│   │   └── terminal.go
│   ├── repository/
│   │   ├── repository.go
│   │   ├── sqlite.go
│   │   └── migrations.go
│   ├── output/
│   │   ├── human.go
│   │   └── json.go
│   └── platform/
│       ├── files_unix.go
│       └── files_test.go
├── migrations/
│   └── 001_initial.sql
├── testdata/
│   ├── golden/
│   └── compatibility/
├── docs/
│   ├── blueprint.md
│   └── blueprint-go.md
├── go.mod
├── go.sum
└── Makefile
```

This is a boundary map, not a demand for tiny files. Merge files when that improves
clarity; do not merge the domain, repository, prompt, and presentation boundaries.

## Architecture and interfaces

Dependency direction:

```text
main -> cli/app -> domain + prompt + output + repository -> config/platform
```

Rules:

- `main` constructs dependencies, calls one runner, and maps its result to an exit
  code. Do not call `os.Exit` below `main`, because deferred cleanup and tests must
  work.
- CLI parsing identifies intent and returns typed command values. It does not open
  the database.
- Domain code validates and normalizes records without SQL or terminal knowledge.
- Prompt code collects missing values only. It does not persist records.
- Repository code owns SQL, migrations, transactions, and stored-data decoding.
- Output code owns stable human and JSON representations.
- Pass `io.Reader`, `io.Writer`, environment access, clock, random reader, and data
  location through narrow dependencies so tests do not use global process state.
- Use `context.Context` at application and repository boundaries; do not store a
  context in a struct.
- Prefer concrete types internally and small interfaces at test or boundary seams.
- Wrap errors with operation context and classify them with sentinel/typed errors
  for exit-code mapping. Do not branch on error strings.

Suggested central runner:

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

The actual interfaces may differ, but tests must be able to run the full CLI without
mutating `os.Args`, the real environment, the user's home directory, or global
stdin/stdout.

## Performance architecture

Performance is a first-class acceptance criterion.

- Handle `--version`, `--help`, and no-argument help before constructing database,
  prompt, or migration dependencies.
- Parse only the selected command. Do not construct a global graph of every command
  on each invocation.
- Embed the version at link time with `-ldflags`, with a deterministic development
  fallback. Never scan package metadata at runtime.
- Embed SQL migrations with `//go:embed`; inspect the current schema version before
  parsing unapplied migrations.
- Open at most one logical database handle per command.
- Push filtering, ordering, and limiting into SQL.
- Query review statuses in SQL rather than loading and discarding inactive records.
- Stream human list output where practical. JSON arrays may buffer if bounded and
  documented.
- Do not add caches, background processes, goroutines, or connection pools without
  evidence that they reduce end-to-end latency.
- Configure the short-lived SQLite handle deliberately (`SetMaxOpenConns(1)` is a
  reasonable starting point) and close it before returning.
- Measure rather than assume that stripping symbols, build flags, or dependency
  changes improve startup.

Required benchmark commands:

```text
forge --version
forge --help
forge capture --quick "benchmark"
forge capture list --limit 10
forge show <existing-id>
forge review --json
```

Record median, p90, executable size, and peak RSS for release builds. Run benchmarks
against an empty database, a small normal database, and a generated large database.
CI may enforce a generous regression ceiling; release evaluation should use a stable
host because shared CI timing is noisy.

## SQLite and migration requirements

- Use parameterized SQL exclusively for values.
- Allow-list any dynamic SQL fragments such as sort clauses.
- Apply pending migrations in order inside transactions.
- Record a migration only after all its statements succeed.
- Initialization must be idempotent.
- Detect unknown newer migrations and return a clear compatibility error rather
  than attempting a downgrade.
- Preserve the database on every failure path.
- Set a finite busy timeout suitable for a local CLI and translate lock errors into
  a concise actionable error.
- Do not change journal or synchronous modes without durability and compatibility
  tests.
- Decode metadata into type-specific structs and reject invalid stored JSON with an
  error that includes the record ID but not a stack trace.
- Preserve unknown metadata fields when an operation does not intentionally replace
  metadata. Status updates must never rewrite metadata.

Repository operations needed for parity:

```text
Create(ctx, record)
Get(ctx, id)
List(ctx, filters)
UpdateStatus(ctx, id, status, updatedAt)
ListFrictionForReview(ctx)
```

For list queries, use `EXPLAIN QUERY PLAN` with generated large fixtures before
adding composite indexes. Index changes require a new migration and therefore occur
after storage parity unless a measured scaling test justifies them.

## Interactive prompting

Define and test a small prompt interface before adapting a terminal library:

```go
type Prompt interface {
    Select(ctx context.Context, label string, choices []string, defaultValue string) (string, error)
    Text(ctx context.Context, label string) (string, error)
    Confirm(ctx context.Context, label string, defaultValue bool) (bool, error)
}
```

Requirements:

- Up/Down and `j`/`k` navigate select fields; Enter selects.
- Prompt output must not contaminate JSON stdout; use the same stream behavior as
  the released CLI and lock it with tests.
- Cancellation must be distinguishable from EOF and ordinary failure.
- Signal handling must restore terminal state.
- Do not open the database until all prompts are complete and confirmed.
- Supply a fake prompt in application tests.
- Keep a small pseudo-terminal integration suite for navigation, Ctrl-C, and
  terminal restoration.
- Assess accessible/fallback prompt behavior explicitly; do not assume a full-screen
  terminal renderer works for every terminal.

## Testing strategy

### Unit tests

Use table-driven tests for:

- every enum and invalid value
- description/project/workaround normalization
- tag normalization and stable deduplication
- timestamp and ID formatting
- terminal-safe escaping
- argument parsing, including option order and `--`
- error classification and exit-code mapping
- deterministic human and JSON formatting

### Repository tests

Use a temporary real SQLite file, not the user's data directory. Cover:

- fresh initialization and migration idempotency
- opening the Python-created schema from a fixture
- Go-created records read by the Python implementation during coexistence
- Python-created records read and updated by Go
- metadata round trips and malformed stored records
- combined filters, deterministic ordering, and positive limits
- status updates preserving metadata and changing `updated_at`
- review filtering in SQL
- duplicate IDs, missing records, lock contention, rollback, and newer-schema errors
- symlink, ownership where safely testable, regular-file, and permission checks

### Full CLI tests

Call `Run` with in-memory streams and a temporary data directory. Cover every command
in human and JSON modes, stdout/stderr separation, exit codes, prompt/no-prompt
selection, cancellation, and the guarantee that read-free commands create no files.

### Golden compatibility tests

Before implementing Go formatting, capture normalized outputs from Forge 2.1.0.
Normalize only inherently variable fields such as generated IDs, timestamps, and
temporary paths. Do not normalize whitespace, JSON structure, error prefixes, or
exit codes.

Maintain a command matrix containing:

- valid minimal and fully flagged invocations
- every help path and short `-h`
- every quick default and explicit override
- empty/whitespace descriptions and option values
- literal `list` with and without `--`
- missing TTY, declined confirmation, EOF, and Ctrl-C
- invalid enum, limit, option, command, and record ID
- empty and populated list/review results
- terminal-control and bidi-control content
- corrupted metadata and unavailable/unsafe data paths

Run the same cases against the Python and Go executables during coexistence and diff
exit code, stdout, stderr, and resulting database state.

### Quality and safety checks

Required local/CI checks:

```bash
gofmt -w <changed-go-files>
go vet ./...
go test ./...
go test -race ./...
go test -cover ./...
```

Also add dependency vulnerability scanning using the official Go vulnerability tool
and retain completion validation and standalone smoke tests. Pin CI actions and
module versions according to repository policy.

## Release design

- Build with `CGO_ENABLED=0` if the selected SQLite driver supports the required
  behavior; verify this rather than assuming it.
- Produce Linux AMD64 and macOS ARM64 artifacts first, matching existing support.
- Inject the release version at link time.
- Use reproducible build flags where practical and document any non-reproducible
  inputs.
- Package the executable with the existing Bash, Zsh, and Fish completions.
- Retain checksums, GitHub build-provenance attestations, tag/version validation, and
  smoke tests.
- Add a smoke test that opens a database created by Forge 2.1.0.
- Do not overwrite or delete the Python release artifacts during development.

The installed executable remains named `forge`; users should not need a data
migration or command change when upgrading.

## Sequential implementation plan

Each phase is independently reviewable. An AI agent must complete and verify one
phase before starting the next unless explicitly told to continue.

### Phase 0: Freeze compatibility fixtures

- Inventory current commands, tests, output, exit codes, database schema, migration
  records, archive contents, and completion files.
- Create the golden command matrix and Python-generated database fixtures.
- Add a benchmark script that reports median and p90 without modifying real data.

Acceptance criteria:

- The current Python suite remains green.
- Fixtures contain no private user data.
- Variable output normalization is narrow and documented.
- Baseline latency and artifact size are recorded.

### Phase 1: Spike risky dependencies

- Build minimal SQLite and prompt experiments outside production packages.
- Verify cross-compilation, startup latency, artifact size, schema compatibility,
  cancellation, and terminal cleanup.
- Record accepted versions and reasons in this document or an ADR.
- Remove spike-only code after decisions are made.

Acceptance criteria:

- SQLite can read and write a copied Forge 2.1.0 database.
- Linux AMD64 and macOS ARM64 release builds are feasible.
- The selected dependencies fit the startup and security budgets.

### Phase 2: Establish the Go executable and fast CLI shell

- Add the module, `cmd/forge`, runtime abstraction, exit handling, version injection,
  no-argument help, `--help`, and `--version`.
- Do not add database code yet.

Acceptance criteria:

- Help/version output and exit codes match golden fixtures.
- Help/version do not create the data directory.
- Release-build median help/version latency is at most 30 ms on the benchmark host.

### Phase 3: Domain values, validation, IDs, and output

- Implement typed values, record validation, normalization, exact timestamps/IDs,
  terminal-safe output, and canonical JSON.

Acceptance criteria:

- Table-driven unit tests cover valid and invalid boundaries.
- Golden record output matches after allowed ID/time normalization.
- Randomness and clock failures are testable.

### Phase 4: Secure paths, SQLite, and migrations

- Implement central path resolution, secure file opening, the driver adapter,
  migrations, and repository operations.

Acceptance criteria:

- Go opens and preserves Python-created databases.
- Python can read records created by Go.
- Security and migration failure tests pass without damaging fixtures.
- One command uses at most one logical database handle.

### Phase 5: Non-interactive commands

Implement in this order:

1. `show`
2. typed lists
3. `update`
4. `review`
5. quick/fully flagged `capture`
6. quick/fully flagged `friction`

Acceptance criteria:

- Each command passes its human, JSON, error, and database-state parity cases.
- Review filtering occurs in SQL.
- A small warm database command has median latency at most 50 ms on the benchmark
  host.

### Phase 6: Interactive capture and friction

- Add the prompt adapter only after non-interactive paths are stable.
- Converge prompted and flag-driven inputs on the same domain functions.

Acceptance criteria:

- Missing fields, confirmation, cancellation, EOF, non-TTY failure, and terminal
  restoration match the contract.
- No database file is opened before confirmation.
- Quick and fully supplied flag paths do not initialize terminal UI dependencies.

### Phase 7: Packaging and end-to-end parity

- Replace PyInstaller release jobs with Go builds while retaining artifact layout,
  completions, checksums, attestations, and smoke coverage.
- Run the full cross-implementation compatibility matrix.

Acceptance criteria:

- Both target artifacts pass smoke and compatibility tests.
- Release archives contain the expected paths.
- Version/tag checks work.
- Latency, binary size, and RSS results are documented.

### Phase 8: Cutover and cleanup

- Make Go the default development and release implementation.
- Remove Python code, packaging, lock files, and Python-only CI only after a tagged Go
  release candidate passes all parity gates.
- Preserve useful language-independent fixtures and documentation.

Acceptance criteria:

- No documented command requires Python.
- Existing databases open without conversion or backup restoration.
- The repository has one authoritative implementation.
- Rollback consists of reinstalling the prior binary, not reversing a schema change.

## AI agent working protocol

For every task, the agent must:

1. Read this blueprint and inspect the current repository state.
2. Check for uncommitted user changes and preserve them.
3. State the single phase/slice being implemented and the files expected to change.
4. Identify the compatibility cases and latency budget affected.
5. Make the smallest coherent change; avoid opportunistic refactors.
6. Add or update tests before claiming completion.
7. Run targeted tests, then the relevant full checks.
8. Benchmark when startup, parsing, dependencies, SQLite opening, or packaging changed.
9. Report exact commands run, results, remaining risks, and files changed.
10. Stop at the phase boundary unless the user explicitly requests continued work.

The agent must not:

- delete or overwrite the Python implementation before Phase 8
- read or mutate the user's real Forge database in tests or benchmarks
- change schema, JSON, human output, enum spelling, IDs, timestamps, exit codes, or
  defaults without an explicit compatibility decision
- use global process state where dependency injection makes deterministic tests easy
- add a dependency without documenting why the standard library is insufficient and
  measuring its startup/artifact impact
- weaken path ownership, symlink, permissions, transaction, or terminal-safety rules
- claim performance improvement from microbenchmarks alone
- combine a behavior change, storage migration, dependency replacement, and release
  rewrite in one review slice
- silently regenerate golden fixtures to make a failing test pass

When ambiguity remains, the agent should preserve released behavior, add a focused
test that exposes the ambiguity, and ask for a decision only if implementation cannot
proceed safely.

## Pull-request checklist

- [ ] Scope names one blueprint phase and one coherent slice.
- [ ] Existing user changes are preserved.
- [ ] Behavior changes are absent or explicitly approved.
- [ ] New dependencies have a documented decision and pinned version.
- [ ] Tests cover success, failure, stdout, stderr, exit code, and database state.
- [ ] No test touches the real home/data directory.
- [ ] `gofmt`, `go vet`, tests, race tests, and relevant security checks pass.
- [ ] Cross-platform build checks pass when platform code or dependencies changed.
- [ ] Compatibility fixtures pass when command/storage behavior changed.
- [ ] Benchmarks are reported when the startup path changed.
- [ ] Help/version remain data-directory-free.
- [ ] No partial write is possible after cancellation or failed validation.
- [ ] Documentation and completion definitions remain synchronized.

## Definition of done

The Go rewrite is complete when:

- Every documented Forge 2.1.0 command works through the Go executable.
- Golden compatibility tests match exit codes, stdout, stderr, JSON, and database
  state.
- Existing Python-created databases work without conversion.
- Quick, flag-driven, interactive, and cancellation paths are covered.
- Filesystem and terminal-output security invariants are preserved.
- Linux AMD64 and macOS ARM64 artifacts are built, attested, checksummed, and smoke
  tested.
- Warm help/version latency is at most 30 ms and small database commands are at most
  50 ms on the designated benchmark host.
- The Python implementation can be removed without losing a test oracle, fixture,
  behavior contract, migration, completion, or release capability.
- The resulting implementation remains small enough to understand in one sitting.

## Architectural principle

```text
compatibility first
-> thin startup path
-> explicit domain rules
-> one secure database operation
-> deterministic output
-> measured performance
```

The rewrite succeeds when Forge feels immediate while existing users, scripts, and
databases cannot tell that its implementation language changed.
