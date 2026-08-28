# Repository Instructions

## Required context

- Before planning, reviewing, or changing this repository, read `README.md` and
  every Markdown file under `docs/`, including `docs/decisions/`.
- Treat the contracts under `docs/` as the source of truth for externally
  observable behavior. Use the architecture document for package boundaries and
  durable engineering constraints, and the ADRs for accepted dependency choices.
- Re-read the relevant contract whenever a task changes CLI behavior, records,
  storage, installation, architecture, dependencies, development workflow, or
  performance.
- For every implementation change, assess its documentation impact and update all
  affected repository documentation in the same change, including `README.md`,
  contracts, architecture notes, installation guidance, and ADRs as appropriate.
  Documentation must describe the resulting implementation accurately; do not
  leave known discrepancies for a later change.

## Product constraints

- Forge has one creation command, `forge capture`, and four capture types:
  `friction`, `action`, `follow-up`, and `decision`.
- Do not add speculative review workflows, lifecycle fields, a universal status,
  a top-level `friction` command, or a generic `update` command.
- Keep IDs opaque at command boundaries and preserve stable public JSON shapes.
- Preserve validation-before-storage, atomic writes and migrations, read-only
  command behavior, deterministic ordering, and terminal-safe human output.
- Never modify a released migration. Add a new migration for schema changes.
- Keep supported release builds CGO-free unless an explicit architectural
  decision changes that requirement.

## Development workflow

- Use the Go version pinned in `mise.toml` and prefer the recipes in `justfile`.
- Keep changes focused and preserve existing user work in the working tree.
- Add or update tests for behavioral changes and regressions.
- Run the narrowest relevant tests while iterating. Before handing off substantive
  changes, run `just check`; run `just ci` for release-sensitive changes when
  practical.
- If implementation and documentation disagree, do not silently choose one:
  identify the conflict and resolve it in favor of the documented contract unless
  the task explicitly approves a contract change.
