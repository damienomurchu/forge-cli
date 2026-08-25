# Friction Command Contract

This document defines the initial product behavior of `forge friction` for the Go
implementation.

## Synopsis

```text
forge friction [--quick] [--project PROJECT] [--frequency FREQUENCY]
               [--impact IMPACT] [--category CATEGORY]
               [--current-workaround TEXT] [--json] DESCRIPTION
```

`DESCRIPTION` is one positional argument. Shell quoting is required when it
contains spaces. `--` ends option parsing, allowing a description that begins with
`-`:

```text
forge friction -- "- releases require too many manual steps"
```

Unknown flags, extra positional arguments, and a missing description are usage
errors with exit status `2`.

## Input normalization

- A description must contain non-whitespace text. Surrounding whitespace is
  removed; internal whitespace is preserved.
- `--project` and `--current-workaround` are optional. Surrounding whitespace is
  removed, and an empty result is treated as absent.
- `--frequency` accepts `daily`, `weekly`, `monthly`, `occasional`, or `unknown`.
- `--impact` accepts `low`, `medium`, `high`, or `unknown`.
- `--category` accepts `information-finding`, `repeated-action`,
  `context-switching`, `remembering`, `verification`, `waiting`, or `other`.
- Invalid descriptions or classification values are validation failures with exit
  status `1`.

## Quick and interactive modes

`--quick` performs a non-interactive friction capture. It supplies these defaults
for omitted classification flags:

```text
frequency: unknown
impact: unknown
category: other
```

Explicit flags always override quick defaults. Optional project and workaround
remain absent when omitted.

Without `--quick`, Forge interactively collects each omitted classification in this
order: frequency, impact, then category. All three classifications are required in
normal mode, although `unknown` remains a valid deliberate selection where offered.
Forge asks for confirmation after all values are available. Even when every value
is supplied explicitly, confirmation is required unless `--quick` is present.

If interaction is required and stdin is not a terminal, Forge reports a validation
failure on stderr, exits `1`, and writes nothing. Declining confirmation exits `0`
and writes nothing. User interruption exits `130`, reports cancellation on stderr,
restores terminal state, and writes nothing.

Forge does not open or create its database until input is valid and any required
prompting and confirmation have completed. Prompt rendering must not contaminate
JSON stdout.

## Results

A successful friction capture creates one record with type `friction` and initial
status `captured`.

Human mode writes a concise confirmation containing the generated record ID to
stdout. Exact wording will be approved with the implementation's golden test.

With `--json`, success emits exactly one record object followed by a newline. It
obeys `docs/cli-contract.md`: no human commentary is mixed into stdout, and the
record follows `docs/record-contract.md`.

Database, filesystem, randomness, and other operational failures write a concise
error to stderr, emit nothing to stdout, and exit `1`.

## Relationship to listing

`list` is not a friction subcommand. Friction records are queried through the
top-level list command:

```text
forge friction --quick "CI failures require manual log searching"
forge list --type friction
```
