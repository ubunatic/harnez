# 506 — exec: agent start/resume timeout exemption misses bash -c wrapped commands

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: 268 (exec default 60s timeout), `cmd/harnez/exec.go` (`isAgentLongRunningCommand`, `isHarnezInvocation`), `harnez exec hook`

## /goal

Any agent can deliberately lift or set the `harnez exec` timeout for **any**
Bash command — not just `harnez agent start|resume` — in one documented,
obvious way that survives the PreToolUse hook rewrite (including `bash -c`
wrapping of compound commands). The implicit 60s default stays as a safety
net only for commands where the agent expressed no intent.

Done when:

- **Explicit opt-out, agent-agnostic**: a documented command prefix works for
  every harness (Claude, Codex, agy), mirroring the existing
  `HARNEZ_EXPECT_FAILURE=1 <cmd>` convention, e.g. `HARNEZ_TIMEOUT=0 <cmd>`
  (no limit) / `HARNEZ_TIMEOUT=10m <cmd>`. Recognized before and after
  `bash -c` wrapping.
- **Tool-native intent is honored**: when the harness passes its own intent
  (Claude Code Bash `tool_input.timeout`, `run_in_background: true`), the hook
  forwards it as `harnez exec --timeout <that>` (background → no implicit
  limit) instead of silently imposing 60s.
- The `agent start|resume` special case either falls out of the above or is
  kept as one entry in the same mechanism — no separate argv-shape heuristics.
- Precedence is documented in one place (`harnez exec --help` + the agent
  instructions harnez installs): explicit prefix > tool-native timeout >
  repo `exec.timeout` > 60s default.
- The kill message names the opt-out (e.g. `timeout kill after 1m0s; rerun
  with HARNEZ_TIMEOUT=0 to lift`), so an agent hitting it learns the fix.

## Open questions

- Does Claude Code's `updatedInput` replace the whole tool input? The hook
  currently returns only `{command}`; if it replaces, `timeout` and
  `run_in_background` are dropped today. Canary this first (docs/Canary.md).
- Should the implicit 60s default apply at all to `run_in_background` calls?

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

## Plan (luna, 5ba246d — superseded in scope by the /goal above; revise)

- Recognize `harnez`/gear `agent start|resume` invocations in the script argument of `bash -c`, using a quote-aware shell token scan so command boundaries and quoted arguments are respected. Keep timeout resolution order intact: explicit `--timeout`, then repo `exec.timeout`, then the long-running exemption, then the implicit 60s default. Avoid changing the hook rewrite contract.
- Update `cmd/harnez/exec.go` (`isAgentLongRunningCommand`, with a small helper for `bash -c` script recognition if needed) and `cmd/harnez/exec_test.go`. Cover direct and wrapped start/resume; compound `cd … && harnez agent start …`; pipeline `harnez agent resume … | tail`; normal/variation-selector gear aliases in scripts; unrelated bash scripts; and explicit flag/config timeouts still applying to wrapped commands. Exercise `formatGearRewrite`/`runExecHook` to confirm the hook's compound-command rewrite reaches the same exempt path.
- Investigate the secondary killed-session loss separately in `cmd/harnez/agent_run.go` (`runStart`/`runResume`): session persistence currently follows a successful provider turn, so process-group SIGKILL prevents the final save. Specify a safe interruption/finalization path that persists a resumable session or reliably reports its provider/session ID, with tests for cancellation/kill semantics; do not conflate this lifecycle change with timeout detection if it needs a distinct design.
