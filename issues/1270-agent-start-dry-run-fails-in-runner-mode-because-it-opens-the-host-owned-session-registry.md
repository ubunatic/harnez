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

## 4. Architect Refinement (`argon:med`, 2026-11-24) — M3

- **Read-only `OpenSessionStore` & read-only subcommands**:
  - `internal/subagent/session.go:307-314`: remove `guardHostSessionStoreAccess(dir)` from `OpenSessionStore(dir)` (which only reads via `os.ReadDir`/`os.ReadFile` and never writes/mkdirs). Keep `guardHostSessionStoreAccess(dir)` on mutating `NewSessionStore(dir)`.
  - `cmd/harnez/agent.go` & `cmd/harnez/agent_manage_cmds.go`: add `ac.openStore` (`subagent.OpenSessionStore(ac.storeDir)`) and use it in `listCmd`, `statusCmd`, `waitCmd`, and `/status` (while `/compact`, `/stop`, `start`, `resume`, `delete` keep `ac.store()`). Verify `resolveSession` / `canManageTarget` (`cmd/harnez/agent_args.go:43-51`) does not block read-only `status`/`wait` in runner mode.
- **`agent start --dry-run` in runner mode**:
  - `cmd/harnez/agent_run.go:563-605`: when `req.DryRun && runstate.InRunnerMode() && req.SessionID == ""`, skip `d.store()`, use `req.Name` or `subagent.GenerateSessionName(nil)`, print preview via `printAgentPreview`, and print `note: name uniqueness is checked against the host session registry at start\n`. Also skip `d.store()` in `withHostTrackingLine` (`cmd/harnez/agent_args.go:209-227`) when `--dry-run` is set.
- **Actionable runner-mode errors (#1279)**:
  - `internal/subagent/session.go:316-333` (`guardHostSessionStoreAccess`): include `dispatch via taia background_task (mcp__taia__background_task) or use launch-bound broker RPC for own session` (preserving `"host-owned"`).
  - `cmd/harnez/agent_cmds.go:358`, `cmd/harnez/agent_run.go:918`, `cmd/harnez/agent_manage_cmds.go:265,326`, `cmd/harnez/agent_stop.go:64`: include `; run via taia background_task (mcp__taia__background_task)` (preserving `"host-only"`).
- **Tests**:
  - `internal/subagent/runner_registry_test.go`: `NewSessionStore` still fails closed naming `mcp__taia__background_task`; `OpenSessionStore` succeeds in runner mode without creating dir.
  - `cmd/harnez/agent_dry_run_test.go`: `TestAgentStartDryRunInRunnerModeSkipsStore` (with and without `--name`, prints preview + uniqueness note, creates no store dir).
  - `cmd/harnez/runner_registry_lifecycle_test.go`: `agent list` and `agent status` succeed in runner mode on pre-populated host registry; mutating `agent start` (non-dry-run), `resume`, `stop`, `delete` fail closed and name `mcp__taia__background_task`.

## 5. M3 Delivery & Review Fixes (Pre-Work)

- **M3 delivered (`d09a7d6a`)**: Unblocked runner-mode `--dry-run`, switched `list`/`status`/`wait`/`/status` to `OpenSessionStore`, and updated runner-mode errors to name `mcp__taia__background_task`.
- **Review (`codex:sol:med`, 2m26s) — Needs pre-work**:
  1. `[HIGH]` `OpenSessionStore` returned a writable `*FileSessionStore` and `waitForAgent` (`cmd/harnez/agent_async.go:538`) calls `store.Save` when a running worker disappeared. Fix: add `readOnly: true` on `OpenSessionStore` and fail closed in all mutating methods on `FileSessionStore` (`Save`, `Delete`, `Purge`, etc.); in `waitForAgent`, skip `store.Save` stale-session repair when read-only / in runner mode.
  2. `[MED]` Add tests asserting `OpenSessionStore(...).Save(...)` fails closed and leaves files untouched, and stale-running `agent wait` in runner mode does not mutate registry files.
  3. `[LOW]` Add a store-call spy in `TestAgentStartDryRunInRunnerModeSkipsStore` proving zero `store()` invocations (including through `withHostTrackingLine`).
