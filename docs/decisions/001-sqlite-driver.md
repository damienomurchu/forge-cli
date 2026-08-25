# ADR 001: Initial SQLite Driver

- Status: accepted for initial implementation
- Date: 2026-08-25
- Driver: `modernc.org/sqlite` v1.57.0
- Go: 1.27.0

## Context

Forge needs SQLite through `database/sql`, native Linux AMD64 and macOS ARM64
builds, secure short-lived connections, transactional migrations, and database
commands with a warm median below 50 ms. The Go standard library has no SQLite
driver.

Avoiding CGO materially simplifies cross-compilation and release packaging, but a
pure-Go driver is acceptable only if its behavior, runtime cost, build cost, and
dependency graph are understood.

## Decision

Use `modernc.org/sqlite` v1.57.0 for the initial implementation behind the
repository boundary. Configure each short-lived handle with foreign keys enabled,
a 250 ms busy timeout, and one open connection.

The driver remains replaceable at the repository boundary. Revisit this decision
if production CLI measurements miss their budgets, binary size becomes a release
constraint, or maintenance and vulnerability signals deteriorate.

## Evidence

The disposable spike in `spikes/sqlite` executes the checked-in
`migrations/001_initial.sql` against temporary real database files. Tests cover:

- valid and idempotent migration 001
- foreign-key activation on each connection
- capture/friction CRUD and ordered tags
- type, ID, status, tag, and type-specific constraints
- atomic status changes and true no-op updates
- rollback after a partial capture failure
- rejection of a newer migration version
- finite lock contention behavior
- query plans for review and project-filtered lists

On the Linux AMD64 development host, using Go 1.27.0:

| Measurement | Result |
|---|---:|
| Linux AMD64 stripped spike binary | 6,566,048 bytes |
| macOS ARM64 stripped spike binary | 6,619,586 bytes |
| Linux open/ping median, 30 warm runs | 5.700 ms |
| Linux open/ping p90, 30 warm runs | 6.641 ms |
| Linux open/ping peak RSS | 6,088 KiB |
| Clean Linux build time | 35.72 s |
| Clean Linux build peak RSS | 734,416 KiB |
| Spike package statement coverage | 80.4% |

The startup sample used Bash's microsecond clock because `hyperfine` was unavailable
on the host. It is diagnostic evidence, not a release benchmark. The project
harness remains authoritative for release measurements.

Both target binaries build with:

```text
CGO_ENABLED=0 GOOS=linux GOARCH=amd64
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64
```

The Linux artifact was executed successfully. The macOS artifact was inspected as
a Mach-O ARM64 binary but cannot be executed on this Linux host.

Query-plan testing produced one schema change: SQLite used
`idx_records_type_created` for actionable-friction review even with 500 mixed-status
rows and `ANALYZE`. The proposed `(type, status, created_at, id)` review index was
therefore redundant and was removed from migration 001 before release.

## Costs and risks

- The stripped spike adds roughly 6.6 MB before the rest of Forge is implemented.
- Clean compilation is CPU- and memory-intensive.
- The runtime module graph includes nine indirect modules in `go.mod`.
- A native macOS runtime smoke test remains required in CI.
- Race testing could not run on this host because no C compiler is installed; it
  remains required in CI even though release builds use no CGO.
- Lock errors still need application-level classification and actionable wording.

## Alternatives

`github.com/mattn/go-sqlite3` is mature but requires CGO and a target C toolchain,
which complicates the initial release matrix. It is the first comparison candidate
if the accepted driver's costs become unacceptable. Other maintained pure-Go
drivers may also be spiked against the same migration and measurements rather than
selected by feature claims alone.

## Reproduction

Core checks:

```bash
go test ./...
go vet ./...
go test -cover ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./spikes/sqlite/cmd/sqlite-spike
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./spikes/sqlite/cmd/sqlite-spike
```

All tests use `t.TempDir`; the spike never resolves or opens the user's Forge data
directory. Remove the disposable spike when its coverage has moved to the
production repository implementation.
