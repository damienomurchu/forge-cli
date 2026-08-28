# Forge CLI Contract

This document defines Forge's active command-line shell. It supersedes the earlier
separate `capture` and `friction` command design.

## Command surface

```text
forge
forge --help
forge --version
forge capture [capture options] DESCRIPTION
forge completion SHELL
forge delete RECORD_ID
forge list [--limit N] [--type TYPE] [--project PROJECT] [--json]
forge show [--json] RECORD_ID
```

Type-aware review is not yet specified or implemented. There is no top-level
`forge friction` command and no generic `forge update` command. They must not
appear in help as available commands.

## Help and version

Running `forge`, `forge -h`, or `forge --help` prints the same complete top-level
help to stdout and exits `0`. Every active command supports `-h` and `--help`.
Requested help takes precedence over missing positional arguments and flags.

`forge --version` prints `forge <version>` followed by a newline and exits `0`.
Release builds inject the version; development builds use `dev`.

Help, version, and command lines rejected during parsing do not inspect, create, or
modify Forge storage.

## Completion

`forge completion bash`, `forge completion fish`, and `forge completion zsh` emit
a sourceable completion script for the selected shell. The script is the only
stdout output and ends with a newline. Completion generation does not inspect or
create storage.

A missing shell, unsupported shell, extra positional argument, or unknown flag is
a usage error. `-h` and `--help` print command help without requiring a shell.

## Output streams

- Help, version, and normal command results go to stdout.
- Usage, validation, and operational errors go to stderr.
- An error is not duplicated across streams.
- Prompt rendering goes to the prompt stream, never JSON stdout.

## Terminal-safe human text

Human output never writes untrusted terminal controls or Unicode bidirectional
controls literally. Newline, carriage return, tab, and backslash use visible
escapes; byte controls and invalid UTF-8 use lowercase `\xhh`; other non-graphic
Unicode and bidirectional controls use lowercase `\uhhhh` or `\Uhhhhhhhh`.

## JSON output

- A capture or show result emits one complete record object.
- List emits one array, including `[]` when empty.
- JSON stdout contains only the value followed by a newline.
- Errors remain concise text on stderr and emit no JSON.

Schemas are defined in `docs/record-contract.md`. Adding an optional field is a
minor-version decision. Removing or renaming a field, changing its type, or
changing a top-level shape requires a major-version decision.

## Delete

```text
forge delete RECORD_ID
```

Exactly one record ID is required. Unknown flags and extra positionals are usage
errors. The ID uses the same opaque lexical validation as `show`: it must contain
non-whitespace text, have no surrounding whitespace or control characters, and is
not interpreted as a particular ID format.

Delete is permanent and does not prompt. It removes exactly one matching capture
atomically and prints:

```text
Deleted capture <record-id>
```

A missing ID reports a not-found error on stderr and emits nothing to stdout.
Missing storage is also treated as not found and is not created. Delete opens only
an existing, current database and does not apply migrations.

## Exit statuses

| Status | Meaning |
|---:|---|
| `0` | Success, including help, version, or declined confirmation |
| `1` | Operational or validation failure |
| `2` | Invalid command-line usage |
| `130` | Interrupted by the user |

Unknown commands and flags, extra positionals, missing required positionals, and
missing flag values are usage errors. An understood command with an invalid domain
value is a validation failure.

## List

```text
forge list [--limit N] [--type TYPE] [--project PROJECT] [--json]
```

With no filters, list returns every capture. Filters combine with AND semantics.
`--type` accepts one of the four capture types. `--project` matches normalized
friction projects and therefore returns only friction captures. `--limit` must be
positive and applies after filtering and ordering. Invalid filter values are
validation failures.

Results use `created_at DESC, id DESC`. Human mode emits one terminal-safe line per
capture with no heading:

```text
<id>  <capture-type>  <description>
```

An empty human result writes nothing. JSON emits a complete record array, including
`[]` when empty. List is read-only and treats missing storage as an empty result.

## Show

```text
forge show [--json] RECORD_ID
```

Exactly one record ID is required. Duplicate `--json`, a value supplied to it,
unknown flags, and extra positionals are usage errors.

The ID is opaque. It must contain non-whitespace text and no control characters and
is not silently trimmed. Lexical violations are validation failures and perform no
storage lookup.

Human mode writes every common and type-specific field in a terminal-safe layout.
JSON emits the record defined by `docs/record-contract.md`. A missing ID reports a
not-found error on stderr and emits nothing to stdout. Show is read-only, does not
modify or migrate storage, and treats missing storage as not found.
