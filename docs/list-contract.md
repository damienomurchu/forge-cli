# List Command Contract

This document defines the initial product behavior of the top-level record list.

## Synopsis

```text
forge list [--limit N] [--type TYPE] [--project PROJECT] [--status STATUS]
           [--json]
```

`forge list` replaces type-specific nested commands such as `forge capture list`
and `forge friction list`. Those nested forms are not aliases.

## Filters

With no filters, the command returns records of every type. Filters combine with
AND semantics:

- `--type` accepts `capture` or `friction`.
- `--project` matches the normalized project value.
- `--status` accepts an approved record status.
- `--limit` accepts a positive integer and restricts the number of returned
  records after filtering and ordering.

Unknown flags, positional arguments, and missing flag values are usage errors with
exit status `2`. Invalid types or statuses, empty project filters, and non-positive
limits are validation failures with exit status `1`.

## Ordering and output

Results are ordered newest first with a deterministic tie-breaker. Human mode emits
a readable list and succeeds without special empty-state errors. Exact human layout
will be approved with implementation goldens.

With `--json`, the command emits one array followed by a newline, including `[]`
when no records match. It emits no human commentary on stdout.

Listing is read-only. A successful list does not alter records or database state.
