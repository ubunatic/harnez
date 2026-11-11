# 1112 — Auto mode refuses Bash calls the gear-shim hook rewrote, doubling tool calls

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## Problem
Reported 2026-11-03 by the lmcoder session. The harnez PreToolUse hook rewrites Bash calls to `⚙ ...` (for example `HTO=0 ⚙ bash -c 'cd <repo>; ...'`); see `formatGearRewrite` in `internal/exec/exec.go`. In Claude Code auto mode, the call is then refused with "a hook changed this call's input after auto mode reviewed it". The agent must resend the rewritten command verbatim, so every rewritten command costs two tool calls. Five background calls hit this in one batch.

`⚙` is the harnez multicall symlink installed by `harnez apply` (`GearSymlinkTargets`, `internal/claude/apply.go`). It runs the command through `harnez exec`, which records telemetry and can distill output.

## Goal
/goal In auto mode, Bash calls go through ⚙ without a refused first attempt. Stop and report if Claude Code offers no supported way to do this.

## Options
- Detect auto mode in the hook and skip the rewrite. Telemetry for those calls is then lost: the bash shim does not cover Claude sessions (see the code check below).
- Tell agents to write `⚙ <cmd>` themselves, so the hook has nothing to rewrite. Check that the hook passes such commands through unchanged.

## Done when
- A hook test shows no `updatedInput` for the chosen auto-mode path, or for an already-routed `⚙` command.
- The agent docs say what agents should type.

## Findings 2026-11-03 (lmcoder)
- A call that already starts with `⚙ bash ...` passes through unchanged when it is the only call in the response.
- If the hook rewrites even one plain call in a response with several Bash calls, auto mode refuses every other call in that response ("another call in this response was changed by a hook"). Each one must be resent alone.
- Writing `⚙` on every call works, but one forgotten prefix breaks the whole batch. A per-agent habit is fragile, so the hook-side fix (skip the rewrite in auto mode) is the better route.

## Finding 2: a self-written ⚙ on a background call gets the 1m timeout (lmcoder, 2026-11-03)
- When the hook rewrites a background call, it adds `HTO=0` (`internal/exec/exec.go`, the `run_in_background` branch). A call that already starts with `⚙` is skipped by `alreadyRoutedThroughExec`, so it never gets `HTO=0`.
- With `⚙ bash script.sh`, where the script runs `HTO=0 harnez agent start ...`, the outer `harnez exec` killed the call after 1m ("timeout kill after 1m0s"). One Jules start was cut off after the session had begun; another was killed during a `sleep 90` before the start ran.
- Workaround: write `HTO=0 ⚙ ...`. The /jules skill now says so.
- Fix direction: adding `HTO=0` in the hook is itself a rewrite, which brings back the auto-mode refusal. Prefer a fix inside `harnez exec`, such as no default timeout when it is not attached to a foreground tool call. That needs a reliable signal, so decide it together with the auto-mode fix.

## Code check 2026-11-03 (host, harnez at bc29b443)
Checked in the code and on this machine; nothing changed yet.
- **Rewrite** (`runExecHookWithStateDir`, `internal/exec/exec.go` ~1231-1259): every Bash call not already routed becomes `⚙ <cmd>` (simple command) or `⚙ bash -c '<cmd>'`. A `timeout` field becomes `--timeout=<d>`; `run_in_background` without a timeout gets an `HTO=0 ` prefix. Any `updatedInput` counts as a change, so every option that adds something (HTO, --timeout) brings back the auto-mode refusal.
- **Pass-through** (`alreadyRoutedThroughExec`, exec.go:1117): skips leading `VAR=val` words and passes the call if its first command is `⚙`/`⚙️` (`isGearInvocation`, `internal/exec/gear.go`). So `HTO=0 ⚙ ...` passes unchanged. It also passes any call whose text merely contains `harnez exec` anywhere (for example `grep "harnez exec" file`); such a call runs without the wrapper, and telemetry misses it.
- **Timeout** (`resolveExecTimeout`, exec.go:915), first match wins: an `HTO=`/`HARNEZ_TIMEOUT=` word in the args, the env vars, `--timeout`, `harnez agent ... --timeout`, unlimited for `harnez agent wait`, `config.yaml exec.timeout`, then 60s. `harnez agent start` and `harnez pr wait` are not exempt, so a self-written `⚙` without `HTO=0` kills either at 1m. Children inherit HTO, so an outer `HTO=0` also lifts nested `harnez exec` timeouts.
- **Bash shim** (`~/.harnez/shims/bash`, written by `internal/claude/apply.go` ~495) is used only when `~/.harnez/shims` is first in PATH (`agyBashShimActive`, `cmd/harnez/hook.go:439`, for Antigravity). It is not in this Claude session's PATH. Skipping the rewrite therefore loses telemetry and distill for those calls; it does not move them to the shim.
- **Auto-mode signal**: the Claude hook decoder reads only `tool_name`, `session_id`, `cwd` and `tool_input`. Claude Code sends `permission_mode` in hook payloads, but its value in auto mode is unchecked. Canary first (`docs/Canary.md`): record one real auto-mode payload before relying on it.

## Careful points for the fix
- Skipping the rewrite in auto mode also drops the 60s kill, since no wrapper runs. That is acceptable, but the docs must say so.
- If agents type `⚙` themselves, a background call needs `HTO=0 ⚙ ...` every time, unless `harnez exec` stops applying the default timeout to long-running harnez commands (`agent start`, `pr wait`), like the existing `agent wait` exemption. That exemption is small, testable and helps either route.
- Do not widen `alreadyRoutedThroughExec`. Narrowing its `harnez exec` substring match is a separate small fix.

## Progress 2026-11-03
- Done: `harnez exec` no longer applies its default 60s timeout to `harnez agent start`, `agent resume`, `agent wait` or `pr wait` (`isHarnezAgentWait`, `internal/exec/exec.go`; cases in `TestResolveExecTimeout_HarnezAgentWaitDefaultsUnlimited`). A self-written `⚙` on these no longer needs `HTO=0`. An explicit timeout still wins.
- Open: the auto-mode refusal itself (canary on `permission_mode` first).

## Audit 2026-11-08 (Jules)

Jules audit (GitHub PR 141, `reports/jules/1112.md`, checked against d8c61da; all 5 references hold): partly done. The timeout exemption (`internal/exec/exec.go:206`) and background `HTO=0` prefix (`exec.go:1257`) are in; the auto-mode refusal is still open. No code reads the hook input's `permission_mode` today. Live evidence the same night: a harnez host session in auto mode hit "a hook changed this call's input after auto mode reviewed it" on about 12 background `harnez agent start/resume` calls and had to reissue each one. Next step: canary first, logging the PreToolUse `permission_mode` value in auto mode (unverified that it is present or what value auto mode uses); then skip the rewrite in `runExecHookWithStateDir` for that mode, with a hook test.

## More evidence 2026-11-11

- loom session (loom `docs/studies/2026-11-11-tuios-research-and-jules-sweep-story.md`, sections 4 and 7): 11
  refusals in one session, all on reviewer and Jules `harnez agent start` calls rewritten to
  `HTO=0 ⚙ bash -c 'cd <dir> && HTO=0 harnez agent start ...'`. Its proposal: leave the input unchanged when it
  already has the `HTO=0`/`⚙` form, or have agents send the rewritten form first.
- harnez host session 2cd64224: 4 tool results with the same refusal
  ("auto mode classifier gave no verdict ... a hook changed this call's input").
