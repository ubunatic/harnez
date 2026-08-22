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
