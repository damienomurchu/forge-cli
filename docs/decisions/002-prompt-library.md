# ADR 002: Interactive Prompt Library

- Status: accepted with adapter constraints
- Date: 2026-08-25
- Library: `github.com/charmbracelet/huh` v1.0.0

## Context

Forge needs select, text, and confirmation prompts with keyboard navigation,
controlled streams, cancellation and EOF classification, and reliable terminal
restoration. Owning raw terminal state, input decoding, rendering, and signal
handling directly would add substantial correctness and usability risk.

## Decision

Use `huh` behind Forge's narrow prompt interface. Construct prompt forms only after
parsing, validation, terminal detection, and confirmation that interaction is
required. Quick paths must not construct prompt machinery.

The adapter must:

- inject input and prompt-output streams;
- keep prompt rendering away from JSON stdout;
- map user aborts to Forge cancellation;
- classify immediate EOF distinctly and prevent hangs;
- validate choices and defaults before starting terminal UI; and
- restore terminal state after cancellation or failure.

## Consequences

The dependency adds binary size, transitive modules, and behavior beyond Forge's
three prompt primitives. Its types must not escape the adapter. Real pseudo-terminal
tests remain necessary because the rendering stack performs terminal capability
queries that byte-buffer tests do not model.

The library's accessible mode does not propagate all EOF and cancellation errors.
Do not enable it unchanged. A future non-terminal or accessibility path must own
explicit EOF and cancellation behavior.

Reconsider this decision if production measurements, upstream maintenance, or
interaction stability become unacceptable. A custom renderer is the fallback, but
would make Forge responsible for raw mode, escape decoding, signals, resizing, and
cleanup.
