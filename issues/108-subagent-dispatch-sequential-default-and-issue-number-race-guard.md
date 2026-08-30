# 108 — Subagent dispatch needs a hard sequential-by-default rule + issue-number allocation race guard

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [docs/practices/AgenticLoop.md](../docs/practices/AgenticLoop.md), [docs/studies/2026-08-29-a-day-of-fresh-sprints.md](../docs/studies/2026-08-29-a-day-of-fresh-sprints.md), [issue 036](036-harnez-status-issues-tracker-linter.md) (tracker linter), issue 106, issue 107, commit `f6d8766`

## Incident (2026-08-30)

During a `/fresh-sprint` session, two subagents were dispatched close together
in time, each with an independent "investigation/ticket-filing only, no code
changes" task. Both subagents independently read `issues/README.md` to
determine "the next free issue number," and both landed on **106**. Each
filed its own ticket file — `issues/106-verify-offline-derivability-of-quota-state.md`
and `issues/106-indicate-data-staleness-via-dimming-marker-in-usage-ui.md` —
added its own row to `issues/README.md`, and committed independently. Result:
two files claiming issue 106, and two duplicate `| 106 | ... |` rows in the
README table.

The host orchestrator caught this after the fact by noticing both completion
reports referenced issue 106, and manually fixed it: renumbered the second
ticket to 107, cross-linked it, de-duplicated the README, and committed the
fix in `f6d8766`.

Notably, both colliding tasks were read-only/doc-only (ticket filing), not
concurrent code edits touching the same files. The earlier lesson in
`docs/studies/2026-08-29-a-day-of-fresh-sprints.md` ("What this says about
the harness itself") was specifically about concurrent *code* edits
converging on the same Go package (`internal/usage`) — "parallel is too
risky anyway" once tickets converge on shared files. This incident shows the
risk is broader than file overlap: any shared, sequentially-allocated
resource — here, "the next free issue number" derived by scanning
`issues/README.md` — can race even when the touched *files* don't literally
overlap, because the race is on a number picked by reading shared state, not
on the files each agent then writes.

This also directly extends the **Parallel Read, Sequential Write** invariant
in `docs/practices/AgenticLoop.md` (item 1): that invariant is framed around
"modify files, write code, or execute build mutations in a shared
workspace" as the write hazard. Allocating an issue number from a scan of
`issues/README.md` is a read followed by a write of a *derived* value (the
number), and the read-then-decide step is exactly where two concurrently
dispatched agents can both observe the same "free" state before either has
committed. The invariant's write-side is respected in isolation (both agents
wrote to *different* files, in the trivial file-path sense) but the shared
count they both derived from was stale by the time either wrote.

## Investigation: does `harnez status`'s tracker linter already catch this?

Checked `internal/issues/issues.go` (`LintFS`, called from `internal/claude/status.go`).

- **It already catches the actual symptom that occurred here.** The linter
  has a `DiagDuplicateNumber` diagnostic kind (line ~53) that counts
  occurrences of each issue number across `issues/README.md`'s table rows
  (`tableNumCount[row.Number]++`, checked at line ~345) and emits a
  diagnostic when `count > 1`. Since both subagents added a `| 106 | ... |`
  row, `harnez status` run after both commits landed *would* have reported
  this as `duplicate issue number 106 appears 2 times in table` — the
  collision was catchable by existing tooling, and was only missed here
  because `harnez status` wasn't run between the two commits.
- **But there is a real, confirmed gap underneath it.** `LintFS` also builds
  `numToFileMap := make(map[string][]IssueFile)` (line 237) while walking
  `issues/*.md`, populating it per file (line 274) — but this map is never
  read again anywhere in the function. It is dead for diagnostic purposes.
  So a duplicate *file-level* issue number (two files both prefixed `108-`,
  say) is only caught today as a side effect of both files' README rows also
  colliding — if only one row had been added (e.g. one agent forgot to
  update the README, or updated it and the other agent's commit raced and
  overwrote just that line), the file-level duplicate would go completely
  undetected by the current linter.

## Proposed fix (two parts)

### a. Behavioral/process rule (already in force)

Subagent dispatch defaults to **strictly sequential** — one subagent in
flight at a time — for **all** task types (code edits, investigation,
ticket filing), not just code edits converging on a shared package. Parallel
dispatch happens only on the user's **explicit** request for a specific
task, and even then the orchestrator must verify concurrency actually
happened and check for exactly this class of shared-derived-state race
afterward (duplicate ticket numbers, duplicate README rows, or any other
"scan shared state, derive next value, write" pattern).

This rule has already been saved to the persistent memory system as
`feedback_no_parallel_agents.md` as of this ticket being filed. This ticket
exists to harden it further (make it discoverable in the tracker, and pair
it with the mechanical guard below) — not to introduce it from scratch.

### b. Mechanical guard (proposed, not yet implemented)

Two independent options worth considering for whoever picks this up:

1. **Close the linter gap.** Extend `LintFS` in `internal/issues/issues.go`
   to actually use `numToFileMap` and emit a new diagnostic (or reuse
   `DiagDuplicateNumber`) when a single issue number is claimed by more than
   one file on disk, independent of what the README table says. This closes
   the confirmed dead-code gap above and makes duplicate detection resilient
   to partial/racing README updates, not just the lucky case where both
   racing agents also both remembered to update the README.
2. **Make allocation atomic by construction.** Consider a small
   `harnez issues new` (or similar) helper that claims the next free number
   transactionally (e.g. lock-file or git-index-based compare-and-swap on
   `issues/README.md`, or a monotonic counter file) rather than relying on
   every dispatch — human or agent — to freshly re-scan `issues/README.md`
   and hope nothing else is mid-flight. This is the stronger fix (removes
   the race instead of just detecting it after the fact) but is a real
   design task of its own — flagging it here as an option, not specifying
   an implementation.

Don't over-design either option in this ticket; pick one or both when
actually picking this up.

## Acceptance Criteria

1. Document (this ticket) whether `harnez status` already catches duplicate
   issue numbers — confirmed above: yes, at the README-table-row level via
   `DiagDuplicateNumber`; no, at the file level, since `numToFileMap` is
   built but unused. If picked up for implementation, add the file-level
   check.
2. The sequential-dispatch behavioral rule stands as the primary mitigation
   (already in force via `feedback_no_parallel_agents.md`), with this ticket
   as its paired process-level documentation and the mechanical-guard
   options as the follow-up engineering work.
