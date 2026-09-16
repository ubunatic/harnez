# 382 — Make user shell shortcuts available to Codex, Claude, AGY, and other agents

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: Issue 197 (human-invoked shell enhancements); Issue 270 (`⚙` agent command shim); `docs/CLIDesign.md` (`apply` owns global harness setup)

---

## 1. Problem & Motivation

The user's shell shortcuts are unavailable when agent tools run commands. In this session, `gss` and `pull` both returned `zsh:1: command not found` in user shell command invocations. The user wants `harnez apply` to make their shell aliases available to Codex, Claude, AGY, and other supported agents, so familiar commands work in those environments.

Shell aliases depend on shell startup and expansion rules; agent command runners may use non-interactive shells or different shells. Simply adding aliases to an interactive shell rc file may therefore leave the reported failure intact.

## 2. Scope

- Identify the user's intended source of truth for personal shortcuts and how `apply` should opt into or discover it. Preserve existing user definitions; do not silently invent meanings for `gss` or `pull`.
- Make supported shortcuts resolvable in command environments configured by `harnez apply` for Codex, Claude, AGY, and other harnesses that `apply` supports. Account for non-interactive shell behavior and aliases versus functions or executable shims.
- Keep installation, updates, and cleanup idempotent. Avoid overwriting user shell configuration or changing unrelated commands.
- Document which agent runners and shell invocation modes are supported, plus any limitations for shell-specific aliases.
- Keep global agent setup in `apply`; do not add project-local behavior to `init`.

## 3. Acceptance Criteria

1. A documented configuration path lets the user expose existing personal shortcuts to supported agent command runners through `harnez apply`.
2. After applying, representative shortcuts including `gss` and `pull` resolve in the actual command execution modes of Codex, Claude, and AGY, where those runners permit user configuration. Unsupported runners or modes are reported clearly.
3. Repeated `apply` runs produce no drift, and cleanup removes only harnez-managed wiring.
4. Verification covers at least one non-interactive invocation and confirms user-owned shell configuration remains intact.

## 4. Verification Guidance

Probe the real command runner for each agent before choosing shell sourcing or a PATH-based mechanism. Record the source definitions used for `gss` and `pull`, then exercise the aliases in each supported agent environment. Run the repository test target and `make install` for any Go implementation.
