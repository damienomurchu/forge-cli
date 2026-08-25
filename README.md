# Forge CLI

Forge is a fast, native Go command-line application for capturing ideas and
friction in day-to-day work. This repository is a new implementation guided by the
CLI's product design, not a clone of the archived Python version.

## Status

The product specification, performance harness, and fast Go CLI shell are
established. Phase 1 risk spikes accepted initial SQLite and interactive prompt
dependencies with measured constraints. Phase 2 introduced the native executable's
help, version, and exit handling without database or prompt initialization. The
next milestone is Phase 3: domain values, validation, and presentation.

The architecture, phased plan, and acceptance criteria are in
[the rewrite blueprint](docs/blueprint.md). Approved behavior is recorded in the
[CLI contract](docs/cli-contract.md), [capture contract](docs/capture-contract.md),
[friction contract](docs/friction-contract.md), [list contract](docs/list-contract.md),
[show contract](docs/show-contract.md), [update contract](docs/update-contract.md), and
[review contract](docs/review-contract.md). The shared domain and JSON schema are in
the [record contract](docs/record-contract.md), and persistence is defined by the
[storage contract](docs/storage-contract.md).

## Targets

- Preserve and refine the useful Forge workflows and domain concepts.
- Keep warm `--help` and `--version` invocations at or below 30 ms.
- Keep warm commands against a small local database below 50 ms.
- Produce native Linux AMD64 and macOS ARM64 executables without Python.
- Define behavior intentionally through the Go specification and tests.

## Development

Work proceeds one blueprint phase at a time. The archived Python implementation may
inform product discussions, but it is not a compatibility oracle, dependency, or
release target. The blueprint is authoritative until the Go implementation and its
tests establish more detailed contracts.

The repository's development commands use the Go version pinned by `mise.toml`.
Run `just` to list them, `just check` for the standard local quality checks, or
`just ci` for the full check set including supported cross-builds and the benchmark
harness self-test.

### Performance harness

With `hyperfine` 1.18.0 or newer installed, benchmark a candidate executable using
isolated temporary data:

```bash
just benchmark /path/to/forge
```

The harness itself can be checked without Forge or hyperfine:

```bash
just benchmark-test
```
