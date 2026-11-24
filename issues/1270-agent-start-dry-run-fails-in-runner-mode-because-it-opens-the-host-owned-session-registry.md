# 1270 — agent start --dry-run fails in runner mode because it opens the host-owned session registry

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 1181, 1186

---

## 1. Problem & Motivation

Reported by the splitty host session (taia, 2026-11-23; full report in
`/home/alfiz/projects/splitty/.harnez/inbox/taia-agent-dry-run-host-owned-registry.md`):

```
$ HTO=0 harnez agent start --dry-run --role developer --model claude:haiku:high --name dry "x"
Error: host session registry /home/alfiz/.harnez/agents is host-owned in runner mode; use launch-bound broker RPC for own session
```

Same for `--model agy:hydrogen:med`. `--dry-run` only previews the model, role and prompt and
writes nothing, so an agent in taia cannot check a model before dispatching it. The error also
does not say what an agent in taia should run instead.

Unverified: whether a real (non-dry-run) `agent start` fails the same way; the reporter started
two scouts successfully earlier in the same session.

## 2. Technical Specification / Findings

- `cmd/harnez/agent_run.go:570-593`: before the dry-run return at `:603`, `agent start` lists
  the session store to generate a name (`s.List`) and to suffix a taken explicit name
  (`resolveSession`, `suffixTakenName`, issue 1186).
- The store open goes through `guardHostSessionStoreAccess` (`internal/subagent/session.go:316`),
  which refuses the default registry when `runstate.InRunnerMode()` (issue 1181).

## 3. Decision, Goal & Verification Plan

/goal Unblock `harnez agent start --dry-run` and read-only session inspection (`OpenSessionStore` for `agent list`/`show`/`status`) in runner mode while keeping mutating `NewSessionStore` guarded, and update runner-mode mutation errors to name the `mcp__taia__background_task` route (resolving #1279 as well); or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural blocker, or no progress, clear the goal and record a failure report with resume point.

## Decision

- Combined `#1270` + `#1279` + Taia's read-only `OpenSessionStore` recommendation (autonomous `/decide`, 2026-11-24):
  1. In runner mode, `--dry-run` skips opening the session store (previews the requested/generated name and notes name uniqueness is checked at start).
  2. Read-only `OpenSessionStore` drops the runner-mode block so `agent list`/`show`/`status` work inside Taia (where Landlock already permits reading `~/.harnez/agents`), while mutating `NewSessionStore` keeps `guardHostSessionStoreAccess`.
  3. Runner-mode mutation errors in `guardHostSessionStoreAccess` name the actionable `mcp__taia__background_task` route (#1279).

- Test: runner mode plus `--dry-run` prints the preview and opens no store; read-only `OpenSessionStore` succeeds in runner mode; mutating `NewSessionStore` keeps the guard and names `mcp__taia__background_task`.
