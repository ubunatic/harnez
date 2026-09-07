# 271 — Decommission agy-hooks PreToolUse interception in favor of guarded bash PATH shim

**Status**: Closed — resolved in 2546f75
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Refactoring
**Related**: Issue 193; Issue 195; Issue 196; Issue 209; Issue 270; `cmd/harnez/agyhooks.go`; `~/.gemini/config/hooks.json`

---

## 1. Problem & Motivation

Antigravity CLI (`agy`) supports two primary ways to intercept shelled-out tool commands (`run_command`):

1. **`PreToolUse` Lifecycle Hook (`~/.gemini/config/hooks.json`)**:
   - The hook intercepts `run_command` and returns an `overwrite.CommandLine` envelope.
   - **UI Artifact**: AGY's chat UI displays the exact overwritten command string (e.g. `Bash(⚙ ...)`, `Bash(harnez exec ...)`, or `Bash(⚙ bash -c '...')`). This leaks internal harness plumbing and wrapper flags into the user-visible chat trace.
2. **Guarded `bash` PATH-Shim (`~/.harnez/shims/bash`)**:
   - AGY resolves `bash` via `$PATH` when executing `bash -c "<CommandLine>"`.
   - A thin `bash` shim placed in an opt-in shim directory (`alias agy="PATH=$HOME/.harnez/shims:$PATH agy"`) intercepts the top-level execution without modifying AGY's `hooks.json`.
   - **Clean UI**: AGY renders the model's pure, native command string (e.g. `Bash(git status)`, `Bash(npm test)`).
   - **Recursion Guard**: Uses `HARNEZ_INTERCEPTED=1` to ensure nested subshells (e.g. `bash script.sh` inside the command) execute directly via `/bin/bash` without double-wrapping.

This ticket tracks deprecating and decommissioning the `agy-hooks` `PreToolUse` hook mechanism in favor of the guarded `bash` PATH-shim architecture.

## 2. Scope

**In scope:**
- Provision and manage the guarded `bash` shim directory (`~/.harnez/shims/bash`) via `harnez apply` / `init`.
- Ensure the `bash` shim correctly forwards arguments, protects against recursion via `HARNEZ_INTERCEPTED=1`, and delegates to `harnez exec -- /bin/bash "$@"`.
- Remove `harnez agy-hooks` from `~/.gemini/config/hooks.json` during `harnez apply` (or clean up agy hook registration).
- Deprecate/remove `cmd/harnez/agyhooks.go` and associated plumbing once the shim workflow is fully validated.
- Update documentation (`docs/HookRewritePattern.md`, `docs/practices/AgenticLoop.md`) to reflect the quiet shim architecture for AGY.

**Out of scope:**
- Modifying Codex CLI or Claude Code hook architectures (which do not exhibit AGY's UI overwrite leak).

## 3. Acceptance Criteria

- [x] `~/.harnez/shims/bash` is created and managed by `harnez apply`.
- [x] AGY launched with `PATH=~/.harnez/shims:$PATH` routes all `run_command` calls through `harnez exec` while displaying clean native commands in the UI.
- [x] Inner `bash` calls inside agent commands do not recurse or record duplicate telemetry rows.
- [x] `harnez apply` ensures `~/.gemini/config/hooks.json` does not contain stale `agy-hooks` entries.
- [x] Automated tests verify the `bash` shim, recursion guard, and telemetry capture.

