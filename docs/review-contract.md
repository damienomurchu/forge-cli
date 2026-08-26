# Review Command Design Boundary

Review is intentionally deferred pending updated product requirements.

## Reserved surface

```text
forge review friction
forge review action
forge review follow-up
forge review decision
```

Review must be type-aware and support meaningful next transitions rather than
merely displaying a filtered list. A universal status model is prohibited unless
later requirements demonstrate genuinely shared semantics.

Potential workflow language is context, not a contract:

- action: execute, defer, delegate, complete
- follow-up: wait, chase, escalate, update, close
- decision: clarify, record outcome and rationale, revisit, validate, reverse
- friction: requirements to be supplied later

## Not yet decided

Do not implement, persist, or expose assumptions about:

- lifecycle states or transition graphs;
- which records are actionable;
- review ordering, batching, or filtering;
- interactive versus non-interactive transition syntax;
- timestamps, assignees, deadlines, rationale, or outcome fields;
- JSON schemas for review results;
- whether a transition completes review or schedules another review.

Until these decisions are approved, top-level help may describe review as planned
but must not present it as an available implemented workflow. General read-only
inspection remains available through `forge list` and `forge show`.
