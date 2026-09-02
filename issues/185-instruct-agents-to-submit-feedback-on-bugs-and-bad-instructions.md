# 185 — Instruct Agents to Use `harnez feedback` When They Observe Bugs or Bad Instructions

**Status**: Blocked — depends on [issues/184](184-harnez-feedback-command-for-agent-filed-tickets.md)
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [issues/184](184-harnez-feedback-command-for-agent-filed-tickets.md) (the command this instructs agents to use), [issues/183](183-session-state-assessment-and-reminders.md) (the proactive-reminder mechanism this can piggyback on), [issues/181](181-narrow-harnez-rate-to-failure-cases.md) (precedent: narrow, event-triggered instruction rather than a chatty blanket one)

---

## 1. Problem & Motivation

Building `harnez feedback` (issue 184) only helps if agents actually call it. Left undocumented,
it will suffer the same fate the original blanket "rate every call" instruction did (see issue
181): either agents never discover the mechanism, or a heavy-handed instruction makes them call it
so often it becomes noise the user starts ignoring. This ticket is the "reverse-feedback" policy
and wiring: telling agents, at the right moments (not on every turn), to use `harnez feedback` when
they themselves notice a real bug or a gap/contradiction/bad instruction in the harness config
they're operating under.

## 2. Technical Specification / Findings

- Add a narrow, event-triggered instruction (global `CLAUDE.md`'s Tool Feedback Protocol section,
  or a new adjacent section/skill — decide during implementation which is the right home) that
  tells agents: when you observe a genuine harnez bug, a broken/contradictory instruction, or a
  clear gap in your own guidance, call `harnez feedback issue "<description>"` — do NOT call it for
  routine friction, uncertainty, or things you could resolve by asking the user. Mirror issue 181's
  hard-won lesson: scope the trigger condition tightly (observed defect, not "any time something
  feels off") so this doesn't become the next ignored, chatty mandate.
- Consider surfacing this as a proactive nudge via issue 183's session-state mechanism (e.g. if an
  agent's own turn included language matching "this instruction seems wrong" or "this looks like a
  bug" and no `harnez feedback` call followed) rather than only as a static always-loaded
  instruction — but keep v1 simple; a static instruction is an acceptable v1 on its own.
- This ticket is explicitly instruction/prompt-design work plus (optionally) light wiring into 183;
  it is NOT about building new detection logic to guess when an agent "should" have filed feedback
  — that's speculative scope creep unless 184's usage data later shows under-reporting.

## 3. Implementation & Verification Plan

1. Wait for issue 184's `harnez feedback` command to exist (or implement both in the same sprint if
   184 is done first).
2. Draft the instruction text; keep it short (aim for the same word-budget discipline issue 181's
   resolution note established, ~50-60 words) and event-triggered, not blanket.
3. Add it to `config.yaml`'s `agents_md.global.sections` (or a new skill, per whichever surface fits
   better) so it flows through `harnez apply` like the existing Tool Feedback Protocol section.
4. Optionally wire a session-state nudge per the note above if it's a small addition on top of
   issue 183's existing `sessionstate` package; otherwise leave as a documented follow-up.
5. Verify with `go test ./...`; run `make apply` to confirm the live `~/.claude/CLAUDE.md` reflects
   the new instruction; update `issues/README.md`.
