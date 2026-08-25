# Show Command Contract

This document defines the initial product behavior of `forge show` for the Go
implementation.

## Synopsis

```text
forge show [--json] RECORD_ID
```

Exactly one record ID is required. A missing ID, extra positional arguments, an
unknown flag, or a missing flag value is a usage error with exit status `2`.

## Record IDs

The command treats `RECORD_ID` as an opaque identifier. Command parsing does not
depend on a record type prefix, timestamp, length, or other internal ID structure.
This keeps lookup independent from the generation scheme selected with the domain
model.

An ID must contain non-whitespace text and must not contain control characters.
Surrounding whitespace is not silently removed because it could turn a mistyped ID
into a different valid identifier. An ID that violates these lexical rules is a
validation failure with exit status `1`; Forge performs no database lookup.

## Results

When the record exists, human mode writes the complete record to stdout. The
presentation includes all user-visible fields applicable to that record type.
Exact layout will be approved with the implementation's golden tests.

With `--json`, success emits exactly one complete record object followed by a
newline. It obeys the stability and stream rules in `docs/cli-contract.md`. Exact
field names and types will be approved with the domain model before release.

If no record has the supplied ID, Forge writes a concise not-found error to stderr,
emits nothing to stdout, and exits `1`. Database, filesystem, and stored-data
failures follow the same stream and exit-status behavior without being described as
not-found errors.

## Read-only guarantee

`forge show` is strictly read-only. It does not modify the record, timestamps,
metadata, status, migrations, or other application state. Any database
initialization policy for read commands must preserve this guarantee and will be
decided with the storage contract.
