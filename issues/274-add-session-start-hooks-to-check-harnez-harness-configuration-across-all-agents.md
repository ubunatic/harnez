# 274 — Add session start hooks to check harnez harness configuration across all agents

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture / Agentic Ergonomics
**Related**: `internal/claude/status.go`; `internal/codex/`; `internal/agy/`; `config.yaml`; Issue 271; Issue 272; Issue 273

---

## 1. Problem & Motivation

Agents (Claude Code, Antigravity/AGY, and Codex CLI) can be started in diverse terminal environments or subshells where harnez hooks, settings, or PATH shims might be unapplied, drifted, misconfigured, or shadowed. When this occurs:
- Tool execution telemetry (`toolcalls` table) is silently skipped.
- Output distillation (`harnez distill`) does not run, leading to potential token waste or missing summary context.
- Agents operate without immediate awareness that the telemetry harness is inactive.

Adding a lightweight, proactive session-start hook (e.g. `SessionStart` / startup hook lifecycle event) across all supported agent ecosystems allows harnez to verify harness readiness immediately upon session launch. If the harness is unconfigured or drifted, it can emit a lean, actionable warning to the user or agent, preventing silent telemetry loss. Furthermore, scaffolding a modular startup check architecture makes it easy to add future environment health checks (e.g. DB connectivity, spec validation, git worktree status).

## 2. Scope

**In scope:**
- Session startup check hooks across supported agents:
  - **Claude Code**: `SessionStart` hook / statusline verification.
  - **Antigravity (AGY)**: Session startup check or banner verification (checking that either the guarded `bash` shim or native `PreToolUse` hook is active).
  - **Codex CLI**: `SessionStart` / startup notification via configured `hooks.harnez`.
- Health check verification logic:
  - Check agent configuration integrity and drift against `harnez status` standards.
  - Verify executable resolution (`harnez`, shims, hooks).
- Extensible check runner architecture: Scaffold a unified health-check interface in `internal/` so additional environment or runtime diagnostics can easily be registered and executed.
- Non-blocking execution: Startup checks must be extremely fast (<20ms) and never block or abort agent startup if a check fails or times out.

**Out of scope:**
- Invasive automatic modifications or forced reconfiguration during session startup without user consent.
- Heavyweight network or long-running checks.

## 3. Acceptance Criteria

- [ ] Startup / session-start hooks or checks are provisioned across Claude Code, AGY, and Codex CLI on `harnez apply`.
- [ ] Startup checks verify that the agent's telemetry harness (hooks or guarded shims) is active and properly wired.
- [ ] If configuration drift or missing shims are detected, a concise, actionable warning is presented at session start.
- [ ] The startup check framework is modularly structured to allow easy addition of future configuration/health checks.
- [ ] Startup checks execute with negligible latency (<20ms) and fail open (never crash or freeze agent startup).
- [ ] Automated tests verify session-start hook generation, check runner logic, and warning formatting.
