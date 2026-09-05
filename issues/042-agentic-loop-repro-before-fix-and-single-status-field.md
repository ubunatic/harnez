# 042 — `docs/practices/AgenticLoop.md` is missing "repro before fix" and "one Status field per ticket" guidance

**Status**: Open
**Category**: Documentation / Agentic Orchestration
**Related**: [039 — AgenticLoop practices and /sprint command](039-agentic-loop-practices-and-sprint-command.md), [Study: 2026-08-20 Lane discipline, deadlock escape, and spatial bucketing (trafficsim)](../../trafficsim/docs/studies/2026-08-20-lane-discipline-deadlock-escape-and-spatial-bucketing.md)

---

## 1. Problem & Motivation

A `/sprint` + `/evergreen` + `/story` session on the `trafficsim` sibling
project surfaced two concrete gaps in the 5-phase agentic loop
(`docs/practices/AgenticLoop.md`, issues/039) that are general enough to
belong in the shared harness practice doc, not just as a one-off lesson
in that project's own ticket.

### 1.1 "Repro before fix" is only a per-ticket convention, not a loop rule

During the trafficsim sprint, two separate mid-implementation bugs were
caught — and only caught — because a script's exact, numeric output
diverged from what a fix was supposed to produce:

- A deadlock-escape trait's first trigger design looked plausible,
  compiled, and passed every *existing* gate, but barely moved a
  reproduction script's freeze-streak metric (1982 frames → 1976). Only
  because the ticket itself had mandated "construct a reliable
  reproduction before implementation begins" was there a number to
  falsify the first attempt against. Root cause (vehicles stuck
  *approaching* a junction, not inside it — a different shape than the
  code assumed) was found by instrumenting the repro, not by review.
- A spatial-bucketing optimization's first candidate-set design
  silently narrowed what a downstream trait could see. It was caught
  because a regression test's *occurrence counts* (not just its exit
  code) diverged from baseline — an existing test happened to assert on
  numbers precisely enough to catch it.

In both cases, "does it pass?" was insufficient — what caught the bug
was "does a specific number match a specific expectation, established
*before* the fix was written." `docs/practices/AgenticLoop.md`'s Phase 2
("Sequential Development & TDD") currently says to add/update tests
alongside implementation and verify test suites at each milestone — it
does not say anything about needing a standing reproduction with a
concrete baseline number *before* attempting a fix for bug-shaped
(especially timing/deadlock/race-shaped) work, as distinct from
feature-shaped work where TDD-as-written is sufficient.

### 1.2 No guidance against duplicate/conflicting Status fields in one ticket file

The same session hit a smaller but recurring paper-cut: a ticket file
had both a top-of-file `**Status:**` summary line and a separate
bottom-of-file `## Status` section (a pattern this project's own issue
template uses for a short header + a fuller narrative status at the
end). After implementation, the bottom section was updated to
"Implemented" but the top line was left at "Open" — because nothing in
the loop's Phase 5 (tracker sync) step calls out checking for *more
than one* status-bearing location in a single file. This was caught
only incidentally during an `/evergreen` pass grepping for `Status` in
that file, not by any systematic check.

`harnez`'s own issue files (see this repo's `issues/*.md`) use a single
`**Status**:` header line with no separate bottom section, so this
project doesn't currently have the failure mode itself — but any
project bundling `docs/practices/AgenticLoop.md` and its own local
ticket template is free to invent a two-field pattern like the one that
bit trafficsim, and the practice doc's Phase 5 step should say
explicitly not to, or to check for it.

## 2. Detailed Technical Specification

### 2.1 `docs/practices/AgenticLoop.md` — Phase 2 addition

Add an explicit sub-rule under "Sequential Development & TDD", something
like:

> **Repro-before-fix for bug/timing/deadlock-shaped tickets**: when the
> ticket is a *defect* (as opposed to a net-new feature), and especially
> when it involves timing, ordering, deadlock, or race conditions,
> construct (or reuse) a reproduction that asserts a concrete numeric
> baseline *before* writing the fix — not just "it should now pass," but
> "this specific measurable quantity should now cross this specific
> threshold." Verify the first implementation attempt against that
> number, not just against `make check`/`go test` exiting 0 — a fix can
> compile, pass every pre-existing gate, and still not address the
> actual defect if the existing gates weren't built to catch it (that's
> presumably part of why the ticket exists). Do not report the ticket
> done until the reproduction's own number moves as expected.

### 2.2 `docs/practices/AgenticLoop.md` — Phase 5 addition

Add a line under "Agentic Flow Quality Retrospective" / tracker sync:

> When updating a ticket's status, check the *entire* file for more than
> one status-bearing field (a top-of-file summary line and a separate
> bottom "## Status" narrative section are both common local
> conventions) — update all of them together, or better, standardize on
> exactly one per file in this project's own ticket template so this
> class of drift is structurally impossible.

### 2.3 Optional: `harnez status` linter check (stretch, not required for this ticket)

`harnez status` (issues/036) already reconciles issue tracker status
against files. Consider (separate, not scoped into this ticket) whether
it could grep for multiple `Status` occurrences in one ticket file and
warn if their values disagree — flagged here for awareness, not
committing to implement it as part of 042.

## 3. Implementation & Verification Plan

1. Add § 2.1 and § 2.2 as new sub-bullets to `docs/practices/AgenticLoop.md`'s
   existing Phase 2 and Phase 5 sections (do not restructure the doc,
   just extend the existing bullets).
2. Update `commands/sprint.md` if it duplicates Phase 2/5 guidance
   inline (check whether it summarizes the practice doc or just
   references it) so the two don't drift.
3. No code changes; this is a docs-only ticket. Verification is
   `harnez diff`/`harnez status` (if applicable) confirming the practice
   doc still copies/syncs cleanly to dependent projects' bundled docs.

## Out of Scope

- Implementing the § 2.3 `harnez status` linter check itself — noted for
  a possible future ticket, not this one.
- Any change to the trafficsim project itself — that project's own
  fixes (the deadlock trigger, the bucket radius, the stale Status line)
  are already resolved there; this ticket is purely about promoting the
  general lesson into the shared harness practice doc.

---

## Implementation Plan

Verified still unaddressed: `docs/practices/AgenticLoop.md` (258 lines) has no
occurrence of "repro", and its only root-cause line is §4 item 3 (friction
reporting), which is a different concern. `commands/sprint.md` (62 lines)
summarizes the phases inline rather than only linking them, so both files need
the edit or they will drift.

### Steps

1. `docs/practices/AgenticLoop.md`, **Phase 2** (§2, starts line ~100): add one
   new bullet at the end of the Mechanics list — the §2.1 text of this ticket,
   trimmed to 3–4 lines. Keep the wording "a fix can pass every pre-existing
   gate and still not address the defect" — that is the load-bearing part.
2. `docs/practices/AgenticLoop.md`, **Phase 5** (§2, starts line ~138): add one
   bullet under the tracker-sync mechanics with §2.2's text, trimmed to 2 lines
   ("check the whole file for more than one status-bearing field; prefer exactly
   one per file in the local template").
3. `commands/sprint.md`: add a one-line echo of each to its Phase 2 (line ~32)
   and Phase 5 (line ~55) blocks — a pointer, not a restatement, so the practice
   doc stays canonical.
4. Verify with `harnez diff` (and `harnez apply` on a scratch HOME, or
   `scripts/smoke-test.sh`) that the bundled doc copies cleanly; no Go changes,
   so `go test ./...` is a no-op gate but should still be run.
5. Do not restructure the doc, do not add a new top-level section, and do not
   implement §2.3's `harnez status` linter — explicitly out of scope here; if it
   is wanted, file it separately after this lands.

### Design decisions

- Both additions go in as bullets inside existing Mechanics lists rather than as
  new sub-headings: the doc is already 6 sections deep and adding headings for
  two rules inflates the very instruction stack 128 is auditing.
- The Phase 2 rule is scoped to *defect-shaped* tickets explicitly, so
  feature work is not burdened with a "produce a baseline number first" step it
  does not need.

### Risks / open questions

- Length: this doc is bundled into every managed project's context. Cap the two
  additions at ~6 lines total; if the drafted text runs longer, cut the
  trafficsim narrative and keep only the rule.
- Sibling projects that already copied `AgenticLoop.md` locally will show drift
  until `harnez init`/`/harnez-sync` reconciles them — expected, not a blocker.

### Scope

Small (docs-only, two files, ~6 added lines).
