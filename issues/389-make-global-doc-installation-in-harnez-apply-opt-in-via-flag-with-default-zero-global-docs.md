# 389 — Make global doc installation in harnez apply opt-in via flag with default zero global docs

**Status**: Closed — made global docs opt-in with zero default global docs on apply
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture & Context Optimization
**Related**: [[386-strip-eager-global-doc-includes-from-global-claude-md-template]], [[388-unified-cross-harness-skill-installation-in-harnez-apply-for-claude-code-agy-codex-and-prime]]

---

## 1. Problem & Motivation

Previously, `config.yaml` listed 16 language and workflow docs under `docs:`, causing `harnez apply` to unconditionally install those docs into `~/.claude/docs/` and `~/.prime/agent/docs/`. This created cross-harness asymmetry:
- Claude Code and Prime had global markdown doc copies in their user home dirs.
- Codex and AGY did not have global doc copies.
- Stale global docs in `~/.claude/docs/` risked causing version drift and cross-project confusion.

Per the Harnez architectural contract:
> *"Minimal Global Docs: keep this global file free of anything not relevant to every project — it applies everywhere and is glue between local and global docs only, steering the agentic setup rather than carrying project-specific content."*

Project conventions belong inside project repositories (`harnez init`), and global capabilities belong in unified Skills (`~/.claude/skills/`, `~/.gemini/skills/`, `~/.codex/skills/`, `~/.prime/agent/skills/`).

---

## 2. Technical Specification

1. **Default Zero Global Docs in `config.yaml`**: Set `docs: []` by default in `config.yaml`.
2. **Opt-in Global Docs via Flag**:
   - `harnez apply` installs zero global docs by default (only hooks, permissions, skills, global instructions glue, and shims).
   - `harnez apply --docs <name>...` (or `-d <name>...` / `--docs all`) allows explicitly installing specified global docs when requested.
   - Support `--remove-docs` / `--no-docs` or cleaning unmanaged global docs when no docs are requested.
3. **Clean Sync & Diff**:
   - `harnez diff` and `harnez apply` reflect the updated doc set accurately.
   - `harnez status` correctly reports global docs when present or zero when omitted.
4. **Regression Testing**:
   - Update tests in `internal/claude` and `cmd/harnez` to verify zero default global docs on bare `apply` and opt-in installation with `--docs`.
   - Pass `make test-q1`.

---

## 3. Acceptance Criteria

- [x] `config.yaml` `docs:` defaults to empty (`[]`).
- [x] Bare `harnez apply` installs zero global docs into `~/.claude/docs/` or `~/.prime/agent/docs/`.
- [x] `harnez apply --docs <name>...` installs only the explicitly requested docs.
- [x] Stale unmanaged global docs in `~/.claude/docs/` can be cleaned up.
- [x] All unit tests in `internal/claude` pass under `make test-q1`.
- [x] Clean workspace canary passes verifying zero global doc baggage.
