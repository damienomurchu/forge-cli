# SQLite Storage Contract

This document defines the initial persistence model for the Go implementation.
SQLite storage follows the product model in `docs/record-contract.md`; it is not a
continuation of the archived Python database schema.

## Decision

Forge uses one `records` table with typed nullable columns for type-specific data
and one normalized `record_tags` table for ordered capture tags. It does not store
type-specific fields in opaque metadata JSON.

This design keeps the shared list and review queries simple, allows SQLite to
validate domain values, preserves tag order without encoding arrays into text, and
makes future migrations explicit. Nullable type-specific columns are acceptable
because a table constraint enforces the valid column set for each record type.

## Migration 001

The first migration creates the following schema. The checked-in SQL migration is
the executable source of truth once implementation begins; this contract must stay
synchronized with it.

```sql
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    applied_at TEXT NOT NULL
);

CREATE TABLE records (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL CHECK (type IN ('capture', 'friction')),
    description TEXT NOT NULL CHECK (length(description) > 0),
    project TEXT CHECK (
        project IS NULL OR (length(project) > 0 AND project = trim(project))
    ),
    status TEXT NOT NULL CHECK (
        status IN ('captured', 'reviewing', 'candidate', 'automated', 'dismissed')
    ),

    capture_kind TEXT CHECK (
        capture_kind IS NULL OR capture_kind IN (
            'thought', 'idea', 'observation', 'question', 'decision', 'seed'
        )
    ),
    friction_frequency TEXT CHECK (
        friction_frequency IS NULL OR friction_frequency IN (
            'daily', 'weekly', 'monthly', 'occasional', 'unknown'
        )
    ),
    friction_impact TEXT CHECK (
        friction_impact IS NULL OR friction_impact IN (
            'low', 'medium', 'high', 'unknown'
        )
    ),
    friction_category TEXT CHECK (
        friction_category IS NULL OR friction_category IN (
            'information-finding', 'repeated-action', 'context-switching',
            'remembering', 'verification', 'waiting', 'other'
        )
    ),
    current_workaround TEXT CHECK (
        current_workaround IS NULL OR (
            length(current_workaround) > 0
            AND current_workaround = trim(current_workaround)
        )
    ),

    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,

    CHECK (
        (type = 'capture'
            AND capture_kind IS NOT NULL
            AND friction_frequency IS NULL
            AND friction_impact IS NULL
            AND friction_category IS NULL
            AND current_workaround IS NULL)
        OR
        (type = 'friction'
            AND capture_kind IS NULL
            AND friction_frequency IS NOT NULL
            AND friction_impact IS NOT NULL
            AND friction_category IS NOT NULL)
    ),
    CHECK (
        (type = 'capture'
            AND length(id) = 36
            AND substr(id, 1, 4) = 'cap_'
            AND substr(id, 5) NOT GLOB '*[^0-9a-f]*')
        OR
        (type = 'friction'
            AND length(id) = 36
            AND substr(id, 1, 4) = 'frc_'
            AND substr(id, 5) NOT GLOB '*[^0-9a-f]*')
    )
);

CREATE TABLE record_tags (
    record_id TEXT NOT NULL REFERENCES records(id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    tag TEXT NOT NULL CHECK (
        length(tag) > 0 AND tag = trim(tag) AND tag = lower(tag)
    ),
    PRIMARY KEY (record_id, position),
    UNIQUE (record_id, tag)
);

CREATE TRIGGER records_type_immutable
BEFORE UPDATE OF type ON records
WHEN NEW.type <> OLD.type
BEGIN
    SELECT RAISE(ABORT, 'record type is immutable');
END;

CREATE TRIGGER record_tags_capture_only_insert
BEFORE INSERT ON record_tags
WHEN NOT EXISTS (
    SELECT 1 FROM records WHERE id = NEW.record_id AND type = 'capture'
)
BEGIN
    SELECT RAISE(ABORT, 'tags require a capture record');
END;

CREATE TRIGGER record_tags_capture_only_update
BEFORE UPDATE OF record_id ON record_tags
WHEN NOT EXISTS (
    SELECT 1 FROM records WHERE id = NEW.record_id AND type = 'capture'
)
BEGIN
    SELECT RAISE(ABORT, 'tags require a capture record');
END;

CREATE INDEX idx_records_created
    ON records(created_at DESC, id DESC);
CREATE INDEX idx_records_type_created
    ON records(type, created_at DESC, id DESC);
CREATE INDEX idx_records_status_created
    ON records(status, created_at DESC, id DESC);
CREATE INDEX idx_records_project_created
    ON records(project, created_at DESC, id DESC);
```

Go domain validation remains responsible for full Unicode whitespace and case
normalization and exact timestamp parsing. Database constraints provide defense in
depth and protect the finite vocabulary and type-specific shape.

## Tags

Tag positions are contiguous zero-based integers assigned after normalization.
Queries load tags ordered by `position ASC`. Creating a capture inserts its record
and all tags in one transaction. Friction records can never have tag rows.

## Migrations

- Migration files use zero-padded numeric versions, beginning with
  `001_initial.sql`.
- Apply pending migrations in numeric order inside transactions.
- Insert the `schema_migrations` row only after that migration's statements
  succeed.
- A failed migration rolls back completely and preserves the prior database.
- An unknown migration version newer than the executable supports is an
  operational compatibility error; Forge never attempts a downgrade.
- Released migration files are immutable. Schema changes use new migrations.
- `PRAGMA foreign_keys = ON` is required on every database connection.

The production migration registry currently embeds and applies
`001_initial.sql`. Schema inspection is separate and read-only: an empty database
reports version `0` with a migration required, while a database newer than the
executable is rejected without modification.

The initial driver decision in `docs/decisions/001-sqlite-driver.md` sets a 250 ms
SQLite busy timeout. Application tests will confirm the resulting error remains
actionable under real command execution. Journal and synchronous modes retain
SQLite defaults until durability measurements justify a change.

## Command transactions

- Capture and friction creation use one transaction for the record and related
  tags.
- A changed status and its `updated_at` value are written atomically.
- An unchanged status performs no update.
- Validation and prompt confirmation complete before a write transaction begins.
- Constraint, lock, cancellation, clock, and randomness failures leave no partial
  record or update.

Transactional capture insertion is implemented: the validated record row and all
ordered tag rows commit together, and any record or tag failure rolls back the
entire capture.

Transactional friction insertion is implemented: all shared and friction-specific
values commit in one record row, capture-only values remain null, no tags are
created, and any failure rolls back the operation.

Read-only lookup by ID is implemented for both record types. It reconstructs the
domain record, loads capture tags by ascending position, distinguishes missing
records from operational failures, and rejects malformed stored values or tag
positions.

Read-only listing is implemented with deterministic
`created_at DESC, id DESC` ordering. The primary result set is closed before tag
hydration to preserve the single-connection limit, empty results are represented
by a non-null empty collection, and one malformed row fails the whole operation.
Optional type, project, and status filters combine with AND semantics, and an
optional positive limit is applied after filtering and deterministic ordering.

Status updates read and validate the complete record inside one transaction. A
real change updates only `status` and `updated_at` atomically; a request for the
stored status performs no write and preserves the original timestamp. Missing IDs,
invalid timestamps, and database failures leave the record unchanged.

## Initialization and read-only commands

Only commands that create records may create the data directory, database, or
apply migrations automatically. `show`, `list`, and `review` preserve their strict
read-only guarantees:

- With no database, `list` and `review` return their successful empty results.
- With no database, `show` and `update` return not-found without creating one.
- When an existing database requires a migration, a read-only command returns an
  actionable operational error instead of applying it.

The update command may open an existing writable database but does not initialize
a missing database or apply migrations. A future explicit migration command may be
added if automatic migration during record creation proves insufficient.

## Index policy

The initial indexes correspond to newest-first listing and individual type, status,
and project filters. Query-plan testing with mixed friction statuses showed that
review uses `idx_records_type_created` to preserve result order; the candidate
`(type, status, created_at, id)` index was redundant and is intentionally omitted.
Add no further index without a measured query need; later index changes require a
migration.

## Security and ownership

Path resolution, ownership, permissions, regular-file checks, symlink rejection,
and no-follow opening follow `docs/blueprint.md`. Tests use temporary directories
and databases exclusively and never inspect or mutate real Forge user data.

Data-directory preparation creates only the final path component with mode `0700`;
its parent must already exist. It rejects symbolic links and non-directories, opens
the directory without following links, verifies effective-user ownership before
correcting permissions to `0700`, and returns the verified open handle for later
descriptor-relative database operations.

Database files are opened relative to that verified directory handle and without
following links. Read-only and existing read-write modes never create a missing
file; create mode atomically opens or creates it with mode `0600`. Every mode
rejects symbolic links and non-regular files, verifies effective-user ownership
before correcting permissions to `0600`, and returns a verified open handle.

SQLite reopens only an already-created, verified database in `ro` or `rw` mode;
it is never allowed to create the file itself. Forge brackets the driver's path
open with descriptor-relative identity checks so a changed file is rejected. Each
handle permits at most one physical connection at a time and configures foreign
keys plus the `250 ms` busy timeout on every connection.

Importing the archived Python database is outside this schema. If demand exists,
provide an explicit importer that validates and converts records into this model.
