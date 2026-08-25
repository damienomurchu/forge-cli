# Review Command Contract

This document defines the initial product behavior of `forge review` for the Go
implementation.

## Synopsis

```text
forge review [--json]
```

`review` is a focused view of actionable friction. It takes no positional
arguments. An extra argument, including the historical `friction` qualifier, an
unknown flag, or a value supplied to `--json` is a usage error with exit status
`2`.

## Included records

The command includes records whose type is `friction` and whose status is one of:

```text
captured
reviewing
candidate
```

It excludes capture records and friction with `automated` or `dismissed` status.
Results are ordered newest first with the same deterministic tie-breaker used by
`forge list`.

This initial command intentionally has no filters or limit. `forge list` provides
general-purpose querying. Review-specific options should be introduced only when a
clear review workflow needs them.

## Results

Human mode writes a review-oriented presentation to stdout. It succeeds when no
records match and displays a concise empty state rather than treating that case as
an error. Exact populated and empty layouts will be approved with implementation
golden tests.

With `--json`, success emits one array followed by a newline, including `[]` when no
records match. The array contains complete friction record objects and obeys the
stability and stream rules in `docs/cli-contract.md`.

Database, filesystem, and stored-data failures emit nothing to stdout, write a
concise error to stderr, and exit `1`.

## Read-only guarantee

`forge review` is strictly read-only. It does not change status, timestamps,
type-specific details, migrations, or other application state. Filtering and
ordering occur in the repository query rather than by loading and discarding
unrelated records in the application layer.
