# SQLite Storage Contract

This document defines the target persistence model for the unified capture design.
Migration 001 remains immutable history. Migration 002 performs the model reset.

## Target model

Forge uses one `records` table. Every row is a capture and `capture_type`
discriminates explicit typed columns. Opaque metadata JSON is not used.

The target schema is equivalent to:

```sql
CREATE TABLE records_v2 (
    id TEXT PRIMARY KEY,
    capture_type TEXT NOT NULL CHECK (
        capture_type IN ('friction', 'action', 'follow-up', 'decision')
    ),
    description TEXT NOT NULL CHECK (length(description) > 0),
    friction_project TEXT CHECK (
        friction_project IS NULL OR (
            length(friction_project) > 0 AND friction_project = trim(friction_project)
        )
    ),
    friction_frequency TEXT CHECK (
        friction_frequency IS NULL OR friction_frequency IN (
            'daily', 'weekly', 'monthly', 'occasional', 'unknown'
        )
    ),
    friction_impact TEXT CHECK (
        friction_impact IS NULL OR friction_impact IN ('low', 'medium', 'high', 'unknown')
    ),
    friction_category TEXT CHECK (
        friction_category IS NULL OR friction_category IN (
            'information-finding', 'repeated-action', 'context-switching',
            'remembering', 'verification', 'waiting', 'other'
        )
    ),
    friction_current_workaround TEXT CHECK (
        friction_current_workaround IS NULL OR (
            length(friction_current_workaround) > 0
            AND friction_current_workaround = trim(friction_current_workaround)
        )
    ),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (
        (capture_type = 'friction'
            AND friction_frequency IS NOT NULL
            AND friction_impact IS NOT NULL
            AND friction_category IS NOT NULL)
        OR
        (capture_type IN ('action', 'follow-up', 'decision')
            AND friction_project IS NULL
            AND friction_frequency IS NULL
            AND friction_impact IS NULL
            AND friction_category IS NULL
            AND friction_current_workaround IS NULL)
    )
);

CREATE INDEX idx_records_v2_created
    ON records_v2(created_at DESC, id DESC);
CREATE INDEX idx_records_v2_type_created
    ON records_v2(capture_type, created_at DESC, id DESC);
CREATE INDEX idx_records_v2_friction_project_created
    ON records_v2(friction_project, created_at DESC, id DESC);
```

The checked-in migration SQL becomes the executable source of truth. It may use a
temporary table and final names appropriate to SQLite rebuilding; this contract
must remain synchronized with it.

No universal status column or lifecycle table is added. Future review contracts
will define type-specific state and migrations. Empty details for action,
follow-up, and decision require no placeholder columns.

## Migration 002

Migration 002 must run in one transaction and:

1. verify migration 001 is the expected Go-owned schema;
2. verify there are no legacy `type = 'capture'` rows or `record_tags` rows;
3. create the target table and indexes;
4. copy every legacy `type = 'friction'` row as `capture_type = 'friction'`;
5. map project, classification, workaround, ID, description, and timestamps
   without loss;
6. remove superseded tables, triggers, and indexes;
7. install final target names; and
8. record migration 002 only after every step succeeds.

The current known development database contains no records, but implementation and
tests must not depend on that fact. Existing friction records are preserved. If an
unexpected legacy capture or tag exists, migration fails with a concise
compatibility error rather than silently discarding data. A future explicit import
can define how old thought/idea/etc. captures map to the new model.

Migration 001 is never edited. Released migrations are append-only and immutable.
A failed migration rolls back completely. An unknown newer migration is rejected;
Forge never downgrades.

## Repository surface

The reset repository exposes one creation path and shared reads:

```text
CreateCapture(ctx, capture)
FindByID(ctx, id)
List(ctx, filters)
```

Creation inserts common and matching typed values atomically. Reads validate the
complete stored shape; malformed rows fail rather than being partially decoded.

Do not retain separate capture/friction creation paths, universal status updates,
or the old specialized-review query after callers are migrated. Review repository
operations are deferred.

## Connection and transaction rules

- Enable `PRAGMA foreign_keys = ON` on every connection.
- Retain the accepted 250 ms busy timeout.
- Use at most one logical database handle per command.
- Parameterize values and allow-list dynamic SQL.
- Validate and confirm before beginning a write transaction.
- Clock, randomness, constraint, lock, cancellation, and migration failures leave
  no partial record or schema change.
- Do not tune journal or synchronous modes without durability evidence.

## Filesystem security

Default locations remain:

```text
Linux: $XDG_DATA_HOME/forge/forge.db
       falling back to ~/.local/share/forge/forge.db
macOS: ~/Library/Application Support/forge/forge.db
override: $FORGE_DATA_DIR/forge.db
```

Configured base paths must be absolute. Path resolution is pure. Create the data
directory lazily with mode `0700`; require directory and database ownership by the
effective user; reject symlinks; require a regular database file; and create or
correct database mode `0600`. Use no-follow/open-and-inspect facilities where
supported to avoid races.

Read commands open existing storage without creating directories, databases, or
migrations. Missing storage is an empty result for list and not-found for show.
