# Capture Command Contract

This document defines the initial product behavior of `forge capture` for the Go
implementation.

## Synopsis

```text
forge capture [--quick] [--project PROJECT] [--kind KIND] [--tags TAGS]
              [--json] DESCRIPTION
```

`DESCRIPTION` is one positional argument. Shell quoting is required when it
contains spaces. `--` ends option parsing, allowing a description that begins with
`-`:

```text
forge capture -- "- investigate startup cost"
```

Unknown flags, extra positional arguments, and a missing description are usage
errors with exit status `2`.

## Input normalization

- A description must contain non-whitespace text. Surrounding whitespace is
  removed; internal whitespace is preserved.
- `--project` is optional. Surrounding whitespace is removed, and an empty result
  is treated as absent.
- `--tags` is optional and accepts comma-separated values. Each tag is trimmed and
  lowercased. Empty tags are discarded, duplicates are removed, and first-seen
  order is preserved.
- `--kind` accepts one of `thought`, `idea`, `observation`, `question`, `decision`,
  or `seed`.
- An invalid description, project, tag, or kind is a validation failure with exit
  status `1`.

## Quick and interactive modes

`--quick` performs a non-interactive capture. When `--kind` is omitted, quick mode
uses `thought`. Explicit flags always override defaults. Optional project and tags
remain absent when omitted.

Without `--quick`, an omitted kind is selected interactively. Forge then asks for
confirmation before writing the record. If all values are supplied explicitly,
Forge still asks for confirmation unless `--quick` is present.

If interaction is required and stdin is not a terminal, Forge reports a validation
failure on stderr, exits `1`, and writes nothing. Declining confirmation exits `0`
and writes nothing. User interruption exits `130`, reports cancellation on stderr,
restores terminal state, and writes nothing.

Forge does not open or create its database until input is valid and any required
prompting and confirmation have completed.

## Results

A successful capture creates one record with type `capture` and initial status
`captured`.

Human mode writes this concise confirmation to stdout:

```text
Created capture <record-id>
```

With `--json`, success emits exactly one record object followed by a newline. It
obeys `docs/cli-contract.md`: no human commentary is mixed into stdout, and the
record follows `docs/record-contract.md`.

Database, filesystem, randomness, and other operational failures write a concise
error to stderr, emit nothing to stdout, and exit `1`.

## Relationship to listing

`list` is not a capture subcommand. Records are queried through the top-level
`forge list` command, so the literal description `list` needs no special syntax:

```text
forge capture --quick list
forge list --type capture
```
