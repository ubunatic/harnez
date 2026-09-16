# 369 — Detect Go library CLI and TUI shape for init guidance

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `docs/studies/2026-09-16-ramp-levels-and-go-repository-init.md`, [[368-report-ramp-evidence-and-projected-changes-from-harnez-init]]

---

## 1. Problem & Motivation

Current `init` treats `go.mod` as enough to select Go docs. It cannot distinguish a public Go library from a runnable CLI, or identify TUI-specific guidance. A single guessed layout would produce misleading instructions in mixed repositories.

## 2. Scope & Design

- Add a read-only Go shape inventory: module/workspace boundaries, library packages and public API surfaces, executable `main` packages, existing build/test entry points, and likely TUI usage. Cobra and terminal libraries are positive signals, not requirements.
- Model `library`, `CLI`, and `mixed` with TUI as an additional capability. Do not infer that every `cmd/` directory is the release target. Expose evidence and ambiguity; allow an explicit project-local override when detection cannot choose.
- Use the inventory to generate a previewable, managed L2 guidance section. Libraries should name real package entry points, API compatibility, examples and test commands. CLIs should cover actual invocations, flags, stdout/stderr, exit behavior, config and cancellation. TUI guidance should cover terminal cleanup, resize, Unicode width and deterministic model tests when relevant.
- Preserve user-authored instructions and current `init` behavior for non-Go projects. Keep additions concise and link to existing docs rather than copying long generic rules.

## 3. Exit Criteria

- [ ] Fixtures cover pure library, Cobra CLI, non-Cobra CLI, CLI+TUI, mixed module/workspace and ambiguous layouts.
- [ ] The preview shows the signals used and does not invent commands, packages or capabilities.
- [ ] First reconciliation writes only project-local managed content; second run is clean.
- [ ] Existing custom AGENTS.md content survives regeneration and profile overrides are documented.

## 4. Verification

Use fixtures and at least one real Go library and CLI/TUI project to review generated instructions for truthfulness. Run the project test target once after source edits under Quota-1, and confirm a second init changes nothing.
