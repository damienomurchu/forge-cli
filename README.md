# Forge CLI

Forge is a fast, native Go command-line application for capturing ideas and
friction in day-to-day work. This repository is a new implementation guided by the
CLI's product design, not a clone of the archived Python version.

## Status

Phases 0 through 5 are complete: Forge has its product contracts, performance
harness, native Go CLI shell, validated domain and presentation layers, secure
SQLite storage, and the complete non-interactive command surface. Phase 6 is in
progress. Interactive capture is complete, and friction records with explicit
frequency, impact, and category values can be confirmed interactively. Prompts for
omitted friction classifications remain to be implemented.

### Command availability

The current executable implements the fast, data-free CLI shell and the following
data-backed command slices:

| Invocation | Status |
|---|---|
| `forge`, `forge -h`, `forge --help` | Implemented |
| `forge --version` | Implemented |
| `forge capture -h`, `forge capture --help` | Implemented |
| `forge capture [--quick] [--project PROJECT] [--kind KIND] [--tags TAGS] [--json] DESCRIPTION` | Implemented |
| `forge friction -h`, `forge friction --help` | Implemented |
| `forge friction --quick [--project PROJECT] [--frequency FREQUENCY] [--impact IMPACT] [--category CATEGORY] [--current-workaround TEXT] [--json] DESCRIPTION` | Implemented |
| `forge friction` with explicit frequency, impact, and category | Interactive confirmation implemented |
| `forge friction` with omitted classifications | Interactive prompts planned |
| `forge list -h`, `forge list --help` | Implemented |
| `forge list [--limit N] [--type TYPE] [--project PROJECT] [--status STATUS] [--json]` | Implemented |
| `forge show [--json] RECORD_ID` | Implemented |
| `forge update [--json] RECORD_ID --status STATUS` | Implemented |
| `forge review [--json]` | Implemented |

The remaining interactive friction prompts, release packaging, and product
hardening remain to be implemented in Phases 6 through 8.

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

### Build and try the current CLI

From the repository root, build a local executable using the pinned Go toolchain:

```fish
just build
```

The `bin/` directory and a root-level `forge` binary are ignored by Git. Exercise
the currently implemented behavior with:

```fish
./bin/forge
./bin/forge --help
./bin/forge --version
./bin/forge unknown
echo $status
```

For a quick smoke test that builds Forge and runs its help and version paths:

```fish
just try
```

The final command prints an error to stderr and returns status `2`. To test version
injection in a release-style build:

```fish
mise exec -- go build \
    -trimpath \
    -ldflags="-s -w -X main.version=0.1.0-dev" \
    -o bin/forge \
    ./cmd/forge
```

### Performance harness

With `hyperfine` 1.18.0 or newer installed, benchmark a candidate executable using
isolated temporary data:

```fish
just bench
```

This builds a stripped, release-style `bin/forge` first. To benchmark an existing
candidate without rebuilding it, run `just benchmark /path/to/forge`.

The harness itself can be checked without Forge or hyperfine:

```fish
just benchmark-test
```
