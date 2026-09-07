# 275 — Promote source over dot in Bash.md and emphasize 3-line if-then-fi block alignment rules

**Status**: Closed — resolved in d7bef73
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Documentation / Agentic Ergonomics
**Related**: `docs/lang/Bash.md`; `AGENTS.md`; `internal/claude/`

---

## 1. Problem & Motivation

AI coding agents frequently fall back to standard POSIX conventions (such as `if [ ... ]; then\n  cmd\nfi` or putting `then` on a standalone line followed by indented commands) unless project-specific conventions are prominently highlighted with concrete visual examples and explicit anti-patterns.

In particular:
1. **Sourcing syntax**: The single dot (`.`) syntax for sourcing shell scripts and dotfiles is less visually distinct, less searchable, and less self-documenting than the explicit `source` keyword.
2. **Conditional layout**: The repository's preferred 3-line `if-then-fi` and 4-line `if-then-else-fi` vertical alignment (`then <1st cmd>` on the same line, no semicolons, clean continuation) is often missed by agents that default to standard POSIX forms.

Updating `docs/lang/Bash.md` (and reinforcing the summary in `AGENTS.md`) to explicitly promote `source` over `.` and clarifying the 3-line conditional formatting rules with side-by-side good vs. bad examples will eliminate ambiguity and improve formatting consistency across scripts, shims, and agent-generated shell code.

## 2. Scope

**In scope:**
- Update `docs/lang/Bash.md`:
  - Add explicit guidance and examples promoting `source` over `.` for sourcing external scripts, environment files, and shell snippets.
  - Strengthen Section 2 (Conditionals) and Section 5 (Line Breaks, Continuation & Indentation) with prominent 3-line `if-then-fi` and 4-line `if-then-else-fi` examples where the first command immediately follows `then` / `else` on the same line without semicolons.
  - Add explicit anti-pattern callouts (e.g. forbidden `[ ... ]` and `[[ ... ]]`, forbidden `; then`, standalone `then` without immediate command).
- Review `AGENTS.md` language conventions summary to ensure concise alignment with the updated Bash doc.
- Check existing shims and scripts in the repository to ensure adherence to these conventions.

**Out of scope:**
- Modifying third-party external scripts or shell tools outside of harnez-managed project files.

## 3. Acceptance Criteria

- [ ] `docs/lang/Bash.md` explicitly documents `source` as preferred over `.` with clear rationale and examples.
- [ ] `docs/lang/Bash.md` prominently illustrates the 3-line `if-then-fi` and 4-line `if-then-else-fi` format (`then <cmd>` on the same line, no semicolons) alongside explicit anti-pattern callouts (`[ ... ]`, `[[ ... ]]`, `; then`).
- [ ] Summary bullet in `AGENTS.md` remains concise and accurately reflects the emphasized rules.
- [ ] Tests and doc linters pass (`go test ./...`, `harnez status`).
