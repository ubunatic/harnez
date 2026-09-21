# 127 — Move `internal/telemetry` SQL statements into `spec/`

**Status**: Closed — delivered in c812720 and d5178e1; remaining SQL scope moved to 458
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

## Implementation Plan

### Step 0 — Re-inventory (the ticket's list is stale)

`query.go` has grown since filing: it now holds **8** statements, not 5 —
`Query` (line 64), `aggregateGroupedBy` (154, shared by `AggregateByTool`/
`AggregateByAgent`/`AggregateByProject`), `Aggregate` (232),
`DistillationSavings` (365), `RateCallOverhead`'s `SELECT MAX(created_at)` (422),
`UnratedFailureCount`'s `SELECT COUNT(*)` (449), `HeartbeatStats`' `SELECT
COUNT(*), MAX(created_at)` (471) and its `CallsSince` `SELECT COUNT(*)` (492) —
plus `Filter.whereClause()` (26). With `schemaDDL` and the `INSERT`, that is 10
spec entries. Re-list them in this ticket before starting, and note that
`query.go` and `cmd/harnez/stats.go` currently have **uncommitted changes**
(issue 227 project aggregation) — land those first; do not start this on a dirty
tree.

### Step 1 — Spec files

- `spec/telemetry.yaml`, following `spec/reminders.yaml`'s shape exactly:
  `# yaml-language-server: $schema=schemas/telemetry.schema.json` header, then a
  file-level comment explaining what this spec is the source of truth for (and,
  as `reminders.yaml` does, what it deliberately is **not**).
- Top-level keys: `schema:` (the DDL), `statements:` (named SQL bodies),
  `predicates:` (the `WHERE` fragments). Every non-trivial entry carries a `#`
  comment above it, ported verbatim from the Go doc comment it replaces — the
  comments on `aggregateGroupedBy`'s `FailureCount` CASE (issue 226) and
  `UnratedFailureCount`'s exclusion list are load-bearing and must survive.
- `spec/schemas/telemetry.schema.json`, draft-07, `$id` under
  `https://ubunatic.com/harnez/spec/schemas/`, mirroring
  `reminders.schema.json`. It validates **structure only**: required top-level
  keys, `additionalProperties: false` on the wrappers, an enum of statement
  names (so a typo'd/removed key fails the embedded-spec test rather than at
  runtime), `minLength: 1` on each SQL string. It cannot validate SQL — state
  that in the schema's `description` (answers design question 2b).
- `embed.go` already embeds `spec` wholesale — no change needed.

### Step 2 — Loader

New `internal/telemetry/sqlspec.go`, copying `internal/usage/actionsspec.go`'s
proven pattern: `fs.ReadFile(harnez.DefaultFS, "spec/telemetry.yaml")` →
`parseTelemetrySQLYAML` → `sync.OnceValues` → a `mustTelemetrySQL()` that panics
on a broken embedded spec (a build-time defect, not a user condition). Plus
`sqlspec_test.go` with the `TestEmbeddedTelemetrySpecIsValid` guard every other
spec has. Answers design question 3: parse once at first use, no per-call
lookup, no caching beyond `OnceValues`.

### Step 3 — Design question 1 (filter composition)

Recommended contract, keeping Go free of column-name strings: each `Filter`
field maps to a *fragment name* that equals the field's spec key, and the spec
owns the column:

```yaml
predicates:
  tool_name:   { sql: "tool_name = ?" }     # binds Filter.ToolName
  project:     { sql: "project_name = ?" }  # note: field name != column name
  since:       { sql: "created_at >= ?" }
  until:       { sql: "created_at < ?" }
```

`whereClause()` becomes: iterate an ordered list of `(fragmentName, value)`
pairs built from the `Filter` struct, look each fragment up by name, append its
`sql` and its arg. Go then holds fragment *names* (`"project"`), not column
names (`"project_name"`) — which is the shadowing the ticket wants removed.
Positional `?` only; no named placeholders (SQLite's `database/sql` path here
uses positional args everywhere else, and mixing is a needless second convention).
Order must stay deterministic — it is part of the arg-binding contract and of
existing test expectations.

### Step 4 — Design question 2 (`aggregateGroupedBy`'s `column`)

Use **one parameterized template with a named substitution point** (e.g.
`{{group_column}}`), plus an explicit spec-side allowlist
(`group_columns: [tool_name, agent_id, project_name]`) that the loader validates
the caller's value against, returning an error for anything else. Two duplicated
variants would drift; three (project grouping already exists) makes that worse.
The allowlist keeps the existing "internal literal, never external input"
guarantee mechanically enforced rather than comment-enforced.

### Step 5 — Migrate the call sites, in this order

1. `schema.go` — `schemaDDL` → `spec.Schema`. Run `go test ./internal/telemetry/...`.
2. `insert.go` — the `INSERT`. Note its column list and the Go arg order are
   coupled; a mismatch is silent data corruption, so add a test that inserts one
   fully-populated row and reads every field back (if `telemetry_test.go` does
   not already assert every column, this is the one test addition the ticket's
   "tests unmodified" rule should allow).
3. `query.go` — `whereClause` + the 8 statements.

Answering design question 4: commit per file (three commits), not one — the
`INSERT` step's column/arg coupling deserves its own reviewable diff.

### Step 6 — Docs

`docs/other/Spec.md`: add `spec/telemetry.yaml` as a worked example of a
**non-UI** spec (all existing examples are actions/colors/strings), specifically
noting the "spec owns SQL, JSON Schema validates structure only" split.

### Design decisions / tradeoffs — read before starting

This is the ticket's real risk. Moving SQL out of Go **costs** compile-time
locality: today a wrong column name is one grep from the struct that scans it;
after the move it is a runtime panic/scan error caught only by tests. The
benefits are the ones `Spec.md` claims (single source of truth, no shadowing,
external tooling can read the schema). Mitigations, all cheap, and all required:
the statement-name enum in the JSON Schema, the `TestEmbeddedTelemetrySpecIsValid`
guard, and the full round-trip insert test in step 5.2. If those three are not in
place, the move is a net loss and should be stopped rather than shipped.

Also worth a decision up front: the smaller `COUNT(*)`/`MAX(created_at)`
one-liners (lines 422/449/492) arguably gain nothing from the move. Recommend
moving them anyway for the acceptance criterion "no SQL string literals remain" —
a partial move leaves the reader unsure which file is authoritative, which is
worse than either extreme.

### Risks / open questions

- **Behaviour-change risk is real despite the "representation only" framing** —
  string concatenation order in `aggregateGroupedBy` (the `ExpectedFailureCallType`
  arg is spliced *before* the where args, matching its position in the SQL text).
  Any spec-side reordering of that statement silently misbinds args. Cover it with
  an existing-behaviour assertion before touching it.
- **Concurrent churn**: `query.go` is actively changing (issue 227). Sequence this
  after that work lands, or the rebase is painful.
- Keep `docs/other/Spec.md`'s "don't shadow the spec" claim honest: after this,
  `types.go`'s `ToolCall` struct is still a hand-maintained parallel field list.
  Out of scope, but say so explicitly rather than implying the shadowing is gone.

### Scope: **large** (10 statements, a new loader + schema, three commits,
plus a docs update) — and largest in review cost, not in line count.

## Milestones (lean-sprint, 2026-09-21)

- **M1 delivered**: `spec/telemetry.yaml` + `spec/schemas/telemetry.schema.json`, `sync.OnceValues`
  loader (`internal/telemetry/sqlspec.go`), schema DDL and every `INSERT` in `insert.go` moved;
  AST-based test forbids SQL literals in `schema.go`/`insert.go`; `Spec.md` references it.
- **M2 (next)**: the 8 `query.go` statements plus `Filter.whereClause()` as named predicate
  fragments; grouped query as one template with an internal allowlist for the column
  (`tool_name`, `agent_id`, `project_name`).
- **Remaining scope after M2 (decide, don't assume)**: `telemetry.go` migrations/introspection
  SQL and independent SQL in `classify.go`, `sanitize_cache.go`, `issuesnapshot.go`, `export.go`,
  `economics_query.go`. 457's canonical queries must live in the same spec file.

- **M2 delivered**: all `query.go` SELECTs, named filter predicates, and the grouped-query column
  allowlist are in `spec/telemetry.yaml`; AST tests cover `schema.go`, `insert.go`, `query.go` and
  fail on dead spec entries. Still outside the spec: `telemetry.go` migration/introspection SQL and
  SQL in other files (see above). Ticket stays open until that scope is decided.

## Closed (2026-09-21)

M1 and M2 delivered. Remaining SQL scope moved to [458](458-move-remaining-telemetry-sql-into-spec-127-follow-up.md).
