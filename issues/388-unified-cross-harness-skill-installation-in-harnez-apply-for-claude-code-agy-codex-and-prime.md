# 388 — Unified cross-harness skill installation in harnez apply for Claude Code, AGY, Codex, and Prime

**Status**: Closed — verified unified cross-harness skill deployment across all 4 targets
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [[130-instruction-distribution-audit-followups]], [[386-strip-eager-global-doc-includes-from-global-claude-md-template]]

---

## 1. Problem & Motivation

Modern agent harnesses (`claude` v2+, `agy` / Gemini CLI, `codex`, `prime`) have converged on the standard **Agent Skills** specification (`<skill-name>/SKILL.md`). In this specification:
- Each skill lives in its own directory: `<skills-root>/<name>/SKILL.md` with optional `references/` or `scripts/`.
- Progressive disclosure allows harnesses to index skill metadata (`name`, `description`) without inlining full instructions up front.
- Claude Code v2+ now natively discovers global skills in `~/.claude/skills/<name>/SKILL.md`, automatically making them available both as slash commands (`/<name>`) and via semantic auto-invocation.

Previously, `harnez apply` installed skills to `~/.gemini/skills`, `~/.codex/skills`, and `~/.prime/agent/skills`, while treating Claude Code exclusively via legacy flat commands (`~/.claude/commands/*.md`).

To achieve unified skill distribution across all harnesses on `harnez apply`:
1. `harnez apply` should install skills to `~/.claude/skills/` alongside `~/.gemini/skills/`, `~/.codex/skills/`, and `~/.prime/agent/skills/`.
2. Skill generation should support standard frontmatter flags where appropriate (`disable-model-invocation`, `user-invocable`, `argument-hint`).
3. Clean idempotent apply, diff, and uninstall/removal behaviors must cover all configured skill targets.

---

## 2. Scope & Acceptance Criteria

- [x] Add `claude_skills_target` (defaulting to `~/.claude/skills`) to `config.yaml` / `internal/claude` skill targets.
- [x] Ensure `skillTargets()` in `internal/claude/apply.go` and `internal/claude/diff.go` includes Claude skills target when active.
- [x] Verify `harnez apply` cleanly writes skills to `~/.claude/skills/`, `~/.gemini/skills/`, `~/.codex/skills/`, and `~/.prime/agent/skills/`.
- [x] Verify `harnez diff` and `harnez clean` correctly track `~/.claude/skills/`.
- [x] Add unit tests in `internal/claude/` verifying all agent skill targets are returned and properly populated.
- [x] Pass `make test-q1`.

