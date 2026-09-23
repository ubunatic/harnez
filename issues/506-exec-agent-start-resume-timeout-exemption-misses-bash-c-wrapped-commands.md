# 506 — exec: agent start/resume timeout exemption misses bash -c wrapped commands

**Status**: Closed — Timeout intent parsing and hook forwarding implemented; make test-q1 passed
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

- ~~Does Claude Code's `updatedInput` replace the whole tool input?~~
  **Yes — canary 2026-09-23 (Claude Code, cati session):** with the hook
  rewriting, a `sleep 5` Bash call with `timeout: 2000` ran to completion,
  and a `run_in_background: true` call ran in the foreground. Control: the
  same calls prefixed `harnez exec -- …` (hook skips already-routed commands)
  timed out at 2s and backgrounded correctly. So the hook currently **drops
  `timeout` and `run_in_background`** for every rewritten Bash call. Fix:
  echo back the full `tool_input` with only `command` replaced (and forward
  the timeout to `harnez exec --timeout`). This is the likely root cause of
  the original kill, independent of the argv-shape exemption.
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

A plain `harnez agent resume` with pipe/parenthesis characters inside its
quoted prompt was also rewritten through `bash -c` and killed at 60s, so the
bug is not limited to shell compound commands outside the quoted prompt.

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

1. Fix the Claude hook first: in `cmd/harnez/exec.go`, preserve the full `tool_input` map and replace only `command`; forward its native `timeout` as `harnez exec --timeout`, and map `run_in_background: true` to no implicit limit. Canary proved `updatedInput` replaces the whole input, dropping both fields on every rewrite.
2. Add agent-agnostic `HARNEZ_TIMEOUT=0|<duration>` parsing to `cmd/harnez/exec.go`, both on direct argv/environment and in quote-aware `bash -c` scripts (including compound lists and pipelines). Document one precedence: explicit timeout prefix > forwarded tool-native timeout/background intent > repo `exec.timeout` > implicit 60s default. Make timeout-kill diagnostics name `HARNEZ_TIMEOUT=0` as the opt-out.
3. Fold the `agent start|resume` exemption into this shared explicit-intent mechanism, or remove the special case if redundant; avoid a separate argv-shape policy.
4. Update `harnez exec --help`, the installed agent instructions, and `docs/HookRewritePattern.md` with the syntax and precedence. Relevant code: `runExecHook`, its input/output types, `resolveExecTimeout`, timeout parsing, and timeout diagnostic in `cmd/harnez/exec.go`; likely tests in `cmd/harnez/exec_test.go` and instruction/template files under `internal/`.
5. Test Claude hook JSON round-trip preserving unknown `tool_input` fields while changing only `command`; native timeout/background forwarding; prefixes before commands and inside compound/piped `bash -c`; prefix/native/config/default precedence; and kill text. Re-run the canary with rewritten `sleep 5` plus `timeout: 2000` and `run_in_background: true`, then compare against the pre-hook `harnez exec --` controls: rewritten calls must retain the 2s timeout and background mode.
6. Keep session-persist-on-kill as a separate follow-up ticket or explicitly defer it; it is independent of timeout policy and needs its own lifecycle design/tests.

Hook payload check (2026-09-23): Claude supplies `tool_input`, where this hook currently decodes a generic map and can preserve fields. Codex's handler uses the analogous `tool_input.command`, but its own source calls that schema inferred and does not confirm a per-call timeout/background field; only hook-level `timeoutSec` is documented in the research. AGY uses `toolCall.args.CommandLine` and the current hook is observational, with no timeout/background field in its payload struct. Do not assume equivalent native fields for Codex/AGY without a payload canary; the explicit prefix remains the cross-harness path.
