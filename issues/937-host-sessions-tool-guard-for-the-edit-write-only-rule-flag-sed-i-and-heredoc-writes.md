# 937 — host sessions: tool guard for the Edit/Write-only rule (flag sed -i and heredoc writes)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: story `docs/studies/2026-10-25-four-days-of-epics-server-monitor-and-skills.md`

---

/goal A harnez PreToolUse hook (opt-in via `harnez apply`) warns or blocks when a Bash command edits a tracked file
with `sed -i`, `perl -i`, a heredoc or `>` redirect into the repo, pointing to Edit/Write; tests cover hits and
near-misses (reading with sed, writing to the scratchpad); or stop and report when the owner prefers prose only.
Circuit breaker: on a loop or no progress, clear the goal and write a failure report here.

## Evidence

The owner's rule is "edit files only with Edit/Write". Host sessions broke it at least four times between
2026-10-22 and 2026-10-25 (ticket table sed, heredoc study appends), each time admitted afterwards.

## Open decision (owner)

Warn or block; which repos opt in.
