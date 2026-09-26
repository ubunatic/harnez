# 605 — goal stop hook deadlocks when progress needs user input; reject goals without exit criteria

**Status**: Closed — Confirmed by voxi-75 after harnez init: the exit-clause rule covers its case
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Bug
**Related**: `/goal` session Stop hook (Claude Code), voxi issue 153 session 2026-09-27

---

## 1. Problem & Motivation
A `/goal` sets a session Stop hook that blocks stopping until its condition holds. When progress
depends on something only the user can give (here: a permission denied by the auto-mode classifier,
followed by a choice between two options), the agent cannot continue and cannot stop. The hook re-fires
after every reply, and the agent answers with the same "waiting for you" line. This repeated five
times until the user ran `/goal clear` by hand. It wastes turns and tokens, and it pushes the agent to
work around safety denials just to satisfy the hook.

Observed goal: `playback some demo texts with my "cloned" voice` (voxi). It was blocked by
`TORCH_FORCE_NO_WEIGHTS_ONLY_LOAD=1` being denied, which is correct behaviour for the denial.

## 2. Technical Specification / Findings
- The goal condition had only a success criterion. It had no exit or break criterion: no "stop when
  blocked on user input", no retry or turn budget, no time limit.
- The hook evaluator correctly saw that the condition was unmet, but it has no notion of "blocked,
  not failing", so it keeps blocking.

## 3. Implementation & Verification Plan
- **Reject goals without exit criteria:** when a goal is set, require (or auto-append) a break
  condition, e.g. "or the agent is blocked on a user decision / denied permission", plus a maximum
  number of hook re-fires.
- **Deadlock detection:** if the hook blocks N times (e.g. 2) with no tool call between blocks, or
  the agent's reply is a question to the user, release the stop and tell the user the goal is paused.
- **Verify:** reproduce with a goal that needs a denied permission; expect at most N re-fires,
  then a clean stop with a pause message.

## Host premise check (peer-assistant, 2026-09-27)

- The `/goal` Stop hook and its evaluator are Claude Code built-ins; harnez installs no goal hook (`harnez find code`).
  Deadlock detection and re-fire limits (§3 bullets 1-2) are upstream, out of harnez's reach.
- harnez-side fix: harnez guidance teaches agents to write `/goal` conditions (`docs/commands/issue.md`,
  `docs/commands/reverse-sprint.md`). Every goal it teaches must carry an exit clause, e.g.
  "... or stop and report when blocked on a user decision or a denied permission". Add that rule where goals are
  written and to `docs/practices/AgenticLoop.md` anti-patterns, with a test on the skill text.

- **M1 delivered (exit-clause rule in issue skill + AgenticLoop)** `fbd7659`.
- voxi-75 feedback (2026-09-27, voxi a9f76de): the clause works (evaluator judges strictly against the condition
  text), but the rule never reached the failing goal: the user typed `/goal ...` directly, the issue skill never
  ran, and AgenticLoop is read only after the goal is set.

## M2 — Pre-Work / Required Refinements

- Add an always-loaded agent-side rule to the managed AGENTS.md block (`config.yaml`): when a `/goal` without an
  exit clause is set, the first reply says so and offers the corrected `/goal ... or stop and report when blocked
  on a user decision or denied permission` line; once blocked on the user, suggest `/goal clear` instead of
  repeating the same wait message.
- Extend the managed-block test to assert the rule; `harnez init -d .` after `make install`.
- **M2 delivered (always-loaded goal rule)** `b5feb5e`, wording fix `169c088` (covers user-typed goals). test-q1 green, `harnez apply` done. Awaiting voxi-75 feedback after `harnez init`.
- voxi-75 (2026-09-27): the rule as worded would have covered its case; `harnez init` in voxi deferred until its developer dev-154 finishes (one writer). Close on its confirmation.
