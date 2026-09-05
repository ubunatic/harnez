# 126 — Document the `Closed — resolved in <commit>` self-reference convention

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: `docs/practices/IssueTracking.md` §3 (Standard Ticket Metadata Schema), `docs/studies/2026-08-31-harnez-tool-observability-and-the-real-environment-verification-gap.md`

## Problem

`docs/practices/IssueTracking.md`'s ticket metadata schema specifies
`**Status**: Closed — resolved in <commit>`, but never addresses an
inherent limitation: a commit's hash is content-addressed, so a commit
that edits its own ticket file to say "resolved in `<this commit's own
hash>`" cannot know that hash before it's created. There are only two
ways to close this loop, and the doc currently documents neither:

1. Commit with a best-guess/placeholder hash, then a small follow-up
   commit corrects it to the real one (two commits, ticket references
   the second — or the first if the convention is "reference the code
   commit, not the fixup").
2. Don't try to self-reference at all — reference the *previous* commit,
   or leave the hash for a human/reviewer to fill in after the fact.

During the 2026-08-31 `harnez-tool-observability` sprint (issues
115–124, see the linked case study), option 1 was used by every dev
subagent across all eight tickets, and needed a follow-up "fix ticket
NNN status hash reference" commit on nearly every single one — a small
but entirely predictable, repeated piece of friction. Each subagent
independently treated the mismatch as something to solve rather than an
expected, documented convention, burning a turn per ticket re-deriving
the same non-solution (perfect self-reference is impossible; a follow-up
commit is not an oversight, it's the mechanism).

## Scope

- Add a short note to `docs/practices/IssueTracking.md` §3 (next to the
  `**Status**` schema line) documenting the self-reference limitation
  explicitly: a commit's own hash isn't known until after it's created,
  so `Closed — resolved in <commit>` is expected to need a small,
  separate follow-up commit to correct a best-guess placeholder — this
  is normal, not a mistake to avoid or a sign the first commit did
  something wrong.
- State which convention this repo actually wants (pick one, don't
  present both as equally valid, since that just reintroduces the
  "which do I use" hesitation this ticket exists to remove):
  - Option A: commit with the ticket referencing a placeholder or the
    commit's expected-but-unconfirmed hash, then one small follow-up
    doc-only commit corrects it. (What every ticket in 115–124 actually
    did, works fine, costs one extra tiny commit per ticket.)
  - Option B: don't put a real commit hash in the ticket at all — use a
    stable reference instead, e.g. `Closed — see git log` or a
    close-dated marker, and let `git log --oneline -- issues/NNN-*.md`
    be the actual source of truth for which commit(s) touched it.
- Update the metadata schema block in §3 if the chosen convention
  changes the literal `Status` line format.

## Acceptance Criteria

- [ ] `docs/practices/IssueTracking.md` explicitly states the
      self-reference limitation and the one sanctioned convention for
      handling it — no future ticket closure should need to re-derive
      "how do I reference my own commit."
- [ ] The convention is simple enough that a fresh dev subagent with no
      prior context can follow it correctly the first time from just
      the doc (this is a copyable, bundled practices doc — the audience
      is exactly "a subagent that has never seen this repo before").

## Notes

Low priority, pure documentation, no code changes. Motivated by repeated
low-signal friction, not a functional bug — see the "Post-review
correction" pattern repeated across issues/115/116/117/118/119/120/121/122
for the concrete recurrence.

## Implementation Plan

Documentation-only. One file, one small section, plus a consistency sweep of the
schema block that repeats the `Status` line.

### Step 0 — Pick the convention (recommendation: **Option B**)

Evidence from the tracker itself, which the ticket did not have when filed:

```
$ grep -h '^\*\*Status\*\*: Closed' issues/*.md issues/archive/*.md | sort | uniq -c | sort -rn
  18  **Status**: Closed
  16  **Status**: Closed — resolved
   3  **Status**: Closed — research complete
   ...
   2  **Status**: Closed — resolved in d5d7566
```

Out of ~50 closed tickets, exactly **two** carry a real commit hash. The repo has
already converged on Option B in practice; Option A is a convention almost nobody
followed and that cost a fixup commit every time someone tried. Nothing in the
code depends on the hash either — `internal/issues/issues.go`'s status parser only
canonicalizes the `Closed` prefix (`StatusClosed`), and `internal/index` and
`internal/find` key off that category, never the tail text. So Option B is free to
adopt and needs no tooling change.

Sanctioned form: `Closed — <short resolution>` (e.g. `Closed — resolved`,
`Closed — research complete`, `Closed — invalid`, `Closed — obsolete`), with
`git log --oneline -- issues/NNN-*.md` as the source of truth for which commits
touched it. A hash stays *allowed but not expected* for the rare case where one
specific commit is genuinely worth pinning (e.g. a revert reference) — and then it
is added by a later commit, deliberately, not chased.

### Step 1 — Edit `docs/practices/IssueTracking.md`

Two touch points, both in §3 (Standard Ticket Metadata Schema):

1. The literal schema block (line ~54):
   `**Status**: Open | In Progress | Blocked — <reason> | Closed — <resolution> | Draft`
   — drop `in <commit>` from the template so the doc stops advertising the hash form.
2. The "Allowed Values → Status" list (line ~78): replace the
   `Closed — resolved in 58d1fa3` example with `Closed — resolved` /
   `Closed — invalid`, and append a 3–4 sentence note directly under it:

   > **Why no commit hash**: a commit's hash is content-addressed and cannot be
   > known by the commit that writes it, so a ticket can never self-reference its
   > own closing commit without a follow-up fixup commit. Don't try — record the
   > resolution in words and let `git log --oneline -- issues/NNN-*.md` be the
   > traceability mechanism. A hash in a `Status` line is optional and, if used,
   > is expected to point at some *earlier* commit, never this one.

   Keep it declarative and one-option-only — presenting both options is what this
   ticket exists to stop.

### Step 2 — Sweep for the same claim stated elsewhere

`grep -rn "resolved in" docs/ commands/ config.yaml AGENTS.md` — §5's ticket-hygiene
rules and any `config.yaml` `agents_md` section or skill body that restates the
status schema must not keep advertising the hash form, or the doc and the injected
instruction disagree (exactly the cross-layer duplication [[128]] is auditing).

### Step 3 — Propagate and verify

`harnez apply` (IssueTracking.md is a bundled copyable doc → `~/.claude/docs/`),
then `go test ./...`. No existing test asserts this doc's prose, so a green run is
the expected outcome; a failure means a managed-section header was disturbed.
Do **not** retroactively rewrite the two existing tickets that carry hashes —
they're valid under the new rule as "a deliberate earlier-commit reference."

### Design decisions / tradeoffs

- Choosing B loses a per-ticket direct link to its commit. Accepted: `git log` on
  the ticket path recovers it exactly, and the link was only ever populated
  correctly on 2 of ~50 tickets, so the "loss" is theoretical.
- Not adding tooling (e.g. `harnez index --check` rejecting a hash) — the point is
  to remove hesitation, not to add a gate.

### Risks / open questions

- Copyable doc: this change propagates to every harnez-managed repo on their next
  `init`/`apply`. Wanted here, but worth a line in the commit message.
- Verify the user actually prefers B before writing it — the recommendation is
  derived from the repo's own behaviour, not from an explicit statement.

### Scope: **small** (one doc section, one grep sweep, `apply` + tests).
