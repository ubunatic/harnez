# 273 — Restore old AGY PreToolUse hook as opt-in configuration option

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: Issue 193; Issue 195; Issue 209; Issue 271; `cmd/harnez/agyhooks.go`; `internal/agy/`; `~/.gemini/config/hooks.json`

---

## 1. Problem & Motivation

Issue 271 decommissioned `agy-hooks` (`PreToolUse` hook overwrite in `~/.gemini/config/hooks.json`) in favor of guarded `bash` PATH shims (`~/.harnez/shims/bash` via Issue 272 `~/.harnez/env.sh`) to eliminate UI wrapping artifacts in AGY chat sessions.

However, some environments or user setups cannot easily modify `$PATH` or use shell function wrappers/aliases (e.g. non-interactive invocations, IDE integrations, custom container environments, or strict PATH policies). For these workflows, native `PreToolUse` hook interception in Antigravity (`agy`) remains a valuable, zero-shell-setup alternative.

The previous `agy-hooks` implementation suffered from complexity due to heuristic command branching and prefix handling. A restored hook mechanism should be kept strictly simple: perform a uniform `harnez exec -- bash -c "<CommandLine>"` (or direct execution) rewrite without brittle string parsing or heuristic branches. It should be provided as an opt-in configuration setting in `config.yaml` or a CLI flag rather than enabled unconditionally.

## 2. Scope

**In scope:**
- Add an opt-in configuration option (e.g., `agy.hook_mode: pretooluse` in `config.yaml` or an explicit flag during `harnez apply`) to manage `~/.gemini/config/hooks.json`.
- Implement a simplified `agy-hooks` PreToolUse hook handler (in `cmd/harnez/` and `internal/agy/`) that rewrites `run_command` invocations to `harnez exec -- bash -c "<CommandLine>"` cleanly without heuristic branching.
- Update `harnez apply`, `harnez diff`, `harnez status`, and `harnez clean` to respect the opt-in hook configuration and verify hook status.
- Add unit and integration tests verifying hook JSON generation, payload rewriting, and drift reporting.

**Out of scope:**
- Replacing or deprecating the default guarded `bash` PATH shim mechanism (which remains the default quiet path).
- Complex heuristic command rewriting or subshell inspection inside the hook handler.

## 3. Acceptance Criteria

- [ ] `config.yaml` (or `harnez apply` flags) supports opting into native AGY `PreToolUse` hook management.
- [ ] When enabled, `harnez apply` writes the simplified `PreToolUse` hook into `~/.gemini/config/hooks.json`.
- [ ] The hook cleanly rewrites `run_command` calls to `harnez exec -- bash -c "<CommandLine>"` without complex branching heuristics.
- [ ] When disabled (default), `harnez apply` does not install the hook (and cleans up stale entries as specified in Issue 271).
- [ ] `harnez status` accurately reports whether AGY is configured via PATH shim or native `PreToolUse` hook.
- [ ] Unit and smoke tests verify hook execution, configuration toggle, and clean removal.
