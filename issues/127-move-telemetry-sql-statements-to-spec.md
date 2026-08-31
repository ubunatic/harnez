# 127 — Move `internal/telemetry` SQL statements into `spec/`

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Architecture
**Related**: `docs/other/Spec.md` (spec-driven architecture), `internal/telemetry/schema.go`,
`internal/telemetry/insert.go`, `internal/telemetry/query.go`

## Problem

`internal/telemetry`'s SQL currently lives as Go string literals scattered across three files:

- `schema.go` — `schemaDDL`: `CREATE TABLE`/`CREATE INDEX` for `tool_calls`.
- `insert.go` — the single `INSERT INTO tool_calls` statement.
- `query.go` — five statements: the row-fetch `SELECT` in `Query`, the two aggregate
  `SELECT`s in `Aggregate`/`DistillationSavings`, the grouped `SELECT` in
  `aggregateGroupedBy` (shared by `AggregateByTool`/`AggregateByAgent`), and the dynamic
  `WHERE`-clause builder in `Filter.whereClause()`.

Per `docs/other/Spec.md`, `spec/` is meant to be the single source of truth for values that
would otherwise be duplicated or hardcoded in application code, embedded into the binary via
`go:embed`. The telemetry package's DDL is already explicitly treated as the schema's source
of truth in commentary (`insert.go`'s comment: "does not duplicate the schema's constraints
... per docs/other/Spec.md's 'don't shadow the spec' rule"), but the DDL itself was never
actually moved into `spec/` — it's still a Go `const` string. This ticket makes that
alignment real for all of the package's SQL, not just the DDL.

## Scope (decided — see clarifying-question answers below)

- **Statements in scope**: all of it — DDL (`schema.go`), the `INSERT` (`insert.go`), and
  every `SELECT`/aggregate query in `query.go`, including the dynamic filter/`WHERE`-clause
  logic.
- **File format**: YAML-wrapped, per `docs/other/Spec.md`'s standard convention (not raw
  `.sql` files) — e.g. `spec/telemetry.yaml` holding named SQL statements as string values,
  with a companion `spec/schemas/telemetry.schema.json`. Each non-trivial statement gets an
  inline comment (YAML `#` comment, immediately above the entry) explaining how it works and
  what it's used for — mirrors the existing Go doc-comments on `Query`/`Aggregate`/etc. today
  so that context isn't lost in the move.
- **Dynamic `WHERE`-clause filters**: move into the spec too, as a `filters`/`predicates`
  subsection of the YAML tree, structured so query.go can compose them stably by name (e.g.
  each optional filter — `tool_name`, `agent_id`, `ticket_id`, `session_id`, `call_type`,
  `created_at >=`, `created_at <` — becomes a named fragment with its own placeholder/arg
  binding convention) rather than only the static base `SELECT`s living in spec while the
  `WHERE` assembly stays a Go string-builder as originally proposed.

## Design questions to resolve during implementation

These were NOT settled by the scoping conversation and need a decision (either up front, or
as the first step of the implementing ticket):

1. **Composition contract**: what does "stable composition" of named filter fragments look
   like concretely — e.g. do fragments carry their own `{column} = ?` shape with a named
   placeholder key, or a fixed positional `?`, and how does `query.go` map a `Filter` struct
   field to the right fragment name without hardcoding column-name strings back in Go (which
   would reintroduce the shadowing this ticket exists to remove)?
2. **`aggregateGroupedBy`'s `column` parameter**: today `column` (`tool_name` or `agent_id`)
   is spliced directly into the grouped `SELECT`/`ORDER BY`/`GROUP BY`. Decide whether the
   spec encodes two variants of the grouped query (one per column) or one parameterized
   template with an internal-only column-name substitution point (must stay caller-input-safe
   — see the existing comment in `query.go` noting `column` is always an internal literal,
   never external input).
2b. Confirm the JSON Schema's role here: since the "values" are SQL statement bodies (opaque
    strings from the schema's point of view), decide what the schema actually validates
    (presence/required keys, statement-name enum, comment-field shape) versus what it can't
    meaningfully constrain (SQL syntax) — the value of `additionalProperties: false` still
    holds for the wrapper keys even if it can't validate SQL itself.
3. **`go:embed` + parsing**: decide the loader shape — parse the YAML once at package init
   into a struct of named statement strings, versus a lazy per-call lookup. Given
   `internal/telemetry` is a low-traffic local-CLI package, prefer whichever is simpler; do
   not over-engineer caching for a workload this small.
4. **Migration granularity**: land as one commit replacing all of `schema.go`/`insert.go`/
   `query.go`'s literals, or a per-file sequence (DDL first, then INSERT, then the five
   SELECT/aggregate statements) — given this repo's existing tickets tend to close cleanly
   per self-contained unit, prefer splitting only if the reviewer finds the combined diff hard
   to verify against the existing test suite in one pass.

## Acceptance Criteria

- [ ] `spec/telemetry.yaml` (or equivalent name) holds every SQL statement currently in
      `schema.go`/`insert.go`/`query.go`, each with a `# yaml-language-server: $schema=...`
      header and a companion JSON Schema under `spec/schemas/`.
- [ ] Each non-trivial statement has an inline comment explaining its mechanics and purpose,
      preserving the intent of the Go doc-comments it replaces.
- [ ] The dynamic filter/`WHERE` composition is driven by named fragments defined in the spec,
      not a Go string-builder assembling raw column names.
- [ ] `internal/telemetry`'s Go code no longer contains SQL string literals for these
      statements — it loads them from the embedded spec.
- [ ] All existing `internal/telemetry` tests pass unmodified in behavior (schema shape,
      insert behavior, query/aggregate results) — this is a representation change, not a
      behavior change.
- [ ] `docs/other/Spec.md` or its layout section gains an example/reference to this as a
      second real spec-file example (today's doc examples are UI-oriented: actions/layout/
      strings), since a SQL-as-spec case is a genuinely different shape than the doc's
      existing worked examples.

## Notes

Filed after asking the user clarifying questions on scope (all SQL, not just DDL), format
(YAML-wrapped per `Spec.md` convention, not raw `.sql`), and how the dynamic query-filter
logic should interact with the spec (filters move into spec too, composed by name) — see
this ticket's own git history / originating conversation for the raw Q&A. This is a
representation/architecture change with no intended behavior change; treat any test
modification beyond what's needed for the new loading mechanism as a signal something
drifted.
