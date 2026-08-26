# ADR 002: Interactive Prompt Library

- Status: accepted with adapter constraints
- Date: 2026-08-25
- Library: `github.com/charmbracelet/huh` v1.0.0
- Go: 1.27.0

## Context

Forge needs small select, text, and confirmation prompts with arrow and `j`/`k`
navigation, Ctrl-C classification, terminal restoration, controlled streams, and no
initialization on quick or otherwise non-interactive paths.

Implementing raw terminal state, input decoding, signal handling, rendering, and
restoration directly is security- and usability-sensitive. A dependency is
acceptable only if its runtime and maintenance costs fit Forge's fast, small CLI.

## Decision

Use `github.com/charmbracelet/huh` v1.0.0 behind Forge's narrow `Prompt` interface
for full interactive terminal prompts. Construct forms only after command parsing,
validation, terminal detection, and confirmation that prompting is required.

The adapter must:

- inject input and prompt-output streams
- route prompt rendering away from JSON stdout
- map `huh.ErrUserAborted` to Forge cancellation
- observe immediate input EOF and cancel the form with a distinct EOF cause
- validate choices and defaults before starting terminal UI
- avoid accessible mode's error-swallowing input loop

The library remains replaceable behind the interface. Quick paths do not construct
a form. In the unified-capture design, interactive capture always uses the adapter
for type selection and confirmation, plus friction-specific fields when friction
is selected.

## Evidence

The disposable spike in `spikes/prompt` verifies:

- selection with `j`, `k`, Up, and Down
- text entry and confirmation
- explicit cancellation and EOF classification
- prompt-output isolation from reserved JSON stdout
- boundary validation before terminal startup
- Ctrl-C exit behavior through a real pseudo-terminal
- restoration of terminal state after Ctrl-C
- CGO-free Linux AMD64 and macOS ARM64 builds
- Linux and macOS cross-compilation of the PTY test binary

The PTY test answers the standard background-color and cursor-position queries sent
by the styling stack before rendering. This exposed an important integration detail
that simple byte-buffer tests would miss.

On the Linux AMD64 development host, using stripped Go 1.27.0 binaries:

| Measurement | Baseline | With `huh` | Difference |
|---|---:|---:|---:|
| Binary size | 1,216,672 bytes | 4,341,920 bytes | +3,125,248 bytes |
| No-op median, 30 warm runs | 2.346 ms | 3.725 ms | +1.379 ms |
| No-op p90, 30 warm runs | 2.586 ms | 4.146 ms | +1.560 ms |

Additional candidate measurements:

| Measurement | Result |
|---|---:|
| macOS ARM64 stripped binary | 4,145,234 bytes |
| Linux no-op peak RSS | 4,324 KiB |
| Clean Linux build time | 17.22 s |
| Clean Linux build peak RSS | 267,952 KiB |
| Prompt adapter statement coverage | 90.0% |

The startup samples use Bash's microsecond clock because `hyperfine` was unavailable
on the host. They are diagnostic comparisons, not release benchmarks.

`govulncheck` v1.7.0 reported no known vulnerabilities in the repository after the
candidate and its transitive modules were added.

Running the candidate's `noop` mode with closed stdin produces no stdout or stderr
and exits successfully, showing that importing the dependency does not itself open
or render terminal UI. The application architecture must still keep form
construction out of non-interactive branches.

## Discovered limitations

`huh` does not terminate a normal form on immediate EOF by itself. The adapter uses
an observing reader and context cancellation to return a distinct EOF error without
hanging.

The library's accessible mode provides useful numbered and line-oriented prompts,
but its input helpers do not return scanner EOF or cancellation errors to the form.
Forge will not enable that mode unchanged. A later accessibility slice must provide
a Forge-owned plain line fallback with explicit EOF/cancellation handling or
document a clear terminal capability error; accessible mode is not silently
assumed to be safe.

The styling stack issues terminal capability queries before rendering. Real
terminals answer them, and the PTY suite must emulate them to avoid false hangs.

## Costs and risks

- The library adds about 3.1 MB to a minimal stripped Linux binary.
- It adds 23 indirect modules to the module graph beyond the SQLite-only module.
- Full-screen behavior is more complex than Forge's three prompt primitives.
- Upstream rendering or key-map changes could alter interaction behavior.
- Native macOS runtime interaction still requires CI or release-host smoke tests.
- Race testing passes with GCC 16.2.1 and remains a CI requirement.

These costs are accepted because measured startup remains far below budget and the
library owns difficult terminal cleanup and input-decoding behavior. Keep the
adapter small and do not expose library types outside the prompt package.

## Alternatives

A custom terminal renderer would reduce dependencies and binary size but would make
Forge responsible for raw mode, escape-sequence decoding, signals, resizing, and
cleanup. It remains the fallback if production measurements or upstream maintenance
make `huh` unsuitable.

Simple line prompts are appropriate as an accessibility/fallback path, not as a
replacement for the approved arrow and `j`/`k` interaction unless the product
contract changes.

## Reproduction

```bash
go test ./spikes/prompt/...
go vet ./...
go test -cover ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./spikes/prompt/cmd/prompt-spike
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./spikes/prompt/cmd/prompt-spike
```

Remove the disposable spike when its adapter and PTY coverage have moved to the
production prompt package.
