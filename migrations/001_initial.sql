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
