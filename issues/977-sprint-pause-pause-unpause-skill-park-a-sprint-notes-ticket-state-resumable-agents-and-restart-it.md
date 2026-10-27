# 977 — sprint-pause pause|unpause skill: park a sprint (notes, ticket state, resumable agents) and restart it

**Status**: Open
**Priority**: P2 (Medium) — saves the host from rebuilding sprint state by hand after each interruption
**Severity**: Minor
**Category**: Workflow
**Related**: 851 (bug-hunt skill, same pattern), 908 (epic: agent sessions and liveness)

---

/goal Add a `sprint-pause` skill (source `docs/commands/sprint-pause.md`, mapped in `config.yaml`, installed by
`harnez apply`) with `pause` (default) and `unpause` (alias `play`) modes, verified by one real pause and one real
unpause on this repo, or stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation

A sprint can be interrupted at any point (usage limit, end of day, another machine). Today the host has to
reconstruct from memory which subagents are still running, which of them did real work, and where each ticket
stands. The skill makes a pause cheap: it stops idle agents, leaves short notes, and updates the tickets. Later,
`unpause` reads those notes and restarts the sprint.

This is a pause, not a report. Nothing from the developer agents' output goes into tickets; the tickets only get
status, the agent names, and a rough progress point.

## 2. Technical Specification

### `pause` (default)

1. **Stop idle agents.** List running `harnez agent` sessions (`harnez agent list`, `harnez agent status --name <session>`).
   Stop each one that is not expected to finish within 10–20 seconds (`harnez agent stop --name <session>`). Let
   near-finished ones complete.
2. **Note resumable agents.** For each agent that did non-trivial work, record its name and id, and whether resuming it
   later is worth more than a fresh agent. Assume developer agents are resumable for a while; the `unpause` run
   confirms this per agent.
3. **Update the tickets worked on.** Add the agent names and a rough progress point ("where it was") when this is
   visible without deep digging: the last commit, the open milestone, the failing test.
4. **Update ticket status.** Best effort. Where a status is an assumption, say so in one short line in the ticket.
5. **Other docs and studies.** Update only what can be done quickly to capture findings up to the pause point.

Pause notes go to `.harnez/sprint-pause.md` (git-excluded like `Local.md`; per-checkout state, not shared docs). This
file is the single input for `unpause`.

### `unpause` (alias `play`)

1. Read `.harnez/sprint-pause.md` and the tickets it names.
2. Re-verify each ticket against live code and recent commit history before acting (the Issue Handling rule), since
   the project may have moved while paused.
3. For each noted agent, try `harnez agent resume --name <session> <prompt>` in the background (`HTO=0`, one call per
   worker). A failed resume is reported and the work is redispatched as a fresh agent.
4. Continue the sprint from the recorded point.

## 3. Implementation & Verification Plan

- Write `docs/commands/sprint-pause.md` with the two modes above, add a `config.yaml` skill entry with a one-line
  description and `user-invocable-only`, then run `make install` and `harnez apply`.
- Verify with the same round-trip approach as `bug-hunt` (installation asserted by the Claude skill round-trip test).
- Verify live: one `pause` on an active sprint, then one `unpause`, checking that the notes file, the ticket
  updates, and at least one successful `harnez agent resume` are real.
- Run `make test-q1` once after the change.
