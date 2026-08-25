# Update Command Contract

This document defines the initial product behavior of `forge update` for the Go
implementation.

## Synopsis

```text
forge update [--json] RECORD_ID --status STATUS
```

Exactly one record ID and one status are required. Flags may appear before or after
the record ID. A missing ID or status, extra positional arguments, duplicate flags,
an unknown flag, or a missing flag value is a usage error with exit status `2`.

## Record ID and status

`RECORD_ID` follows the opaque identifier rules in `docs/show-contract.md`: it must
contain non-whitespace text, contain no control characters, and is not trimmed or
interpreted by the command parser. An ID that violates these rules is a validation
failure with exit status `1` and causes no database access.

`STATUS` accepts one of:

```text
captured
reviewing
candidate
automated
dismissed
```

Forge does not impose a transition graph. Any accepted status may replace any
other accepted status for either record type. An invalid status is a validation
failure with exit status `1` and writes nothing.

## Update behavior

When the requested status differs from the stored status, Forge changes only the
record's status and `updated_at` timestamp. Description, type, project,
type-specific details, creation time, ID, and all other fields are preserved. The
status change and timestamp change occur atomically.

Setting a record to its current status is a successful no-op. It does not change
`updated_at` or any other stored value. The command still returns the existing
record as its successful result.

If no record has the supplied ID, Forge writes a concise not-found error to stderr,
emits nothing to stdout, exits `1`, and changes no state. Validation, database,
filesystem, clock, and stored-data failures also leave the record unchanged.

## Results

Human mode writes a concise confirmation containing the record ID and resulting
status to stdout. Exact wording will be approved with an implementation golden
test.

With `--json`, success emits exactly one complete resulting record object followed
by a newline. This applies to both a changed status and a successful no-op. Output
obeys the stability and stream rules in `docs/cli-contract.md`.

Errors emit nothing to stdout, write concise text to stderr, and use the exit-status
categories in `docs/cli-contract.md`.
