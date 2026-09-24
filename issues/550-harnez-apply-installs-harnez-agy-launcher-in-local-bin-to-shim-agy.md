# 550 — harnez apply installs harnez-agy launcher in ~/.local/bin to shim agy

**Status**: Open
**Priority**: P1
**Severity**: Medium
**Category**: Feature / Agents
**Related**: [[537-agy-route-shell-commands-through-harnez-exec-via-hooks-json]], [[271-decommission-agy-hooks-pretooluse-interception-in-favor-of-guarded-bash-path-shim]], [[272-add-harnez-env-sh-and-opt-in-shell-flag-for-harnez-apply-to-manage-agy-wrapper]], [[545-lean-sprint-guardrails-history-preflight-optional-luna-low-cpu-check-for-loop-wait-changes-agent-start-dry-run-host-only-ticket-close]]

---

## Problem

agy burned more of the plan than it used to. Suspect: the `hooks.json` pre-shell hook (537) rewrites
agy's commands to `harnez exec`, which forwards up to 256 KB head + 1 MB tail of output. agy's SDK
docs say large `run_command` output "may be truncated before delivery" (no limit given); whether the
hook rewrite bypasses that is unknown (web research 2026-09-24 found nothing).

State on this machine: `~/.harnez/shims/` does not exist, so the guarded `bash` PATH shim (271/537 M4)
is inactive and the hook is doing all routing. `~/.local/bin/agy` is the real agy binary. The shim is
only set up when harnez itself launches agy (`agyLaunchEnv`); an agy started by hand gets the hook.

## /goal

`harnez apply` installs a `harnez-agy` launcher in `~/.local/bin` that starts the real `agy` with the
shim PATH and agent env (same as `agyLaunchEnv`), and provisions `~/.harnez/shims/`. With the shim
active, the agy hook stays out of the way (537's fallback rule). `revert --managed` removes both.

## Notes

- Do not overwrite `~/.local/bin/agy` (the real binary). The user runs `harnez-agy` instead.
- Check first: is `~/.harnez/shims/` missing because `apply` no longer provisions it, or was it removed?
- Open question (canary): does agy truncate output from the shimmed bash, and from the hook-rewritten
  command? Measure new input tokens for a ~1 MB output command under hook vs shim vs neither. If
  neither path is truncated by agy, the byte cap on `harnez exec` output (context guard) is still needed.
- `apply` vs `init` scope (docs/CLIDesign.md): this is a global `~/` install, so it belongs in `apply`.
