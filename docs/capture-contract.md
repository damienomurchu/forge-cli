# Capture Command Contract

Forge uses one capture entry point for all captured work.

## Synopsis

```text
forge capture [--json] DESCRIPTION
forge capture --quick --type TYPE [type-specific options] [--json] DESCRIPTION
```

`DESCRIPTION` is exactly one positional argument. `--` ends option parsing so a
description may begin with `-`. Unknown flags, extra positional arguments, and a
missing description or flag value are usage errors with exit status `2`.

Supported capture types are `friction`, `action`, `follow-up`, and `decision`.

## Common normalization

- Description must contain non-whitespace text. Surrounding whitespace is removed
  and internal whitespace is preserved.
- Optional text is trimmed and becomes absent when empty.
- Invalid domain values are validation failures with exit status `1`.

## Interactive capture

Without `--quick`, Forge:

1. validates the description;
2. requires stdin to be a terminal;
3. prompts for one capture type;
4. prompts only for fields belonging to that type;
5. displays a terminal-safe summary of the proposed capture;
6. requires confirmation; and
7. only then generates metadata, opens storage, and persists atomically.

The type prompt offers the four types in contract order and does not infer a type
from the description. `--type` is reserved for quick mode so interactive capture
always exercises explicit type selection.

Declining confirmation exits `0` and writes nothing. Cancellation exits `130`,
reports cancellation, restores terminal state, and writes nothing. EOF is a
distinct operational failure. Non-terminal interactive use exits `1` and writes
nothing.

## Quick capture

`--quick` performs no prompting and requires `--type TYPE`. Omitting `--type` is a
usage error. Forge never supplies a default type or infers one. An unsupported type
is a validation failure.

Quick and interactive capture use the same finalized domain input, validation,
record construction, atomic repository operation, and result rendering. Quick mode
must not construct prompt machinery.

## Friction details

Friction supports:

```text
--project PROJECT
--frequency FREQUENCY
--impact IMPACT
--category CATEGORY
--current-workaround TEXT
```

Approved values are:

```text
frequency: daily, weekly, monthly, occasional, unknown
impact: low, medium, high, unknown
category: information-finding, repeated-action, context-switching, remembering,
          verification, waiting, other
```

Interactive friction collects project, frequency, impact, category, and current
workaround. Optional project and workaround may remain absent. Classification
defaults are `unknown`, `unknown`, and `other`.

Quick friction uses the same classification defaults when flags are omitted.
Explicit flags override defaults. Friction-only flags supplied for another type
are usage errors rather than silently ignored input.

## Other typed details

Action, follow-up, and decision initially have no additional fields. Their details
remain explicitly typed in the domain and JSON model but are empty. Do not add
lifecycle or review metadata until approved review requirements define it.

## Result

Success creates one capture. Human mode prints:

```text
Created <type> capture <record-id>
```

With `--json`, success emits one complete record object followed by a newline and
no human success text. Operational failures emit nothing to stdout, write a
concise error to stderr, and exit `1`.
