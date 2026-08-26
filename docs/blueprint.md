# Forge CLI: Go Implementation Blueprint

## Purpose

Forge is a fast, private, local command-line tool for capturing work that should
not be lost. The product has one capture entry point and four explicit capture
types. The implementation should remain small, direct, secure, and easy to evolve.

The archived Python implementation and the earlier Go command model are historical
context only. They are not compatibility or behavioral specifications.

## Sources of truth

Use these sources in order:

1. The active product contracts in `docs/`.
2. This blueprint.
3. Tests encoding the active contracts.
4. User-facing documentation.
5. Earlier implementations and Git history, as context only.

When code and an active contract differ, do not preserve the code accidentally.
Implement the contract in a small coherent slice and update its tests.

## Product model

Forge has one creation command:

```text
forge capture <description>
forge capture --quick --type <type> <description>
```

Supported capture types are `friction`, `action`, `follow-up`, and `decision`.
There is no top-level `forge friction` command. Friction is a capture type, not a
separate record family or creation path.

Every record is a capture with common fields and exactly one typed-details shape.
The initial friction details are project, frequency, impact, category, and current
workaround. Action, follow-up, and decision begin with no additional persisted
metadata. Do not add speculative fields for future review workflows.

## Capture workflows

Interactive capture performs these steps in order:

1. Validate and normalize the description without opening storage.
2. Require a terminal and prompt for the capture type.
3. Prompt only for fields relevant to the selected type.
4. Display a terminal-safe summary of the complete proposed capture.
5. Require explicit confirmation.
6. Generate the ID and timestamps, open storage, and persist atomically.

Declining confirmation writes nothing and exits successfully. Cancellation or EOF
writes nothing. Prompt rendering must not contaminate JSON stdout. Storage must not
be inspected or created until all input is valid and confirmation succeeds.

Quick capture is non-interactive. `--type` is mandatory and Forge never infers it.
Quick and interactive modes converge on the same domain construction, validation,
repository, and output paths after input collection.

## Review workflows

The intended command family is:

```text
forge review friction
forge review action
forge review follow-up
forge review decision
```

Review will be type-aware and perform meaningful transitions rather than merely
list records. Product requirements, state machines, interaction, and persistence
fields are intentionally deferred. Do not retain a universal lifecycle, generic
status update command, or guessed transition before those contracts are approved.

The words execute, defer, delegate, complete, wait, chase, escalate, update,
close, clarify, record outcome and rationale, revisit, validate, and reverse are
workflow context only. They are not yet a schema or transition graph.

## Active command scope

The implemented command surface covers:

```text
forge
forge --help
forge --version
forge capture ...
forge list ...
forge show RECORD_ID
```

`forge review <type>` is reserved for later design. The old
`forge update RECORD_ID --status STATUS` and top-level `forge friction` commands
are not part of the new product surface.

## Domain rules

- Descriptions contain non-whitespace text. Trim surrounding whitespace and
  preserve internal whitespace.
- Optional text becomes absent when empty after trimming.
- Capture type is always one of the four approved values.
- Each record has exactly one details shape matching its capture type.
- Human output visibly escapes terminal controls and bidirectional controls.
- Machine output is stable, valid JSON and contains no human commentary.
- Lists use deterministic newest-first ordering.
- IDs are opaque at command boundaries.
- Lifecycle rules belong to the capture type that owns their meaning.

## Storage direction

Use SQLite with one shared records table and explicit typed columns. Migration 002
introduces `capture_type`, converts the old capture/friction split into the unified
capture model, and removes the schema's universal lifecycle assumption. Although
the current development database contains no records, the migration must still be
transactional and structurally safe for any valid migration-001 database.

Ordered tags from the old capture model are not part of the new product model.
Migration behavior is specified in `docs/storage-contract.md`; do not silently
discard data merely because the known development database is empty.

Storage invariants remain unchanged:

- Resolve paths centrally and create storage lazily.
- Require absolute configured base directories.
- Use directory mode `0700` and database mode `0600`.
- Require effective-user ownership and regular files.
- Reject symbolic links at the data-directory and database paths.
- Apply migrations transactionally and reject unknown newer schemas.
- Parameterize values and allow-list dynamic SQL.
- Open at most one logical database handle per command.

## Architecture

```text
main -> cli/app -> domain + prompt + output + repository -> config/platform
```

- Parsing returns typed intent and does not open storage.
- Domain code validates without SQL or terminal knowledge.
- Prompt code collects input and never persists.
- Repository code owns SQL, migrations, transactions, and decoding.
- Output code owns human and JSON representations.
- Inject streams, environment, clock, randomness, terminal detection, and data
  location through narrow dependencies.
- Do not call `os.Exit` below `main`.
- Keep prompt and SQLite dependencies behind their existing boundaries.

## Performance and security

Warm median budgets on the release benchmark host remain 30 ms for help/version
and 50 ms for commands against a small local database. Security, durability, and
validation are never weakened to meet latency. Measure complete release-binary
invocations according to `docs/performance-contract.md`.

## Revised implementation stages

Complete one coherent stage at a time. Do not continue into review design without
new approved requirements.

### Stage 1: Contract and model reset

- Replace the separate record-type model with four capture types.
- Remove obsolete friction-command and universal-update contracts.
- Define capture JSON and migration 002.

Acceptance: documentation is internally consistent and deferred decisions are
explicit. This stage changes no production code.

### Stage 2: Domain and presentation

- Add the four capture types and explicit typed details.
- Remove old capture-kind, tags, universal status, and separate friction-record
  assumptions.
- Implement proposed-capture summary output.
- Update JSON and human-output tests.

### Stage 3: Storage migration and repository

- Add and test migration 002.
- Move creation and decoding to one capture repository path.
- Preserve valid existing friction records through conversion.
- Remove universal status-update and old specialized-review repository behavior.

### Stage 4: Command reset

- Make quick `--type` mandatory.
- Implement interactive type selection, relevant prompts, summary, and
  confirmation.
- Move friction behavior beneath `forge capture --type friction`.
- Implement minimal action, follow-up, and decision capture.
- Remove top-level friction and update dispatch/help; update list and show.

### Stage 5: Cleanup and measurement

- Remove superseded code and fixtures only when unused.
- Run formatting, vet, unit, race, coverage, vulnerability, cross-build, and
  benchmark checks appropriate to the changes.
- Reconcile all user-facing documentation.

### Deferred stage: type-aware review

Wait for updated requirements. Then define each lifecycle, transition effects,
review UX, stored fields, JSON changes, and migration before implementation.

## Agent protocol

For each task:

1. Read this blueprint and relevant active contracts.
2. Inspect the repository and preserve unrelated changes.
3. State the single stage and coherent slice.
4. Identify product decisions, security invariants, and latency budgets affected.
5. Make the smallest coherent change without opportunistic refactors.
6. Add or update tests before claiming completion.
7. Run targeted and relevant full checks.
8. Benchmark startup, parsing, dependency, storage, or packaging changes.
9. Report commands, results, risks, and changed files.
10. Stop at the stage boundary unless asked to continue.

Do not use archived behavior as an oracle, touch real user data, invent review
semantics, preserve obsolete commands accidentally, weaken storage security,
combine unrelated stages, or silently regenerate goldens.
