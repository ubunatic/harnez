# 075 — ConciseMode: Promote From Passive Doc to a Real, Invocable Skill

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics & UI Standards
**Related**: [[065-concisemode-caveman-skill-and-output-distillation]], `config.yaml` (`skills:` section), `docs/practices/ConciseMode.md`

---

## 1. Problem

Ticket 065's title calls this the "Caveman **Skill**," and it's closed. What actually shipped is
`docs/practices/ConciseMode.md` — a passive reference doc, registered the same way as `Go.md` or
`Git.md`, surfaced only as a one-line hint in `AGENTS.md`/`CLAUDE.md`'s bundled conventions block
when a project opts in via `harnez init --docs concise-mode`.

This is **not** a real Skill in the sense `config.yaml`'s actual `skills:` mechanism produces (see
e.g. `evergreen`, `story`, `grilling` — each gets a generated `SKILL.md` installed to
`~/.claude/skills/<name>/`, invocable as `/name`). There is no `/concise-mode` command, no
selectable tier, no enforcement — just a hint an agent may or may not act on, identical in
mechanism to every other convention doc already in this repo.

This gap was flagged mid-session when the user asked whether restarting a session with the doc
newly wired in would make the agent "adhere to" ConciseMode — the honest answer was no, not
reliably, because nothing about the mechanism enforces it. That answer is not recorded anywhere
except this ticket.

## 2. What "Real" Would Look Like

- Register under `config.yaml`'s `skills:` list with a generated `SKILL.md`, invocable via
  `/concise-mode <lite|standard|ultra>` (or similar), so a user/agent can explicitly select a tier
  for a session rather than hoping a hint gets noticed.
- Decide whether tier selection should persist per-session, or be a one-shot instruction — open
  design question, not resolved here.

## 3. Implementation & Verification Plan

1. Design the skill invocation shape (flags/args for tier selection) — needs a decision, not
   assumed here.
2. Add a `skills:` entry in `config.yaml` plus the generated `SKILL.md` content (reuse
   `docs/practices/ConciseMode.md`'s tier definitions as the source of truth, don't duplicate them).
3. Verify via `harnez apply` + a real session restart that `/concise-mode` actually appears and
   measurably changes response style — not just that the file was written.
