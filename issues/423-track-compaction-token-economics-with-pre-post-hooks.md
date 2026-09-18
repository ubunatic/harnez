# 423 — Track compaction token economics with pre/post hooks

**Status**: Open
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

### M2 — Session token reconciliation

- Reconcile transcript/provider token snapshots across compaction boundaries,
  retaining per-turn deltas and cumulative totals through `SessionEnd`.
- Attribute cached and uncached input separately for the remaining session.
- Verification: fixture-driven tests cover zero, one, and multiple compactions,
  cumulative-counter resets, missing `PostCompact`, and session exit after a
  compaction.

### M3 — Pricing and compaction economics

- Define configurable model pricing with versioned pricing inputs and a safe
  default for unknown models.
- Calculate estimated compaction cost, post-compaction consumption cost, and
  estimated savings versus the no-compaction baseline where the available data
  supports that comparison.
- Verification: table-driven cost tests cover cached/uncached rates, output and
  reasoning rates, rounding, missing rates, and provider counter resets.

### M4 — Query/reporting and documentation

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

**Status**: Draft

---

Reserved placeholder ticket.
