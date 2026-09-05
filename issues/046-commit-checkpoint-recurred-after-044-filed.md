# 046 — The exact gap from 044 recurred in the same session that filed it

**Status**: Open
**Category**: Documentation / Agentic Ergonomics
**Related**: [044](044-git-md-proactive-commit-vs-harness-ask-first.md), [docs/Git.md](../docs/Git.md)

---

## 1. Problem & Motivation

[044](044-git-md-proactive-commit-vs-harness-ask-first.md) documented that
`docs/Git.md`'s "commit proactively" guidance is silently unenforceable
under harnesses (Claude Code among them) that require explicit user
authorization before `git commit` — and proposed fixing this by having
`docs/Git.md` say so explicitly, so the fallback becomes "ask the user to
commit" rather than "silently defer forever."

That proposed fix has not yet landed (044 is still Open), and the same
session that filed 044 reproduced the exact failure a second time,
immediately afterward, in the same sibling project (`weg`): two more
reviewed, tested tickets (002 — Nextcloud install; 011 — stderr/`--verbose`)
landed, were independently reviewed, and then sat uncommitted for the rest
of the session — through a full story/evergreen-docs pass, a
`/compact`, and a `/context` check — until the user asked, unprompted,
"did we commit as we went along?" and then had to separately say "yes,
commit it" before anything happened.

Nothing about this second occurrence is new information — it's the same
gap 044 already named, recurring because the fix is still just a proposal.
Worth recording anyway, because it's evidence for a specific claim: a
one-time documentation note in `docs/Git.md` may not be a strong enough
checkpoint on its own, since the same agent, in the same session, having
just personally written up the failure mode, still didn't self-trigger the
"ask to commit" fallback 044 proposes — it took another human question to
surface it, exactly as before.

## 2. Detailed Technical Specification

This sharpens 044's fix rather than replacing it:

- A passive doc note ("read 'commit proactively' as 'ask the user'") relies
  on the agent re-deriving, unprompted, that *now* is a checkpoint moment —
  the same inference that already failed to fire once in this session.
- A more durable version of the same fix needs an actual trigger condition,
  not just permission to ask: e.g. `docs/AgenticLoop.md`'s Phase 3→4
  handoff (review gate → retro/hygiene) explicitly listing "ask whether to
  commit" as a mandatory step of *leaving* Phase 3, not an optional
  courtesy — so it fires at a structural point in the loop rather than
  depending on the agent noticing the tension itself.
- Consider whether this belongs as a habit in `docs/AgenticLoop.md`'s
  retro/hygiene phase specifically: "before closing out a retro or writing
  a session story, check `git status` — uncommitted reviewed work is itself
  a retro finding."

## 3. Implementation & Verification Plan

1. Land 044's original fix first (the explicit harness-constraint note in
   `docs/Git.md`).
2. Additionally add the structural checkpoint above to
   `docs/AgenticLoop.md`'s Phase 3/4 description, not just the passive
   doc note — this issue's evidence is that the passive version alone
   didn't hold up even within the session that produced it.
3. No code changes. Verify by re-reading `docs/AgenticLoop.md` for a
   concrete, non-optional trigger point rather than general guidance.
4. Close together with 044 once both land.

---

## Implementation Plan

### Current state (verified 2026-09-04)

- **044 is Closed** — its fix landed: `docs/lang/Git.md:17` now carries the
  "Ask-first harness interaction" bullet (ask the user *at session kickoff*
  for standing commit authority).
- `docs/practices/AgenticLoop.md:96` already mirrors that in the sprint
  kickoff mechanics ("Establish commit authority upfront…").
- **The gap this ticket names is still open**: both existing notes fire at
  *kickoff*. Neither Phase 3 (`AgenticLoop.md:111-129`) nor Phase 4
  (`:130-137`) nor Phase 5 (`:138-149`) contains a checkpoint that fires at
  the *end* of verified work. Phase 5 ends with "Prepare clean, conventional
  commit messages" — preparing a message is not the same trigger as
  "uncommitted verified work is a finding, ask now."

So the residual scope is exactly step 2 of §3 above: the structural trigger,
docs-only, no code.

### Steps

1. `docs/practices/AgenticLoop.md`, **Phase 3 mechanics** (after the Review
   Checklist block, ~line 129): add a closing mechanic making the commit
   question part of *leaving* Phase 3, not optional courtesy — e.g.
   "**Exit condition**: Phase 3 is not complete until reviewed, verified work
   is either committed or the user has been explicitly asked to authorize the
   commit. Under an ask-first harness with no standing authority from kickoff,
   asking *is* the exit action — do not carry a clean, reviewed diff into
   Phase 4."
2. Same file, **Phase 5 mechanics** (~line 140, alongside the existing
   `harnez status` bullet): add "Run `git status` before writing the retro or
   session story — uncommitted reviewed work is itself a retro finding, not a
   background condition."
3. Same file, **section 4 Anti-Patterns** (~line 237, next to the existing
   *Blind Revert of Failed Work* bullet): add
   "❌ **Reviewed-But-Uncommitted Carryover**: finishing a review gate and
   moving on (retro, story, `/compact`, next ticket) with verified work still
   in the working tree, waiting for the user to notice. Two occurrences in one
   `weg` session (issues 044, 046)."
4. Do **not** re-edit `docs/lang/Git.md` — 044's note is landed and correct;
   this ticket deliberately adds the trigger elsewhere rather than
   strengthening the passive note.
5. Re-sync the copyable doc: `harnez apply` (installs to `~/.claude/docs/`),
   plus `harnez init --docs AgenticLoop` in downstream projects on their next
   sync. No `go` changes, so no `make install` needed.
6. Verify by re-reading the three edited spots: the Phase 3 wording must be a
   non-optional exit condition ("is not complete until"), not advice
   ("consider asking") — that distinction is the entire point of this ticket.
7. Close 046 (044 is already Closed; no joint close needed anymore).

### Design decisions / tradeoffs

- **Three placements, not one.** Kickoff (already landed) covers "may I",
  Phase 3 exit covers "now is the moment", Phase 5 covers "did we actually".
  This ticket's evidence is that a single passive placement did not hold, so
  redundancy across trigger points is the deliberate fix, not bloat.
- **Docs-only, no enforcement hook.** A `Stop`-hook that greps `git status`
  for uncommitted tracked changes was considered and rejected for now: it
  would fire on every session including deliberate WIP, and this repo prefers
  a doc trigger before a mechanism. If 046-style recurrence shows up a third
  time, file a follow-up for the hook rather than pre-building it.

### Risks / open questions

- Doc-trigger durability is unproven — the same failure mode could recur
  despite three placements. Mitigation is the recurrence itself becomes the
  evidence for escalating to a hook.
- Wording must not conflict with kickoff authority: if the user granted
  standing commit permission at kickoff, the Phase 3 exit action is *commit*,
  not *ask*. State both branches explicitly.

### Scope

**Small** — three bullet-sized edits to one doc plus a resync.
