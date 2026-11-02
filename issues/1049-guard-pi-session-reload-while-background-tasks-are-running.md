# 1049 — Guard Pi session reload while background tasks are running

**Status**: Backlog — grooming 1083 (2026-11-03): active list focused on taia support, P1s and open bugs; promote when needed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: 1011, 1047; `.pi/extensions/background-tasks.ts`

## Problem

After reloading a Pi session with three background sleeps, the task list and UI were empty; no sleep processes remained when checked. The timing did not establish whether reload killed the processes or they had already completed. Pi already warns that reload is unavailable while a prompt is running. The owner wants the same protection for running background tasks rather than trying to revive tasks across reloads.

## Goal

/goal Prevent session reload while managed background tasks are running and clearly warn the user, using Pi's existing reload warning mechanism where supported; verify the guard and normal reload after completion or cancellation, or stop and report if an API, user decision or permission is needed. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with a resume point.

Re-check live Pi APIs, task lifecycle and recent code first; investigate whether the reload warning can be extended or intercepted without assuming support. Cover multiple running tasks, completion/cancellation races and reload after tasks finish in automated tests and a real Pi session. Do not implement task revival across reloads as part of this issue; report any unsupported reload path or unavoidable limitation honestly.

## Discovery and blocker

A read-only Pi 1.4.0 source audit found the built-in interactive `/reload` checks streaming and compaction but exposes no cancellable pre-reload extension event; `session_shutdown` cannot veto teardown. A guard around an extension's own `ctx.reload()` cannot intercept the built-in command or other extensions' calls. A reliable guard therefore appears to require a Pi API change, such as a cancellable pre-reload hook honored across reload paths. Reverify against the installed Pi version before resuming; do not present a partial wrapper as full protection. No live reproduction of the API limitation was run in this discovery.
