# 076 — `AGENTS.local.md` Overlay & Ephemeral Runtime Mode Switching

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics & Architecture
**Related**: [075-concisemode-promote-doc-to-real-skill.md](075-concisemode-promote-doc-to-real-skill.md), `docs/practices/ConciseMode.md`, `internal/mode`

---

## 1. Problem & Motivation

`harnez mode <tier>` dynamically switches ConciseMode terseness by:
1. Emitting a high-priority steering directive to stdout (for the active session).
2. Updating `<!-- harnez:begin Concise Mode -->` in `./AGENTS.md` (for subagents and subsequent sessions).

While effective, modifying `./AGENTS.md` leaves a dirty working tree (`git status` shows untracked/modified changes), which is unwelcome in clean repositories, during ongoing commits, or in multi-developer environments.

---

## 2. Proposed Architecture: `AGENTS.local.md` Overlay

### A. Static Doc Reference in `AGENTS.md`
`harnez init` and standard `AGENTS.md` templates include a static reference:
```markdown
<!-- harnez:begin Local Overlays -->
- Local ephemeral overrides: @AGENTS.local.md
<!-- harnez:end Local Overlays -->
```

### B. Git Exclusion
`AGENTS.local.md` is added to `.git/info/exclude` (or project `.gitignore` / global ignore) so local mode changes never dirty `git status`.

### C. `harnez mode` Target Selection
- **Default**: Writes to `./AGENTS.local.md` (or the file specified via `--file`). If mode is `off`, removes `./AGENTS.local.md` or its managed section.
- **Flags**:
  - `--ephemeral` / `-e` / `--no-file`: Stdout directive only; does not write any file to disk.
  - `--persist` / `--main`: Writes directly to `./AGENTS.md` instead of `AGENTS.local.md`.

---

## 3. Acceptance Criteria

- [ ] Update `internal/mode` to target `./AGENTS.local.md` by default while reading/updating managed sections.
- [ ] Add `--ephemeral` (`-e` / `--no-file`) and `--persist` (`--main`) flags to `harnez mode`.
- [ ] Ensure `harnez init` / `harnez apply` includes the `@AGENTS.local.md` pointer convention and auto-ignores `AGENTS.local.md` in `.git/info/exclude`.
- [ ] Update unit and CLI tests in `internal/mode/` and `cmd/harnez/`.
- [ ] Verify `git status` stays clean when toggling `harnez mode <lite|std|ultra|off>`.
- [ ] Pass `go test ./...`, `harnez apply`, and `harnez status`.
