# 550 — harnez apply installs harnez-agy launcher in ~/.local/bin to shim agy

**Status**: Closed — M1 cd667b5+7add6b8+e7c74bf+427592a; installed, live check: shim first on PATH, ANTIGRAVITY_AGENT=1
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

## Findings 2026-09-24 (host)

- Measured plain agy session 7e5f6ea1 (hook active, no shim): agy still truncates hook-rewritten
  command output (`<truncated 182 lines>`, `<truncated 707 lines>`), max ~8 KB per result; 69 KB of
  tool results over 17 calls. The hook does not bypass agy's truncation.
- Remaining cost of the hook: agy adds "A pre-tool hook changed the arguments of this tool call
  before it ran. Changed: CommandLine." to every command result, and agy runs many single commands.
  The shim avoids that line. This is the reason for the launcher (user, 2026-09-24).
- Preflight: shim path code exists (`internal/claude/apply.go` BashShimPath, `internal/subagent/agy.go`
  agyLaunchEnv, `cmd/harnez/hook.go` shim-aware route); no `harnez-agy` launcher exists.

## M1 — harnez-agy launcher (single milestone)

- `harnez apply` writes `~/.local/bin/harnez-agy` and provisions `~/.harnez/shims/bash`.
- `harnez-agy "$@"` starts the real `agy` with the same env as `agyLaunchEnv` (shim PATH first,
  agent marker). It must not find itself or recurse, and must not touch `~/.local/bin/agy`.
- With the launcher, the hook must not rewrite (537 fallback rule), so the "pre-tool hook changed"
  line disappears. Verify by test on the hook route decision.
- `revert --managed` removes the launcher. Idempotent on re-apply.
- Acceptance: unit tests for file content/mode, env equivalence with agyLaunchEnv, revert; `make test-q1` green.

## M1 review (host) — cd667b5: fixes required

- `make test-q1` (host): `TestUnifiedCrossHarnessSkillTargets` fails, "DiffAll reported changes
  immediately after ApplyAll" (claudeskills_test.go:338). DiffAll checks the launcher unconditionally,
  but ApplyAll installs it only inside the bash-shim condition. Guard the DiffAll check (and CleanAll)
  with the same condition, or install unconditionally; apply and diff must agree.
- CleanAll: do not `os.Remove(filepath.Dir(launcherPath))`; never try to remove `~/.local/bin`.
- Launcher script: indentation mixes tabs and spaces; use spaces only.
- Round 2 (7add6b8): does not compile, `internal/claude/apply.go:1585: declared and not used: shimPath`.
  Remove the leftover variable. Build with `go vet ./internal/claude/` before the test run.
- Round 3 (e7c74bf), root cause found by host: `gearSymlinkTargets` (apply.go ~831) adds
  `~/.local/bin/⚙` only if `~/.local/bin` exists. ApplyAllVariant computes the ⚙ targets before
  EnsureHarnezAgyLauncher creates `~/.local/bin`, so DiffAll then expects a ⚙ that apply never wrote.
  Fix: in ApplyAllVariant, install the launcher (and shim) before the ⚙ symlink step. Verify with
  `go test ./internal/claude/ -run TestUnifiedCrossHarnessSkillTargets -count=1` first.
