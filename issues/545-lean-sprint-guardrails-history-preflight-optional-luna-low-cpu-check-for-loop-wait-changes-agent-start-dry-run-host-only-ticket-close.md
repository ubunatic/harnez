# 545 — Lean-sprint guardrails: history preflight (optional luna:low), CPU check for loop/wait changes, agent start --dry-run, host-only ticket close

**Status**: Open
**Priority**: P2
**Severity**: Medium
**Category**: Agents / Workflow
**Related**: [[544-handoff-2026-09-24-open-threads-from-the-harnez-bf-session-clean-agy-routing-exec-cpu-read-i-oom-cxxxe-rule]], [[537-agy-route-shell-commands-through-harnez-exec-via-hooks-json]], [[541-harnez-exec-busy-waits-at-1-core-stopped-group-monitor-scans-all-of-proc-every-25ms]], [[540-harnez-agent-short-model-aliases-opus-terra-low-and-a-correct-error-when-p-swallows-model]]

---

## Background

These come from the 2026-09-24 retro (`docs/feedback/2026-09-24-lean-sprints-clean-agy-exec.md`):
537 M2 rebuilt a design that 271 had retired, because nobody searched the history. 533 M4 shipped a
3-core CPU regression with green tests. An alias probe launched real agent runs. A developer closed
541 itself before the host had reviewed it.

## /goal

1. **History preflight, optional `luna:low` step.** The lean-sprint preflight may dispatch a
   `codex:luna:low` advisor (read-only) that searches closed and related tickets and evergreen docs on
   the ticket's subsystem (`harnez find issues <area> --all`, `docs/README.md`). It reports prior
   decisions that the plan must respect or explicitly revisit. It is optional, and the host decides per ticket.
2. **CPU check only for CPU-relevant changes.** Add a lean-sprint review item: when a diff adds or
   changes a wait, a poll, a loop, a ticker, a watcher or streaming, the host measures CPU (and RSS) of
   the real command (`ps -o %cpu,rss` while it waits). Other diffs skip this.
3. **`harnez agent start/-p --dry-run`.** It resolves the model, role, session and working directory,
   prints them, and exits without launching a provider. Use it for alias probes (540).
4. **Host-only ticket close.** `harnez issues close` refuses when it runs inside a developer or reviewer
   leaf session (as detected by the harnez agent role env), with a message to report back to the host
   instead. The host and the user can still close tickets.

## Notes

- Items 1–2 are skill/doc text (`docs/commands/lean-sprint.md`, sync the installed copy via `apply`).
  Items 3–4 are code with tests.
- Deferred by the user (handled after compaction via 544): 353 M2 and rolling out the managed block to other repos.

## Addition (2026-09-24): retire agy sessions with a large context

5. **`harnez agent resume` refuses agy sessions with a large context.** agy has no compact command
   (Google says it compacts in the background, but a harnez-queued `/compact` does nothing there). In the 543
   sprint one session reached 693k new tokens, and a single turn then used 21.5M tokens incl. cached.
   Above a threshold (e.g. 300k tokens), `resume` refuses with a message to start a fresh session, and
   `harnez agent` closes the old one. Lean sprints then use one fresh developer session per milestone,
   with the ticket as the handoff.
