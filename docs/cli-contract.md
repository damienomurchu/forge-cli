# Forge CLI Contract

This document defines Forge's active command-line shell. It supersedes the earlier
separate `capture` and `friction` command design.

## Command surface

```text
forge
forge --help
forge --version
forge capture [capture options] DESCRIPTION
forge list [--limit N] [--type TYPE] [--project PROJECT] [--json]
forge show RECORD_ID [--json]
```

The intended `forge review <type>` family is reserved but not yet specified or
implemented. There is no top-level `forge friction` command and no generic
`forge update` command. They must not appear in help as available commands.

## Help and version

Running `forge`, `forge -h`, or `forge --help` prints the same complete top-level
help to stdout and exits `0`. Every active command supports `-h` and `--help`.
Requested help takes precedence over missing positional arguments and flags.

`forge --version` prints `forge <version>` followed by a newline and exits `0`.
Release builds inject the version; development builds use `dev`.

Help, version, and command lines rejected during parsing do not inspect, create, or
modify Forge storage.

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
