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
