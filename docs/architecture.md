# Forge Architecture

## Product boundary

Forge has one creation command, `forge capture`, and four explicit capture types:
`friction`, `action`, `follow-up`, and `decision`. Every persisted record has
common fields and exactly one matching typed-details shape.

There is no separate friction record family, generic status model, or generic
update command. Type-aware review is deliberately deferred until its product
requirements and lifecycle semantics are approved. Do not add speculative review
fields or transitions.

The archived Python implementation and earlier Go command models are historical
context, not behavioral specifications.

## Package boundaries

```text
main -> cli -> domain + prompt + output + repository -> config + storage
```

- CLI parsing produces typed intent without opening storage.
- Domain code owns normalization and validation without SQL or terminal knowledge.
- Prompt code collects input and never persists it.
- Output code owns terminal-safe human output and stable JSON.
- Repository code owns queries and complete record decoding.
- Storage code owns paths, filesystem checks, connections, and migrations.
- `main` owns process exit; lower layers return errors.

Streams, environment, clock, randomness, terminal detection, and data location are
injected through narrow boundaries. Prompt and SQLite dependencies remain hidden
behind their adapters.

## Durable invariants

- Validate all input before inspecting or creating storage.
- Interactive writes occur only after explicit confirmation.
- A command opens at most one logical database handle.
- Writes and migrations are atomic.
- Read-only commands do not create storage or apply migrations.
- Human output visibly escapes terminal and bidirectional controls.
- JSON stdout contains only the documented JSON value.
- Lists use deterministic newest-first ordering.
- IDs are opaque at command boundaries.
- Storage paths, ownership, file types, permissions, and symlink rejection are
  enforced as specified by the storage contract.
- Security, durability, and validation are never weakened for latency.

## Sources of truth

Externally observable behavior belongs in the contracts under `docs/`. Exact SQL,
internal structure, and implementation status belong in code and tests. Accepted
dependency rationale belongs in ADRs. This document records only boundaries and
constraints that should survive ordinary refactoring.

## Outstanding product work

Review remains intentionally undesigned. Before implementation, define separately
for each capture type its lifecycle, valid transitions, interaction, persistence,
and JSON effects. A universal lifecycle should be introduced only if approved
requirements demonstrate genuinely shared semantics.
