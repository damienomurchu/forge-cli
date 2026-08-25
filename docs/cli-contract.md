# Forge CLI Contract

This document defines the initial public behavior of Forge's command-line shell.
It is an intentional contract for the Go project, not a record of behavior from the
archived Python implementation.

## Top-level invocation

Running `forge` without arguments prints the complete top-level help to stdout and
exits successfully.

```text
forge
forge -h
forge --help
```

All three invocations print the same top-level help and exit with status `0`.

## Command help

Every command and subcommand supports both `-h` and `--help`. Help may appear after
the command name or subcommand name and exits with status `0`.

Help is written to stdout. Requesting help takes precedence over requirements for
the command's positional arguments and flags.

## Version

`forge --version` writes the version to stdout in this form and exits with status
`0`:

```text
forge <version>
```

For example:

```text
forge 0.1.0
```

Release builds inject the version at link time. Development builds use the
deterministic fallback `dev`.

## Output streams

- Requested help and version output go to stdout.
- Normal command results go to stdout.
- Usage mistakes, validation failures, and operational errors go to stderr.
- An error must not duplicate its message across stdout and stderr.

## JSON output

JSON is an explicit machine-readable mode enabled with `--json` on commands that
support it. Commands use human-readable output by default.

- A successful command that returns one record emits one JSON object.
- A successful list or review command emits one JSON array, including `[]` when no
  records match.
- JSON stdout contains only the requested JSON value: no styling, progress,
  confirmation, labels, or other human commentary.
- Errors are concise text on stderr and produce no JSON on stdout. Structured JSON
  errors are not part of the initial interface.
- Successful JSON output is terminated by a newline.

Field names and JSON value types become public API when they are included in a
released version. Minor releases may add optional fields. Removing or renaming a
field, changing its type, or changing the top-level object/array shape requires a
major-version decision and release. Record fields and types are defined in
`docs/record-contract.md`.

## Exit statuses

Forge uses these initial exit-status categories:

| Status | Meaning |
|---:|---|
| `0` | Success, including requested help or version output |
| `1` | Operational or validation failure |
| `2` | Invalid command-line usage |
| `130` | Interrupted by the user |

A validation failure means that the command was understood but its requested value
or operation is invalid. A usage failure means that the command line could not be
parsed into a valid command, such as an unknown command, unknown flag, or missing
flag value.

## Filesystem isolation

The following paths complete without inspecting, creating, or modifying Forge
configuration, data directories, or databases:

- `forge`
- `forge -h`
- `forge --help`
- `forge --version`
- command and subcommand help
- command lines rejected during argument parsing

This guarantee keeps startup paths fast and ensures that asking for guidance or
making a typing mistake has no persistent side effects.

## Stability

The behaviors in this document are the contract for the first Go implementation.
Exact help wording and layout will be approved with intentional golden tests when
the fast CLI shell is implemented. They are not derived from Python 2.1.0.
