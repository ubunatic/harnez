# 466 — Detect Claude Stop-hook loops and break them (hooks, claude CLI, last resort logout)

**Status**: Open
**Priority**: P2
**Severity**: Moderate
**Category**: Feature / Discovery
**Related**: 034, `harnez hooks`

## Goal

harnez detects when a Claude Code Stop hook (for example a `/goal` condition check) keeps
returning the same unsatisfiable verdict and the session keeps answering it, and can break the
loop cheaply: warn the agent to stop replying, and if it persists, cancel or remove the hook.

## Observed

loom session 2026-09-21: a `/goal` said "haiku dev agents" after the plan moved to other
developers. The judge failed the condition after every turn, the agent replied each time, and about
10 to 20 near-identical turns (each with a full-transcript judge call) passed until the user ran
`/goal clear`. The agent could not clear the goal itself.

## Discovery (open questions, ideas only)

1. Detection: repeated `Stop hook feedback` messages, similarity of the last N hook texts and of
   the agent replies, or frequency (N within a time window). Where can harnez see this
   (`harnez hooks` transcript access, the session jsonl)?
2. In-session: can a harnez hook return a signal that stops the loop, or add a
   "do not answer, report once" rule to the agent context?
3. External: can harnez remove the goal/hook through the `claude` CLI, the settings file, or a
   session command (is there a supported way to run `/goal clear` for a session)?
4. Last resort: emergency `/logout` or killing the session. Probe first (Canary-first), and confirm
   with the user before any destructive action.
5. Prevention: warn when the developer set or plan changes while a `/goal` is active.

Record findings here before building anything.
