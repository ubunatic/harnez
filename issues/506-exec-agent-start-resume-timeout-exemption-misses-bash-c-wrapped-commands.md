# 506 — exec: agent start/resume timeout exemption misses bash -c wrapped commands

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: 268 (exec default 60s timeout), `cmd/harnez/exec.go` (`isAgentLongRunningCommand`, `isHarnezInvocation`), `harnez exec hook`

## /goal

`harnez agent start` / `resume` launched from an agent's Bash tool is never
killed by the implicit 60s `harnez exec` default, however the hook wraps the
command — including compound commands (`cd x && harnez agent start … | tail`)
rewritten to `bash -c '…'`.

## Observed

From a Claude Code session in `~/projects/cati`, dispatching a codex:luna:low
developer agent:

```
cd /home/uwe/projects/cati && harnez agent start --name bench-056 --model codex:luna:low ... 2>&1 | tail -30
```

The hook ran it as `⚙ bash -c '<command>'`. Both foreground (Bash tool
`timeout: 600000`) and background runs died after 60s with
`harnez exec: timeout kill after 1m0s` (exit 137). The agent had already
edited files; no session was saved (`harnez agent list` empty), so the work
could neither be resumed nor cleanly attributed.

## Cause

`isAgentLongRunningCommand` only inspects `args[0..2]`. For a hook-wrapped
compound command `args[0]` is `bash`, so the exemption never matches and the
60s default applies.

## Notes

- Fix direction (to decide): detect `harnez agent start|resume` inside a
  `bash -c` script, and/or have the hook itself pass an explicit `--timeout`
  (or none) when the script contains an agent turn. Keep the exemption table
  in sync with gear alias handling (2fff1fc).
- The Bash tool's own `timeout` parameter is invisible to `harnez exec`;
  consider documenting that `exec.timeout` / `--timeout` are the only knobs.
- Secondary: a killed `agent start` should still persist the session (or
  report its id) so partial work is resumable.
- Re-verify against current `exec.go` and hook rewrite before starting.

## Plan

- Recognize `harnez`/gear `agent start|resume` invocations in the script argument of `bash -c`, using a quote-aware shell token scan so command boundaries and quoted arguments are respected. Keep timeout resolution order intact: explicit `--timeout`, then repo `exec.timeout`, then the long-running exemption, then the implicit 60s default. Avoid changing the hook rewrite contract.
- Update `cmd/harnez/exec.go` (`isAgentLongRunningCommand`, with a small helper for `bash -c` script recognition if needed) and `cmd/harnez/exec_test.go`. Cover direct and wrapped start/resume; compound `cd … && harnez agent start …`; pipeline `harnez agent resume … | tail`; normal/variation-selector gear aliases in scripts; unrelated bash scripts; and explicit flag/config timeouts still applying to wrapped commands. Exercise `formatGearRewrite`/`runExecHook` to confirm the hook's compound-command rewrite reaches the same exempt path.
- Investigate the secondary killed-session loss separately in `cmd/harnez/agent_run.go` (`runStart`/`runResume`): session persistence currently follows a successful provider turn, so process-group SIGKILL prevents the final save. Specify a safe interruption/finalization path that persists a resumable session or reliably reports its provider/session ID, with tests for cancellation/kill semantics; do not conflate this lifecycle change with timeout detection if it needs a distinct design.
