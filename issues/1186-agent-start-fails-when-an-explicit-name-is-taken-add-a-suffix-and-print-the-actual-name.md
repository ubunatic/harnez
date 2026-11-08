# 1186 — agent start fails when an explicit --name is taken; add a suffix and print the actual name

**Status**: Closed — Taken explicit --name now gets first free -N suffix with stderr notice (suffixTakenName in cmd/harnez/agent_run.go, SuffixedName in internal/subagent/name.go), in runStart and detached start. TestAgentStartFileOnlyAndNameCollision updated (fails without fix), TestSuffixTakenName added; make test-q1 exit 0. Jules impl-1186 ended without a PR; host implemented.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1086, 1108

---

/goal When `harnez agent start --name X` finds X taken, the session is created as X plus a short suffix, and the actual name is printed where callers read it; covered by a test that fails without the fix. Stop and report when blocked on a user decision or denied permission; on a loop or no progress, clear the goal and write a failure report here.

## Problem

Session names are global across repositories on one machine. `FileSessionStore.Create`
(`internal/subagent/session.go:242`) returns `ErrSessionNameInUse` when any stored session,
finished ones included (kept on purpose, see 1086), has the same name or ID
(`session.go:259-261`). An explicit `--name` that is taken makes the start fail.

Reported by the taia session as taia#146 (confirmed twice by Jules audits) and hit again on
2026-11-08: `taia-jules-audit-144` was refused with "names are global across repos".

## Want

- On a taken explicit name, append a short suffix (reuse the existing fallback-suffix helper in
  `internal/subagent/name.go` if it fits) and create the session under that name.
- Print the actual name on stderr in one line (e.g. `name X taken; using X-2`) and in the
  start output's session line, so hosts resume by the real name.
- Keep finished sessions holding their names (1086) and keep the ID collision check as an error.
- Test: create session X, then start/create X again; assert the second gets a suffixed name and
  the message names it.

## Notes

Unverified: whether callers rely on the error to detect a duplicate start; check
`cmd/harnez/agent_run.go` callers of `Create` and `ErrSessionNameInUse` before changing the contract.
