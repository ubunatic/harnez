# 642 — harnez guard: wire decide hooks into config.yaml and add guard status/check commands

**Status**: Closed — resolved
**Priority**: P2 (Medium)
**Severity**: Normal
**Category**: Feature
**Related**: issues/634, config.yaml, cmd/harnez/guard.go, internal/guard/

---

## 1. Problem & Motivation
In issue 634, the core decision model rule enforcer and safety guardrail engine (`internal/guard`) and hook handlers (`harnez hook pre-edit`, `harnez hook pre-exec`) were implemented.
To make these hooks active in agent harnesses (Claude Code, Antigravity, Codex):
1. The `PreToolUse` matchers for `Edit` and `Write` must be installed by default via `config.yaml` on `harnez apply`. Since `harnez hook pre-edit` is fail-open and gated behind `guard.Enabled()`, it has zero overhead when inactive.
2. Developers and operators need a fast diagnostic command `harnez guard status` to check whether decision guardrails are active, which backend is configured, and whether keys are set.
3. Developers need `harnez guard check <file>` for dry-run validation against repository rules (`AGENTS.md`, `.harnez/rules/*.md`) without waiting for live tool calls.

## 2. Technical Specification / Findings
1. **Config Wiring (`config.yaml`)**:
   Add to `hooks:` in `config.yaml`:
   ```yaml
   - event: PreToolUse
     matcher: Edit
     command: "harnez hook pre-edit"
   - event: PreToolUse
     matcher: Write
     command: "harnez hook pre-edit"
   ```
2. **CLI Commands (`cmd/harnez/guard.go`)**:
   - `harnez guard status`: Reports active status (`HARNEZ_DECIDE_GUARD` / `~/.harnez/config.yaml`), default backend (`spec/decide.yaml`), configured threshold, and API key environment state.
   - `harnez guard check <file> [--content <text>] [--repo <dir>]`: Runs `guard.CheckFileEdit` against repository rules and prints the violation analysis.

## 3. Implementation & Verification Plan
1. Update `config.yaml` to include `PreToolUse` `Edit` and `Write` hooks.
2. Update `internal/claude` tests to account for the new default hooks.
3. Implement `newGuardCmd()` with `status` and `check` subcommands in `cmd/harnez/guard.go`.
4. Add unit and CLI tests in `cmd/harnez/guard_test.go`.
5. Verify with `go test ./...` and `make install`.
6. Dispatch review to `terra:med` via `harnez agent start`.
