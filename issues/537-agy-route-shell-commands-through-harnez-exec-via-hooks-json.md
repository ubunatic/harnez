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

### M2 — rewrite agy run_command to `harnez exec`
- In `harnez hook agy`, rewrite `run_command` to `harnez exec --tool agy -- <cmd>`, using the same
  adapter as the Claude/Codex exec hook (look at `harnez exec hook` and `internal/codex/hooks.go`).
  Keep the existing telemetry. Do not double-wrap commands that already start with `harnez exec`.
- Tests: rewrite, no double-wrap, non-shell tools untouched.

### M3 — live acceptance
- A real `harnez agent -p` agy run with `sh -c 'kill -STOP $$'` returns exit 125 within seconds,
  and `sleep 120` hits the exec timeout.
