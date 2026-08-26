# Performance Measurement Contract

This document defines reproducible performance measurement for Forge. Performance
is evaluated against the Go product's budgets, not against the archived Python
implementation.

## Initial budgets

On the designated release benchmark host, warm wall-clock medians over at least 30
measured invocations must satisfy:

| Command class | Median budget |
|---|---:|
| `forge --help` | 30 ms |
| `forge --version` | 30 ms |
| Small local database command | 50 ms |

Record p90 alongside every median. A passing median does not excuse severe p90
regressions; the first stable release establishes explicit regression ceilings.

## Harness

Run:

```bash
scripts/benchmark.sh /path/to/forge
```

The harness uses `hyperfine` 1.18.0 or newer for high-resolution process timing.
It defaults to five warm-up runs and 30 measured runs. `--runs` and `--warmup` may
increase those counts; fewer than 30 measured runs are rejected.

The report contains, for each command:

- measured invocation count
- median wall-clock latency
- p90 wall-clock latency
- peak RSS where the host `time` implementation exposes it
- executable size

The harness currently measures `--help` and `--version`. It does not yet enforce
the database-command budget. Adding isolated, deterministic database fixtures and
representative capture, list, and show cases remains required before release.

## Isolation

Every run receives a new temporary `FORGE_DATA_DIR` outside the user's real data
location. Help and version fail the benchmark if they create that path. Future
database fixtures also live entirely inside the harness's temporary directory.

Benchmarks must use a release build and an otherwise idle machine. Record the OS,
architecture, Go version, Forge commit and version, build flags, CPU, run counts,
and harness version with published release results. Do not compare results from
different hosts as if they were a controlled regression measurement.

## Interpretation

Measure complete command invocations, including process startup, parsing, output,
and shutdown. Package microbenchmarks may diagnose a result but cannot establish
that a CLI budget passes. Shared CI may use generous regression ceilings; release
acceptance uses the designated stable host.
