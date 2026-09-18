# 423 — Track compaction token economics with pre/post hooks

**Status**: Closed — M4 correction delivered and verified
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: #420 (Codex analytics hooks), #422 (Codex telemetry cleanup)

---

## 1. Problem & Motivation

Codex exposes `PreCompact` and `PostCompact` hooks, but harnez does not yet
persist compaction events or connect them to the token usage that follows. This
makes it impossible to assess whether a compaction was worthwhile: what token
and estimated cost the compaction consumed, how much context was saved, and how
much cached versus uncached input was used by the remaining session until exit.

The goal is an evidence-based compaction assessment per session, including
estimated cost using configurable provider/model pricing rather than an
unsupported claim of exact billing.

## 2. Technical Specification / Findings

- Install and handle Codex `PreCompact` and `PostCompact` hooks alongside the
  existing lifecycle hooks.
- Persist a compaction event with session ID, timestamps, ordering, trigger or
  reason when supplied, and pre/post token snapshots when available.
- Associate subsequent tool calls and lifecycle usage with the latest
  compaction boundary so the session can report post-compaction consumption
  through `SessionEnd`.
- Track input, cached-input, output, reasoning, and total tokens separately;
  distinguish per-turn deltas from cumulative provider snapshots.
- Report tokens before compaction, after compaction, tokens consumed by the
  compaction if the hook payload exposes them, and tokens consumed after it
  until session exit.
- Add pricing metadata and cost math for cached input, uncached input, output,
  and reasoning tokens. Preserve the raw token values and pricing inputs so
  estimates are reproducible when pricing changes.
- Handle missing or partial hook payloads explicitly: retain nullable fields and
  mark estimates as partial instead of fabricating values.
- Expose analytics that compare estimated compaction cost against estimated
  downstream savings, including cached and uncached usage, with a clear
  “insufficient data” state.

## 3. Implementation & Verification Plan

### M1 — Hook installation and event persistence

- Add `PreCompact` and `PostCompact` to Codex hook generation, summary, removal,
  and event parsing.
- Add a migration and data model for compaction events and their session
  boundaries.
- Verification: unit tests prove idempotent installation/removal and persistence
  of complete, partial, and malformed-safe hook payloads.

#### M1 delivery review

- Delivered in `b6a0184`: Codex compaction hooks, parser support, schema v5,
  `compaction_events`, `session_boundaries`, lifecycle boundary persistence, and
  complete/partial/malformed payload tests.
- Verification reported by the developer: `make test-q1` and `make install`
  passed; working tree clean.
- Review result: accepted. M2 must preserve nullable/partial snapshots and
  distinguish compaction boundaries from ordinary lifecycle boundaries.

### M2 — Session token reconciliation

#### Pre-Work / Required Refinements

- Use the M1 compaction rows and boundaries as the source of ordering, rather
  than assuming every compaction has a matching tool call.
- Add explicit tests for both `PreCompact` and `PostCompact` in one session,
  multiple compactions, missing `PostCompact`, and cumulative counter resets.
- Ensure session-exit reconciliation cannot overwrite the boundary event with a
  later tool-call snapshot and that absent provider fields remain NULL.

- Reconcile transcript/provider token snapshots across compaction boundaries,
  retaining per-turn deltas and cumulative totals through `SessionEnd`.
- Attribute cached and uncached input separately for the remaining session.
- Verification: fixture-driven tests cover zero, one, and multiple compactions,
  cumulative-counter resets, missing `PostCompact`, and session exit after a
  compaction.

#### M2 delivery review

- Delivered in `c6585b7`: append-only token snapshots, cumulative and per-turn
  fields, derived uncached input, nullable-field preservation, compaction
  ordering, session-exit snapshots, and reset/missing-hook fixtures.
- Verification reported by the developer: `make test-q1` and `make install`
  passed; working tree clean.
- Review result: accepted. M3 must calculate economics from these immutable
  snapshots and retain the pricing inputs used for every estimate.

### M3 — Pricing and compaction economics

#### Pre-Work / Required Refinements

- Treat `token_snapshots` as append-only evidence; do not mutate prior rows when
  calculating deltas or estimates.
- Define how a missing baseline or provider counter reset affects savings: emit
  a partial/insufficient-data result instead of inferring a negative or zero
  saving.
- Include cached input, uncached input, output, and reasoning components in the
  calculation and preserve the exact pricing revision/rates with the result.

- Define configurable model pricing with versioned pricing inputs and a safe
  default for unknown models.
- Calculate estimated compaction cost, post-compaction consumption cost, and
  estimated savings versus the no-compaction baseline where the available data
  supports that comparison.
- Verification: table-driven cost tests cover cached/uncached rates, output and
  reasoning rates, rounding, missing rates, and provider counter resets.

#### M3 delivery review

- Added schema v7 `compaction_economics` persistence with immutable pricing
  revision/rate inputs and integer micro-USD estimates.
- Added configurable model pricing lookup, cached/uncached input, output, and
  reasoning cost math with deterministic half-up rounding.
- Added explicit complete, partial, and insufficient-data results. Missing
  baselines and provider counter resets withhold savings instead of inferring
  zero or negative savings.
- Verification: the required `make test-q1` run reached the new economics
  tests and exposed stale v6/test-fixture assertions, which were corrected;
  `make install` then passed. The suite was not rerun under the repository's
  single-test quota guardrail, so M4 must make the first post-M3 full-suite
  verification and report its result before closure.
- Review result: accepted with M4 refinements. The economics primitives and
  persistence are sound; M4 must wire an actual pricing source/configuration
  into reporting and ensure unknown models remain explicitly insufficient.

### M4 — Query/reporting and documentation

#### Delivered

- Review of `48019ea` found the report reader and pricing loader, but no
  production path that calculates and inserts `compaction_economics` rows from
  real Codex compaction/session data, and no M4 end-to-end fixture tests.

#### Pre-Work / Required Refinements (correction pass)

- Wire the recorded/configured pricing catalog into the compaction/session
  reconciliation path so real sessions create persisted economics rows with
  model, revision, rates, costs, savings, and complete/partial/insufficient
  status. Do not merely load pricing in `stats`.
- Add end-to-end fixture tests for a session with compaction and a session
  without compaction; assert database economics rows and text/JSON stats.
- Preserve schema migration coverage for the model metadata column and ensure
  the report's session/time filters query valid economics columns.
- Keep the existing estimate-vs-provider-billing documentation and pricing
  revision/rate disclosure.

#### Pre-Work / Required Refinements

- Run `make test-q1` once before declaring M4 complete; if it fails, modify
  source/tests and use the quota rule for any subsequent run.
- Wire the pricing catalog to an explicit configuration or recorded model-price
  source; do not leave `PricingCatalog` as an unconnected in-memory type.
- Persist/report the economics result for real session compaction data and
  expose complete, partial, and insufficient-data states in both text and JSON.
- Document that estimates are model-price estimates, not provider billing, and
  show the pricing revision/rates used.

- Add stats output (text and JSON) that reports compaction count, token deltas,
  cached ratio, estimated costs, estimated savings, and confidence/data
  completeness.
- Document the schema, pricing assumptions, limitations, and example queries.
- Verification: end-to-end fixture test asserts the reported economics for a
  session with compaction and a session without compaction; existing telemetry
  tests remain green.

## 4. Acceptance Criteria

- `harnez apply` installs all supported Codex compaction hooks idempotently.
- A session with compaction has queryable pre/post boundaries and token usage
  through exit, including cached and uncached input where supplied.
- Reports distinguish exact provider-reported values from estimated values and
  identify incomplete telemetry.
- Cost calculations retain their pricing inputs and are reproducible.
- A user can determine whether estimated downstream savings exceeded compaction
  cost without inspecting raw hook logs.
- Existing hook, migration, and telemetry behavior remains backward compatible.

## 5. Open Questions

- Which Codex `PreCompact`/`PostCompact` payload fields and transcript events are
  guaranteed across supported Codex versions?
- Does Codex expose direct compaction token usage, or must it be inferred from
  token snapshots and transcript deltas?
- Should pricing be configured globally, per project, or per recorded model
  snapshot, and which provider price source is authoritative?

**Status**: Closed — M4 correction delivered and verified

### M4 correction delivery

- Wired Codex post-compaction and session-end lifecycle handling to persist
  recorded pricing revision/rates and complete, partial, or insufficient
  economics rows; added with- and without-compaction fixture assertions.
- Preserved schema migration coverage and the existing estimate-vs-provider-
  billing documentation.
- Root causes fixed: recorded snake_case pricing rates now deserialize into
  priced usage; session-end snapshots use the canonical source (with legacy
  source fallback); and final reconciliation refreshes every compaction row
  after the session baseline is available.
- The with-compaction fixture now requires `complete` status and positive
  savings; the without-compaction fixture remains `insufficient_data`.
- `make install` completed. Final `make test-q1` passed (`go vet ./...` and
  `go test ./...`).

---

Reserved placeholder ticket.
