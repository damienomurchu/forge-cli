# ADR 001: SQLite Driver

- Status: accepted
- Date: 2026-08-25
- Driver: `modernc.org/sqlite` v1.57.0

## Context

Forge needs SQLite through `database/sql`, transactional migrations, and native
Linux AMD64 and macOS ARM64 builds. Avoiding CGO simplifies cross-compilation and
release packaging, provided runtime performance and dependency maintenance remain
acceptable.

## Decision

Use `modernc.org/sqlite` behind the repository boundary. Configure each short-lived
handle with foreign keys enabled, a 250 ms busy timeout, and one open connection.

## Consequences

- Release builds remain CGO-free for the supported targets.
- The pure-Go driver increases binary size and clean-build resource use.
- Driver-specific types must not escape the repository boundary.
- Race tests and native release-host smoke tests remain part of release confidence.
- Reconsider the driver if command benchmarks miss their budgets, binary size
  becomes a release constraint, or maintenance and vulnerability signals worsen.

`github.com/mattn/go-sqlite3` is the first comparison candidate if CGO becomes an
acceptable packaging cost. Any replacement must be measured against the same
storage, durability, and performance contracts.
