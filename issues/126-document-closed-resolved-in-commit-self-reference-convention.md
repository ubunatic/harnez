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
