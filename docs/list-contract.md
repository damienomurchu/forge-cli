# List Command Contract

## Synopsis

```text
forge list [--limit N] [--type TYPE] [--project PROJECT] [--json]
```

With no filters, list returns every capture. Filters combine with AND semantics:

- `--type` accepts `friction`, `action`, `follow-up`, or `decision`;
- `--project` matches normalized friction project values and therefore only
  returns friction captures;
- `--limit` is positive and applies after filtering and ordering.

There is no universal `--status` filter. Lifecycle filtering will be designed with
type-specific review contracts.

Unknown flags, positional arguments, and missing flag values are usage errors.
Invalid types, empty project filters, and non-positive limits are validation
failures.

Results use `created_at DESC, id DESC`. Human mode emits one terminal-safe line per
capture with no heading:

```text
<id>  <capture-type>  <description>
```

An empty human result writes nothing. JSON emits a complete record array followed
by a newline, including `[]` when empty. Listing is strictly read-only and never
initializes or migrates missing storage.

Implementation status: the live list command uses `parseUnifiedListRequest` and
`executeUnifiedList`. The parser produces unified
repository filters, recognizes only the four capture types, normalizes project
values, validates positive limits, and rejects the removed universal status
filter. The executor passes those filters to the
unified repository and renders the shared human or JSON result without owning
storage discovery or session setup. The live handler retains that ownership and
keeps missing-storage reads non-creating.
