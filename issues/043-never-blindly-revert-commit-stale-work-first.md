# 043 — Never blindly `git revert`/`checkout --`/`stash drop` unsuccessful work; commit stale/failed code first

**Status**: Closed
**Category**: Documentation / Agentic Orchestration — `docs/practices/AgenticLoop.md`
**Related**: [039 — AgenticLoop practices and /sprint command](039-agentic-loop-practices-and-sprint-command.md), [042 — repro before fix and single-status field](042-agentic-loop-repro-before-fix-and-single-status-field.md)

---

## 1. Problem & Motivation

A `/sprint 019` cycle on the `trafficsim` sibling project attempted a fix
for a hard, high-risk defect (`traits/turns.js`'s turn-obstacle scan).
Three fix strategies were tried in sequence, each verified against the
project's own gate (`make test-turn-completion`) before moving to the
next. All three regressed the gate relative to the pre-sprint baseline,
so each was discarded:

- Attempt 1 (a weak second obstacle-detection cone) was abandoned by
  directly overwriting it with attempt 2's edits — no git operation at
  all, just further `Edit` calls layered on top. Its exact diff is gone.
- Attempt 2/3's intermediate states were partially preserved via one
  `git stash` (to free the working tree for a from-scratch rewrite), but
  the stash was later `git stash drop`ped once attempt 3 also failed —
  discarding it deliberately, on the reasoning that the finding (a new
  "mutual-stall" failure mode between two same-lane turning vehicles)
  had already been captured in prose, so the diff itself felt redundant.
- The final revert (`git checkout -- website/traits/turns.js`) returned
  the file to its last-committed state, with only a narrative writeup of
  the three attempts surviving in the ticket file.

This "worked at the time" — the narrative note is genuinely useful and
was written carefully — but it is strictly worse than it needed to be.
None of the three attempts' actual diffs are recoverable now. If a
future session wants to resume from attempt 3 (the closest one, and the
one whose failure mode is the most instructive), it has to reconstruct
the code from prose instead of `git diff`ing an actual commit. A prose
description of a diff is not the diff — it drops exact line numbers,
exact threshold constants, and the ability to `git apply` or `git
cherry-pick` the attempt as a starting point.

This is a **general agentic-loop gap**, not a trafficsim-specific one:
nothing in `docs/practices/AgenticLoop.md`'s Phase 2 (sequential dev/TDD)
or its anti-patterns list tells an agent what to do with code it's about
to discard after a failed attempt. The natural, fast move under time
pressure is exactly what happened here — `git checkout --`/`stash drop`
the failed state and move on, because the code "doesn't work" and
therefore feels disposable. But "doesn't pass the gate we wanted" and
"has no future value" are different claims, and the loop currently
conflates them by giving no explicit guidance either way.

## 2. Why this matters beyond "nice to have"

- **Session continuity**: agentic sessions get compacted, interrupted, or
  handed off. A committed (even if reverted-on-main) attempt survives
  compaction as git history; a narrative note survives only as long as
  the ticket file does, and only as well as the prose was written.
- **Re-attempt cost**: the next person (human or agent) picking up a
  P0/P1 ticket with a documented failed attempt should be able to `git
  diff <stale-commit> <another-stale-commit>` to compare strategies, not
  re-derive each one from a paragraph.
- **Falsifiability**: a stale commit is a testable artifact — someone
  can check it out, re-run the gate, and confirm the regression
  described in prose actually reproduces. A prose-only record cannot be
  independently re-verified without redoing the implementation work
  first.
- **This is the mirror image of 042's "repro before fix" gap**: 042
  established that a *fix* needs a concrete falsifiable baseline before
  being trusted as done. This ticket is the same principle applied to a
  *discarded* fix attempt — a claim ("this regressed the gate") should
  also be backed by a concrete, inspectable artifact, not just prose,
  even when the artifact describes a failure rather than a success.

## 3. Detailed Technical Specification

### 3.1 `docs/practices/AgenticLoop.md` — Phase 2 addition

Add a sub-rule alongside 042's "repro before fix" addition:

> **Commit stale/failed work before discarding it.** When an
> implementation attempt is abandoned — because it regressed a gate,
> because a cleaner strategy was found, or because it was simply wrong —
> do not `git checkout --`/`git reset --hard`/`git stash drop` it away
> as the first move. Commit it first, on the current branch or a
> throwaway one (e.g. `git commit -m "wip: attempt N, reverted — see
> issue NNN" --no-verify` only if hooks block a WIP commit, otherwise a
> normal commit), *then* revert the working tree with `git revert` or by
> checking out the prior commit. This keeps the failed attempt in `git
> log`/`git reflog` as a real, diffable artifact instead of only as
> prose in a ticket. A short-lived local branch (`git branch
> attempt-2-endpoint-cone`) pointing at the WIP commit is even better
> when more than one attempt is worth preserving side-by-side. Only skip
> this for genuinely trivial, single-line experiments where the
> narrative description *is* the diff (e.g. "tried threshold=50, tried
> threshold=25, both failed" needs no commit) — the bar is "would a
> future reader want to `git diff` this," not "is this attempt tidy."

### 3.2 `docs/practices/AgenticLoop.md` — Anti-Patterns list addition

Add to § 4 "Anti-Patterns to Avoid":

> - ❌ **Blind Revert of Failed Work**: Running `git checkout --`, `git
>   reset --hard`, or `git stash drop` on a failed implementation attempt
>   without first committing it somewhere recoverable. A prose summary of
>   what was tried is not a substitute for the actual diff — it cannot be
>   `git diff`ed, re-applied, or independently re-verified against the
>   gate it was tested against.

### 3.3 Optional: `harnez status`/`harnez diff` awareness (stretch, not required for this ticket)

Not scoped here, but worth a future ticket: a lightweight nudge (not a
hard block) when `harnez` detects an agent about to run a destructive git
command against a file with uncommitted changes larger than a few lines
— asking "commit this first?" rather than assuming discard is intended.
This is distinct from the existing "before a destructive git command, run
`git status` and stash/commit" guidance already present in some project
`CLAUDE.md`/`AGENTS.md` files (which covers *accidental* loss of
in-progress work) — this ticket is about *deliberate* discarding of a
completed-but-failed attempt, which the existing guidance doesn't cover
because the agent isn't worried about losing anything at the moment it
reverts.

## 4. Implementation & Verification Plan

1. Add § 3.1 and § 3.2 as new sub-bullets to `docs/practices/AgenticLoop.md`'s
   existing Phase 2 section and § 4 Anti-Patterns list — do not
   restructure the doc, just extend the existing bullets (same approach
   as issue 042).
2. Check `commands/sprint.md` for any inline duplication of Phase 2/
   anti-patterns guidance that would also need the same addition so the
   two don't drift (042 flagged the same risk; if 042's pass already
   handled this generally, just confirm this addition is captured too).
3. No code changes; this is a docs-only ticket. Verification is `harnez
   diff`/`harnez status` confirming the practice doc still copies/syncs
   cleanly to dependent projects' bundled docs.

## Out of Scope

- Implementing the § 3.3 `harnez` git-command nudge itself — noted for a
  possible future ticket, not this one.
- Any change to the trafficsim project itself, or attempting to
  reconstruct the three lost `turns.js` attempts from this session's
  transcript — the prose record in trafficsim's issues/019 sprint note
  is what exists and is being treated as sufficient after the fact; this
  ticket is about preventing the same loss going forward, not repairing
  this specific instance of it.
