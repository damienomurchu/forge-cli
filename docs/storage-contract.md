# SQLite Storage Contract

## Model

Forge stores every capture in one `records` table. `capture_type` is one of
`friction`, `action`, `follow-up`, or `decision` and discriminates explicit typed
columns; opaque metadata JSON is not used.

Every row stores an ID, capture type, normalized description, creation timestamp,
and update timestamp. Friction rows additionally store optional project and current
workaround values plus required frequency, impact, and category classifications.
Non-friction rows must not contain friction values. Action, follow-up, and decision
need no placeholder columns while their details are empty.

There is no universal status column or lifecycle table. Future review requirements
must define type-specific state and any required migration.

Checked-in migrations are the executable source of truth for exact SQL, constraints,
and indexes.

## Migrations

Released migrations are append-only and immutable. Migrations run transactionally,
record their version only after success, and roll back completely on failure. Forge
rejects unknown newer schema versions and never downgrades.

Migration 002 converts representable schema-1 friction rows to the current model
without changing their IDs, descriptions, typed values, or timestamps. It rejects
legacy captures, tags, and friction lifecycle states that have no lossless current
representation rather than silently discarding data.

Creation commands may apply required migrations. Read-only commands report when an
existing database requires migration and do not modify it.

## Repository behavior

The repository provides one creation path and shared find and list operations.
Creation writes the complete common and matching typed values atomically. Reads
validate the complete stored shape and fail on malformed data rather than returning
partially decoded records. List filters combine with AND semantics and preserve
deterministic ordering.

Repository operations use parameterized values. Any dynamic SQL identifiers or
clauses must be selected from an allow-list.

## Connections and transactions

- Enable `PRAGMA foreign_keys = ON` on every connection.
- Use the accepted 250 ms busy timeout.
- Open at most one logical database handle per command.
- Validate and confirm before beginning a write transaction.
- Clock, randomness, constraint, lock, cancellation, and migration failures leave
  no partial record or schema change.
- Do not change journal or synchronous modes without durability evidence.

## Filesystem security

Default database locations are:

```text
Linux: $XDG_DATA_HOME/forge/forge.db
       falling back to ~/.local/share/forge/forge.db
macOS: ~/Library/Application Support/forge/forge.db
override: $FORGE_DATA_DIR/forge.db
```

Configured base paths must be absolute, and path resolution must not touch the
filesystem. Create the data directory lazily with mode `0700`. Require directory
and database ownership by the effective user, reject symbolic links, require a
regular database file, and create or correct database mode `0600`. Use
no-follow/open-and-inspect facilities where supported to avoid races.

Read commands open only existing storage. Missing storage is an empty result for
list and not found for show; it must not create directories, databases, or schema.
