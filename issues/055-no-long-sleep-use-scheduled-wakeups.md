# 055 — Agents must not use long `sleep` to wait; schedule a wakeup/BG task instead

**Status**: Closed — resolved
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [038](038-research-subagent-lifecycle-and-cleanup-friction.md), [052](052-headless-agent-cli-probes-for-idle-telemetry-refresh.md)

---

## 1. Problem & Motivation

Agents commonly wait for a slow or external condition (CI run, deploy,
build, remote queue, a background process finishing) by chaining `sleep N`
in a shell call, or looping `sleep` inside a poll. This is wasteful and
risky:

- A long-running `sleep` blocks the tool call and the agent's turn for its
  full duration, burning wall-clock time and context/turn budget even
  though nothing is happening.
- If the harness or session is interrupted mid-sleep, the wait is silently
  lost with no record of what was being waited for.
- Polling via repeated short sleeps in a loop is worse: it wastes cycles
  and tokens on empty checks instead of yielding control back until state
  actually changes.
- When the harness already offers a proper primitive for "come back later"
  (a scheduled wakeup, a cron/loop mechanism, or a background task the
  harness tracks and notifies on completion), sleeping instead of using it
  bypasses the notification path entirely — the agent (or user) has no way
  to know work finished except by the agent itself re-polling.

## 2. What's being requested

Document in the relevant practice doc (`docs/practices/AgenticLoop.md` or
a new short rule) that agents must not use long blocking `sleep` calls (nor
sleep-loops) to wait out external state. Instead:

1. Prefer letting harness-tracked background work notify on completion
   (e.g. a background process/task the harness itself watches) over
   polling for it at all.
2. When polling *is* necessary (external state the harness cannot track —
   a CI run, a remote deploy, a queue), use the harness's scheduled-wakeup
   or interval-loop mechanism, if the harness exposes one, so the agent
   yields control between checks instead of blocking on `sleep`.
3. Match the polling/wakeup interval to how fast the watched state
   actually changes — not a fixed short interval "just in case."
4. Never chain long leading `sleep` commands to work around a harness
   restriction on blocking sleep; that defeats the restriction's purpose.

## 3. Explicit non-goal

Not proposing to forbid all `sleep` usage — short sleeps inside scripts,
tests, or retry/backoff logic with strict bounds are fine. This is
specifically about agents using `sleep` as a substitute for a proper
wait/notify or scheduled-wakeup mechanism when one is available.

## 4. Progress

**2026-08-29**: Added a new "❌ Blocking `sleep` Waits" bullet to the
"Anti-Patterns to Avoid" list (section 6) in `docs/practices/AgenticLoop.md`,
covering: blocking sleeps/sleep-loops waste turn and context budget and
leave no record of the wait if interrupted; prefer harness-tracked
background-work notification over polling at all; when polling is
unavoidable, use the harness's scheduled-wakeup or interval-loop mechanism
(e.g. `/loop`, background-task notifications) instead of blocking `sleep`,
sized to the actual rate of change of the watched state; never chain long
leading sleeps to route around a harness restriction on blocking sleep.
Mirrored the same addition into the project's own installed copy,
`docs/AgenticLoop.md`, to keep the two in sync (no existing "ScheduleWakeup"
or background-task doc reference was found to cross-link instead — this is
the first place that mechanism is documented as a `sleep` alternative).
Did not touch issue 056 (buffered-output anti-pattern) or resync the
global `~/.claude`/`~/.prime` installed copies — that install step is out
of scope for this doc-only ticket and is normally triggered separately via
`harnez apply`.
