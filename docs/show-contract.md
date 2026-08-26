# Show Command Contract

## Synopsis

```text
forge show RECORD_ID [--json]
```

Exactly one record ID is required. A missing ID, extra positional argument,
unknown flag, duplicate `--json`, or value supplied to `--json` is a usage error.

The ID is opaque. Parsing does not depend on a prefix, length, capture type, or
timestamp. It must contain non-whitespace text, contain no control characters, and
is not silently trimmed. A lexical violation is a validation failure and performs
no storage lookup.

When found, human mode writes every user-visible common and type-specific field in
a terminal-safe layout protected by intentional golden tests. JSON emits one
complete record object following `docs/record-contract.md`.

A missing ID writes a concise not-found error to stderr, emits nothing to stdout,
and exits `1`. Database, filesystem, and malformed stored-data failures use the
same stream and exit category without being mislabeled as not found.

Show is strictly read-only. It does not initialize or migrate storage, change
timestamps, or alter type-specific details.

Implementation status: the existing `parseShow` boundary already satisfies the
reset syntax and opaque-ID validation rules. The live handler uses `executeShow`
with the capture repository and shared human or JSON renderers while retaining
storage discovery, session setup, and user-facing
not-found presentation.
