# 593 — Adopt .ansi file design mockup workflow for TUI design and review

**Status**: Open
**Priority**: P2
**Severity**: Minor
**Category**: Practices / Documentation / TUI Design
**Related**: `docs/CLIDesign.md`, `docs/data/`, `codeberg.org/ubunatic/loom`

---

## Goal

`/goal`: Establish and document the `.ansi` file design mockup workflow as a standard recommended practice for TUI and terminal application design, human review, and agent dispatch across Harnez-managed projects.

## 1. Context & Problem

When creating or refactoring Terminal User Interfaces (TUIs), describing layout and styling changes purely through conversational text or jumping directly into implementation code often leads to misalignment, excess token consumption, and repeated review cycles.

During the Loom `examples/textedit` UI enhancement session (2026-09-25), we adopted a mockup-first workflow:
1. Discussed layout gaps and visual hierarchy conceptually.
2. Generated a concrete, pixel-aligned terminal artifact at `docs/data/textedit-design-001.ansi` with full ANSI SGR colors, status pills, line gutters, and borders.
3. The human reviewer inspected the exact rendering via `cat docs/data/textedit-design-001.ansi` in their native terminal.
4. Committed the `.ansi` file to version control as a persistent design spec.
5. Filed issue #117 pointing directly to the `.ansi` file as the technical spec for autonomous agent (`luna:med`) implementation.

This approach proved fast, clear, and eliminated ambiguity in visual expectations.

## 2. Proposed Practice & Guidance

Integrate this pattern into Harnez bundled documentation (`docs/CLIDesign.md` or a new `docs/TUIDesign.md`):

### The .ansi Mockup Lifecycle
1. **Design Proposal**: Outline conceptual changes (active focus styling, line gutters, badges, keycaps).
2. **Artifact Creation**: Generate a dedicated `docs/data/<app>-design-<nnn>.ansi` file:
   - Target standard terminal geometries (e.g. 80x24, 100x30).
   - Use standard ANSI 16/256/RGB escape sequences matching project themes.
   - Ensure proper unicode character width and border column alignment.
3. **Interactive Human Review**: Provide a simple `cat docs/data/<file>.ansi` command for the developer to inspect in their live terminal.
4. **Commit Design Artifact**: Check the `.ansi` file into git under `docs/data/` for durability and visual regression reference.
5. **Issue & Task Dispatch**: File an issue citing the `.ansi` file, allowing implementation agents (e.g., `luna:med`, `terra:med`) to work against an explicit, verifiable visual target.

## 3. Acceptance Criteria

- `docs/CLIDesign.md` (or `docs/TUIDesign.md`) includes a dedicated section on `.ansi` visual mockup workflows for TUI development.
- Conventions specify storing design mockups under `docs/data/<name>-design-<nnn>.ansi`.
- Harnez documentation reflects this as a recommended design-first practice before large TUI changes.
