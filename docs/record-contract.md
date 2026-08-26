# Record and JSON Contract

Every persisted record is a capture. `capture_type` determines its validation,
typed details, future lifecycle, and review behavior.

## Common fields

| Field | JSON type | Meaning |
|---|---|---|
| `id` | string | Opaque Forge-generated capture identifier |
| `capture_type` | string | `friction`, `action`, `follow-up`, or `decision` |
| `description` | string | Normalized description |
| `details` | object | Type-specific details |
| `created_at` | string | UTC creation timestamp |
| `updated_at` | string | UTC timestamp of the most recent change |

All fields are present. Timestamps use fixed-width UTC RFC 3339 with six fractional
digits, for example `2026-08-25T12:34:56.123456Z`.

There is no universal public `status` field in the current model. Lifecycle fields
will be added per capture type only after its review requirements are approved.

## Type-specific details

Friction details:

| Field | JSON type |
|---|---|
| `project` | string or null |
| `frequency` | string |
| `impact` | string |
| `category` | string |
| `current_workaround` | string or null |

Action, follow-up, and decision currently emit empty details objects:

```json
"details": {}
```

Details are discriminated by `capture_type`; a record cannot contain fields from a
different type. Optional friction text is `null`, never omitted or encoded as an
empty string.

## IDs

IDs are opaque outside the domain generator. New captures use a stable identifier
form chosen by the implementation and protected by domain tests. Command parsing
must not infer capture type, timestamp, or validity from an ID prefix.

Migration 002 preserves IDs of migration-001 rows. JSON consumers must therefore
treat IDs as opaque and must not depend on one prefix or length.

## JSON stability

Field names and value types become public API when released. Type-specific fields
are added only with an approved workflow. Invalid records are rejected before
rendering; output never silently drops mismatched details.
