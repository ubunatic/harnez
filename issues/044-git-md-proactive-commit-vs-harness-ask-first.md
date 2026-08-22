# 044 — `docs/Git.md`'s "commit proactively" conflicts with harness ask-first defaults

**Status**: Open
**Category**: Documentation / Agentic Ergonomics
**Related**: [docs/Git.md](../docs/Git.md), [039 — Agentic Loop Practices](039-agentic-loop-practices-and-sprint-command.md)

---

## 1. Problem & Motivation

`docs/Git.md` (bundled into every harnez-managed project, e.g. via
`AGENTS.md`'s `<!-- harnez:bundled -->` block) instructs agents to:

> Commit proactively:
> - after intermediate steps once tests are clean
> - after finished features (with review pass)
> - between iteration attempts on a stuck bug

In practice, most coding-agent harnesses (Claude Code among them) run with
an operating rule that takes precedence over project convention: **never
run `git commit` without an explicit user request**, regardless of what the
project's own docs say. This is a sensible default at the harness level —
committing is a "visible to others / hard to fully reverse" action class —
but it means `docs/Git.md`'s proactive-commit guidance is silently
unenforceable in exactly the harnesses it's meant to guide, and nothing in
the doc or in `AGENTS.md` says so.

**Observed in the wild**: a session in a harnez-managed sibling project
(`weg`) implemented and independently-reviewed four tickets end to end
(scaffold, spec system, canary, a hardening pass) over several hours,
correctly following `docs/Git.md`'s "review before commit" step each time —
but never committed anything, because the harness's ask-first rule silently
won every time the proactive-commit guidance would have applied. The agent
noticed the tension only during a retrospective, not during the work
itself, and the six tickets' worth of reviewed work sat uncommitted and
unprotected for the whole session. See that project's
`docs/feedback/2026-08-22-first-provisioning-sprint.md` for the full
account.

## 2. Detailed Technical Specification

The fix is documentation, not code — `docs/Git.md` should acknowledge the
harness constraint explicitly rather than let it resolve silently:

- Add a short note near the "Commit proactively" section along the lines
  of: *"Some harnesses (e.g. Claude Code) require explicit user
  authorization before any `git commit`, overriding this doc's proactive
  guidance. In that case, read 'commit proactively' as 'ask the user to
  commit proactively' — surface the recommendation at the stated
  checkpoints (after a reviewed feature, between fix attempts) rather than
  silently deferring it to whenever the user happens to ask."*
- Consider whether `docs/AgenticLoop.md`'s Phase 3→Phase 5 handoff (review
  gate → retro) should explicitly include "ask the user whether to commit"
  as a Phase 3/4 step, since that's the natural point in the loop where the
  tension surfaces and currently isn't addressed by either doc.

## 3. Implementation & Verification Plan

1. Add the harness-constraint note to `docs/Git.md`.
2. Cross-check `docs/AgenticLoop.md`'s phase descriptions for the same gap
   and add an explicit "ask to commit" checkpoint if missing.
3. No code changes; verify by re-reading both docs for internal consistency
   (a harness-constrained agent following them literally should end up
   *asking* to commit at the right points, not silently skipping it).
4. Close once both docs make the ask-first fallback explicit.
