# 298 — Third recurrence of the commit-checkpoint gap: file granularity, git checkout -- destructiveness

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [044](044-git-md-proactive-commit-vs-harness-ask-first.md), [046](046-commit-checkpoint-recurred-after-044-filed.md)

---

## 1. Problem & Motivation

[044](044-git-md-proactive-commit-vs-harness-ask-first.md) and
[046](046-commit-checkpoint-recurred-after-044-filed.md) already diagnosed and
(per 046's own Implementation Plan) landed a three-placement doc fix in
`docs/practices/AgenticLoop.md`: commit authority asked at kickoff, a Phase 3
exit condition ("commit or ask, don't carry a reviewed diff into Phase 4"),
and a Phase 5 `git status` check before writing a retro. 046 explicitly
flagged its own escalation path if the doc-only fix didn't hold: *"If
046-style recurrence shows up a third time, file a follow-up for the hook
rather than pre-building it."* This is that third occurrence, in a
different sibling project (`dai`), with a sharper, more concrete failure
mode than either prior ticket described.

**What happened**: mid-session in `dai`, working through a batch of small,
independent TUI fixes (issue 010), I ran `git stash` to temporarily test
pre-fix behavior for one specific bug (proving a new regression test
actually caught it, not just passed vacuously). That step was fine. Earlier
in the *same* debugging arc, I had used `git checkout -- internal/tui/render.go`
for the same kind of "temporarily see the old behavior" goal — but
`checkout --` discards *every* uncommitted change to that file, not just
the one thing being probed. It silently destroyed an unrelated,
not-yet-committed fix (a header-spacer change) that happened to live in
the same file, requiring it to be redone from scratch. The user's own
correction was explicit: *"always commit intermediate results when the
build is clean."*

This differs from 044/046's framing in two ways worth naming separately,
since they suggest a different (or additional) fix rather than "try the
same doc note harder":

1. **044/046 were about *finished, reviewed* work sitting uncommitted for
   a whole session.** This occurrence is about *file-level granularity*
   during active, uncommitted-by-design iteration — several small fixes
   accumulating in the same file before any of them individually felt
   "done enough" to commit. Phase 3's exit condition (commit before
   leaving the review gate) doesn't obviously fire mid-iteration, before a
   review gate is even reached.
2. **The actual damage mechanism was a specific destructive git command
   (`checkout --`), not merely idle time.** 044/046's fix (ask/commit at
   defined checkpoints) reduces *how long* work sits exposed, but doesn't
   address the sharper risk that a *routine-looking, non-obviously-destructive*
   command (temporarily reverting a file to test something) can discard
   uncommitted sibling changes in the same file with no warning. This is a
   near-miss class distinct from "forgot to commit" — it's "committing
   would have made this specific command safe, but nothing flagged that
   the command was risky in the first place."

## 2. Detailed Technical Specification

Two independent angles, either or both worth pursuing:

- **Sharpen the guidance, again, but at a different trigger point.**
  Neither 044's kickoff note nor 046's Phase 3/5 checkpoints cover
  *mid-iteration* commits — "commit once the build is clean" as a standing
  habit *within* a task, not just at its boundaries. Consider whether
  `docs/practices/AgenticLoop.md`'s Phase 2 (Sequential Development) needs
  its own explicit line, e.g.: "commit after each milestone once
  `go build`/`go vet`/tests are clean, not only once the whole ticket is
  reviewed — this bounds the blast radius of any later destructive command
  to 'since the last clean build,' not 'since the review gate.'"
- **Consider the rejected-for-now enforcement mechanism now that doc-only
  has failed to hold three times.** 046 explicitly considered and rejected
  a `Stop`-hook grepping `git status` for uncommitted tracked changes,
  reasoning it would fire on deliberate WIP too. A narrower mechanism might
  avoid that false-positive concern: a `PreToolUse` hook (or harness-level
  guard) specifically on destructive git subcommands against paths with
  uncommitted changes (`checkout --`, `restore`, `reset --hard`, `clean
  -f`) that warns (not blocks) when the target has *other* uncommitted
  hunks beyond what's presumably being reverted — i.e. gate the dangerous
  command itself, not general idleness. This is a different, more targeted
  proposal than the one 046 rejected, worth evaluating on its own rather
  than treating 046's rejection as covering it.

## 3. Implementation & Verification Plan

1. Decide whether to pursue the doc-only angle (Phase 2 addition), the
   mechanism angle (targeted `PreToolUse` guard), both, or neither for now
   pending further evidence.
2. If doc-only: same three-file-location precedent as 046 — edit
   `docs/practices/AgenticLoop.md`, resync via `harnez apply`, verify by
   re-reading for a concrete, non-optional trigger rather than general
   advice (046's own stated bar for what counts as landed).
3. If mechanism: scope as its own ticket rather than folding into this
   one's doc edit, since it's a genuinely different kind of change (hook
   code, not a doc line) with its own design/testing needs.
4. Whichever is chosen, close this ticket referencing what actually landed
   — do not close on "acknowledged" alone, matching 044/046's own history
   of a diagnosis-only close not holding.
