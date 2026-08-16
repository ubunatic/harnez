# 019 — Rename project from claudeconfig to harnez

**Status:** Closed

## Context

The project originally focused solely on managing Claude Code configuration (`~/.claude`), but has grown into a multi-agent harness:
- Distributes skills/commands across Claude (`~/.claude/commands/`), Antigravity/Gemini (`~/.gemini/skills/`), and Codex (`~/.codex/skills/`).
- Manages project scaffolds (`AGENTS.md`, `CLAUDE.md` symlinks, Makefile sentinels and targets).
- Enforces shared engineering discipline (Canary-first development, Spec-driven architecture, language conventions).

The name `harnez` reflects this role as a lightweight, cross-agent developer harness.

## Scope of Work

1. **Go Module & Binary:**
   - Rename module in `go.mod` to `ubunatic.com/harnez`.
   - Rename entrypoint directory from `cmd/claudeconfig/` to `cmd/harnez/`.
   - Update `Makefile`: `BINARY ?= harnez`.
   - Update all Go package imports.

2. **Section Markers & Backward Compatibility:**
   - Update primary markers in `internal/markdown/` and `internal/claude/`:
     - Markdown: `<!-- harnez:begin <section> -->` / `<!-- harnez:end <section> -->`
     - Bundled doc tag: `<!-- harnez:bundled -->`
     - Makefile: `# harnez:begin <section>` / `# harnez:end <section>`
   - Maintain backward-compatible matching for legacy `claudeconfig:` markers so existing projects continue to parse cleanly.

3. **Development Scripts & Tests:**
   - Update `scripts/smoke-test.sh` to build and test `./harnez`.
   - Update all test assertions in `internal/`.

4. **Documentation & Scaffolding:**
   - Update `AGENTS.md`, `CLAUDE.md`, `README.md`, `CONTEXT.md`, and docs under `docs/`.
   - Update `website/index.html`.
