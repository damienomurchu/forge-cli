# Forge CLI

Forge is being rewritten in Go as a drop-in replacement for Forge 2.1.0. The goal
is a fast native executable that preserves the existing command-line behavior,
SQLite data, security properties, output, and release experience.

## Status

The rewrite is currently in the planning stage. Implementation has not started.
The next milestone is Phase 0: capturing compatibility fixtures, baseline behavior,
and performance measurements from Forge 2.1.0.

The complete contract, architecture, phased plan, and acceptance criteria are in
[the rewrite blueprint](docs/blueprint.md).

## Targets

- Preserve Forge 2.1.0 commands, outputs, exit codes, and database compatibility.
- Keep warm `--help` and `--version` invocations at or below 30 ms.
- Keep warm commands against a small local database below 50 ms.
- Produce native Linux AMD64 and macOS ARM64 executables without Python.

## Development

Work proceeds one blueprint phase at a time. Compatibility fixtures and automated
parity checks must be established before the Python implementation is replaced.
Until implementation begins, the blueprint is the authoritative project document.
