# 537 — agy: route shell commands through harnez exec via hooks.json

**Status**: Open
**Priority**: P1
**Severity**: Medium
**Category**: Bug / Agents
**Related**: [[532-quota-1-counts-suspended-killed-test-runs-agent-turns-wait-forever-on-stopped-commands]], [[533-harnez-clean-reap-stuck-processes-and-release-stuck-quota-1-state]], [[196-agy-native-hooks-plan-alongside-claude-hooks]], [[209-move-agy-hooks-management-into-harnez-apply]], [[199-research-codex-hook-surface-for-transparent-exec-distill]]

---

## Problem

Claude (PreToolUse hook) and Codex (`internal/codex/hooks.go`) rewrite every shell command to
`harnez exec`. That gives each command the 60s default timeout, a process group that is killed on
timeout, and (since ef0408b) stopped-child recovery. agy's shell commands bypass `harnez exec`: the
harnez agy hooks were retired (209) and never rewired. A hung or stopped command therefore blocks an
agy turn indefinitely. This is 532 item 3: a `go test` stopped for 10h, and the turn waited ~30 min.

## /goal

Every agy shell command runs under `harnez exec`, just like Claude and Codex. Only harnez launches agy.

## Notes

- Canary first (docs/Canary.md): check that the current agy `hooks.json` has a pre-shell-command
  hook that can rewrite (not only observe) the command, and what its payload and response look like.
  196 has earlier findings. If rewriting is not possible, record that here and stop.
- Reuse the Codex adapter's shape (a per-provider adapter that emits the rewrite) rather than a new mechanism.
- `harnez apply` owns the managed hook entry, and `revert --managed` removes it (see 209's
  preserve/decommission behaviour).
- Acceptance: an agy session that runs `sh -c 'kill -STOP $$'` gets exit 125 back within seconds,
  and a `sleep 120` hits the exec timeout.

## Milestones (lean sprint, 2026-09-24)

Preflight: `internal/agy/hooks.go` already installs PreToolUse/PostToolUse (`harnez hook agy`, `agy-post`),
but it is observe-only ("without rewriting commands"), so the premise holds.

### M1 — canary: can agy's PreToolUse rewrite run_command?
- Probe the installed agy (`~/.local/bin/agy`): check whether a PreToolUse response can replace the
  `run_command` command line, and record the exact payload and response schema here. Use a throwaway
  hooks file or a scratch HOME, not the user's live hooks. If a rewrite is impossible, record that, stop, and report.

- Pre-Work / Required Refinements (from the plan review):
  1. Only harnez calls agy. Run the probe via `harnez agent -p --model agy:gemini-3.7-flash:low ...`, never `agy -p`.
  2. Prefer a scratch HOME (copy only the oauth token into it) over editing the live
     `~/.gemini/antigravity-cli/hooks.json`. If agy refuses a scratch HOME, you may edit the live file
     with a byte-identical snapshot and restore it. Only do that when `harnez agent list` shows no active
     agy session, and verify the restore with `cmp`.

#### M1 findings (2026-09-24)

The installed agy customization guide documents this `PreToolUse` input shape for a shell call:

```json
{
  "toolCall": {
    "name": "run_command",
    "args": { "CommandLine": "<command>" }
  },
  "stepIdx": 19,
  "conversationId": "<id>",
  "workspacePaths": ["<path>"],
  "transcriptPath": "<path>",
  "artifactDirectoryPath": "<path>",
  "modelName": "auto"
}
```

The documented rewrite response is:

```json
{
  "decision": "allow",
  "overwrite": { "CommandLine": "<replacement>" }
}
```

`overwrite` is documented as a shallow merge into the tool-call arguments, so replacing
`CommandLine` is supported by the documented contract. The live canary did not run: the required
`harnez agent -p --model agy:flash37:low` invocation was rejected before agy launch because this
worker has the developer leaf role; the CLI directs live agy checks to the host. Therefore the
installed agy's runtime rewrite behavior and actual payload remain unverified. No live hooks were
modified. The host must run the canary to complete M1.

**M1 delivered (canary, docs only): d76e6a6.** The documented response is `decision` + `overwrite.CommandLine`.
Live behaviour is unverified because developer leaf roles cannot launch `harnez agent`, so the host runs the live check in M3.

### M2 — rewrite agy run_command to `harnez exec`
- In `harnez hook agy`, rewrite `run_command` to `harnez exec --tool agy -- <cmd>`, using the same
  adapter as the Claude/Codex exec hook (look at `harnez exec hook` and `internal/codex/hooks.go`).
  Keep the existing telemetry. Do not double-wrap commands that already start with `harnez exec`.
- Tests: rewrite, no double-wrap, non-shell tools untouched.

**M2 delivered (unconditional hook rewrite): 3566199.** This is live, and it brings back the UI leak that 271 retired.

## Design change (2026-09-24, with the user): hook as a fallback, shim as the quiet path

History: 271 retired the hook rewrite because agy's chat shows the rewritten command. The bash PATH
shim (`~/.harnez/shims/bash`, 271/272) was meant to be the quiet path, but it is not installed on this
machine, and `harnez agent` never put it on agy's PATH, which is why 532 happened.

### M3 — rewrite only when the shim is inactive; record the chosen route
- The hook always records the command (the existing telemetry).
- If agy's `PATH` (the hook's own env) starts with `~/.harnez/shims` and `~/.harnez/shims/bash` is
  executable, pass the command through unchanged (route `shim`). Otherwise rewrite to `harnez exec
  --tool agy` (route `hook`). Record the route with the hook telemetry row.
- Unknown: does agy pass its PATH to hooks? The fallback is to always rewrite (safe). M6 verifies it live.
- Tests: shim active → unchanged; shim missing or not on PATH → rewrite; route recorded.

**M3 delivered (shim-aware routing): 06494de.**

### M4 — `harnez agent` sets up the shim for agy
- Pre-Work (from the M3 review): `agyCommandRoute` returns `hook` for commands that already start with
  `harnez exec`, although the hook does not rewrite them. Give them their own route (e.g. `direct`) so
  that M5 does not miscount them.
- Launch agy with `PATH=~/.harnez/shims:$PATH` and `ANTIGRAVITY_AGENT=1` (as `env.sh` does).
  Install or refresh the shim if it is missing, using the same code as `apply`.
- Tests: the launch env has the shims first, and a missing shim is created.

**M4 delivered (shim provisioned for managed launches): 7f43412.**

### M5 — coverage check in `harnez stats`
- Pre-Work (from the M4 review): M4 only covers the `-p` launch in `internal/subagent/agy.go`. Interactive
  `harnez agent chat` with an agy model launches via `internal/subagent/interactive.go` and gets no
  shim env. Apply the same `EnsureBashShim` + `agyLaunchEnv` there (for agy only), with a test.
- Per agy session: commands the hook saw vs `harnez exec` rows, split into via-shim / via-hook /
  unrouted / double-wrapped. Unrouted or double-wrapped counts are the alarm.

**M5 Pre-Work (interactive launch):** `harnez agent chat` and attach both pass through
`internal/subagent/interactive.go`; before this milestone they inherited the parent environment
without ensuring the managed bash shim. The agy interactive child now calls the same
`EnsureBashShim` helper as the print-mode driver and gets `agyLaunchEnv` (shim first on PATH and
`ANTIGRAVITY_AGENT=1`). Other providers retain their prior environment.

**M5 delivered (coverage + interactive launch):** `harnez stats --agents --days N` now emits a
per-conversation AGY shell-coverage section, joining `hook:prep` observations to `agy` shell rows
by Antigravity conversation ID and time. Matched routes are split into `via-shim`, `via-hook`, and
`via-direct` (already-wrapped calls, kept distinct); observed commands without an exec row are
`unrouted`, and surplus exec rows are `double-wrapped`. Exec rows without any hook observation are
also shown as unrouted. JSON includes the same data as `agy_coverage`. Fixture tests cover route
matching, absent/surplus rows, table rendering, and interactive shim provisioning. Live coverage
remains host-verifiable in M6.

### M6 — live acceptance (host-run)
- A `harnez agent -p` agy run: `sh -c 'kill -STOP $$'` returns exit 125 within seconds, `sleep 120`
  hits the exec timeout, agy's chat shows plain commands (shim route), and the M5 view shows 0 unrouted.
- Then mark 273 as superseded by 537 (the hook becomes the automatic fallback instead of an opt-in).

**M5 delivered (coverage view + interactive chat env): f96d816.**

**M6 live result (host, 2026-09-24, `agy:flash37:low` via `harnez agent start`, scratch repo):**
- `sh -c 'kill -STOP $$'` → exit 125 in about 1s ("stopped process group detected; sent SIGCONT"). ✅
- `sleep 120` → exit 137 after 60s ("harnez exec: timeout kill after 1m0s"). ✅
- `~/.harnez/shims/bash` was created by the launch (15:39). ✅
- ❌ Coverage view for that session `3e5fefeb…`: HOOK 2, EXEC 0, UNROUTED 2, which is false. Both commands
  demonstrably ran under `harnez exec`. The exec rows written via the shim are likely not attributed to the
  agy session/conversation id (M5's join assumed `ANTIGRAVITY_AGENT=1` + the conversation id resolve in both).

### M7 — attribute shim exec rows to the agy session
- Find why exec rows from the shim route don't join (missing or different session id, agent id,
  or timestamp window), and fix the attribution or the join. Tests: the fixture reproduces a shim-route
  pair and it counts as VIA SHIM.
- Acceptance (host): rerun `harnez stats --agents --days 1`; session `3e5fefeb…` (or a fresh live
  run) shows 0 UNROUTED and the commands under VIA SHIM (or VIA HOOK if the shim was not first on PATH;
  the view must say which).
