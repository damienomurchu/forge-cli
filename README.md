# Forge CLI

Forge is a fast, private command-line tool for capturing work that should not be
lost. Captures are stored locally in SQLite and have one of four types:
`friction`, `action`, `follow-up`, or `decision`.

## Commands

```text
forge capture DESCRIPTION
forge capture --quick --type TYPE DESCRIPTION
forge completion SHELL
forge list [--limit N] [--type TYPE] [--project PROJECT] [--json]
forge show [--json] RECORD_ID
```

Interactive capture asks for the type, collects only the fields relevant to that
type, shows a summary, and writes only after confirmation. Quick capture never
prompts and requires an explicit type.

Friction captures can also record a project, frequency, impact, category, and
current workaround. The other capture types deliberately start with no additional
metadata.

Type-aware review workflows are planned but not yet specified or implemented.

Run `forge --help` or `forge <command> --help` for complete command usage.

## Shell completion

Generate a completion script for Bash, Fish, or Zsh:

```sh
forge completion bash
forge completion fish
forge completion zsh
```

Source the output from your shell startup file or save it in that shell's
completion directory. Repository contributors can run `just completions` to write
all three scripts under `completions/`, or `just completion-check` to generate and
syntax-check them using the locally installed shells.

## Development

The project requires the Go version pinned in `mise.toml` and uses
[just](https://github.com/casey/just) for development commands:

```sh
just build            # build bin/forge
just test             # run the test suite
just completion-check # syntax-check generated shell completions
just check            # formatting, modules, vet, tests, race, coverage, vulnerabilities
just ci               # checks plus cross-builds and benchmark harness tests
```

Build and exercise the command with isolated data:

```sh
just build
export FORGE_DATA_DIR="$(mktemp -d)"
./bin/forge capture --quick --type action "Ship the release"
./bin/forge list
```

For repeatable performance measurement, run `just bench` with `hyperfine` 1.18.0
or newer, or `just benchmark /path/to/forge` for an existing executable.

## Design documentation

- [Architecture](docs/architecture.md) describes package boundaries and durable
  engineering constraints.
- [CLI contract](docs/cli-contract.md) defines the command shell, list, and show.
- [Capture contract](docs/capture-contract.md) defines interactive and quick
  capture behavior.
- [Record contract](docs/record-contract.md) defines records and public JSON.
- [Storage contract](docs/storage-contract.md) defines persistence, security, and
  durability invariants.
- [Performance contract](docs/performance-contract.md) defines measurement and
  latency budgets.

Accepted dependency decisions are recorded under `docs/decisions/`.
