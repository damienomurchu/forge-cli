CREATE TEMP TABLE migration_002_no_legacy_captures (
    count INTEGER NOT NULL CHECK (count = 0)
);
INSERT INTO migration_002_no_legacy_captures(count)
SELECT COUNT(*) FROM records WHERE type = 'capture';

CREATE TEMP TABLE migration_002_no_legacy_tags (
    count INTEGER NOT NULL CHECK (count = 0)
);
INSERT INTO migration_002_no_legacy_tags(count)
SELECT COUNT(*) FROM record_tags;

CREATE TEMP TABLE migration_002_representable_friction (
    count INTEGER NOT NULL CHECK (count = 0)
);
INSERT INTO migration_002_representable_friction(count)
SELECT COUNT(*) FROM records
WHERE type = 'friction' AND status <> 'captured';

CREATE TABLE records_v2 (
    id TEXT PRIMARY KEY,
    capture_type TEXT NOT NULL CHECK (
        capture_type IN ('friction', 'action', 'follow-up', 'decision')
    ),
    description TEXT NOT NULL CHECK (length(description) > 0),

    friction_project TEXT CHECK (
        friction_project IS NULL OR (
            length(friction_project) > 0
            AND friction_project = trim(friction_project)
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
    ),
    CHECK (
        length(id) = 36
        AND substr(id, 1, 4) IN ('cap_', 'frc_')
        AND substr(id, 5) NOT GLOB '*[^0-9a-f]*'
        AND (substr(id, 1, 4) = 'cap_' OR capture_type = 'friction')
    )
);

INSERT INTO records_v2 (
    id,
    capture_type,
    description,
    friction_project,
    friction_frequency,
    friction_impact,
    friction_category,
    friction_current_workaround,
    created_at,
    updated_at
)
SELECT
    id,
    'friction',
    description,
    project,
    friction_frequency,
    friction_impact,
    friction_category,
    current_workaround,
    created_at,
    updated_at
FROM records
WHERE type = 'friction';

DROP TABLE record_tags;
DROP TABLE records;
ALTER TABLE records_v2 RENAME TO records;

CREATE INDEX idx_records_created
    ON records(created_at DESC, id DESC);
CREATE INDEX idx_records_type_created
    ON records(capture_type, created_at DESC, id DESC);
CREATE INDEX idx_records_project_created
    ON records(friction_project, created_at DESC, id DESC);

DROP TABLE migration_002_no_legacy_captures;
DROP TABLE migration_002_no_legacy_tags;
DROP TABLE migration_002_representable_friction;
