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

Integrate this pattern into Harnez bundled documentation (`docs/TUIDesign.md`, `docs/CLIDesign.md`, and core `docs/practices/AgenticLoop.md`):

### The .ansi Mockup Lifecycle
1. **Host-Crafted Visual Spec (High-Tier Model)**:
   - The host orchestrator (e.g. Opus, Flash 3.7) one-shots the visual mockup directly to `docs/data/<app>-design-<nnn>.ansi` (or `docs/data/<app>-<subpane>-design-<nnn>.ansi` for partial widgets/sub-panes).
   - Target standard terminal geometries (e.g. 80x24, 100x30, or bounded sub-pane dimensions like 40x12, 24x4).
   - Use standard ANSI 16/256/RGB escape sequences matching project themes, with proper unicode column width and border alignment.
2. **Interactive Human Review**:
   - Provide a simple `cat docs/data/<file>.ansi` command for immediate native terminal inspection by the human developer.
3. **Commit Design Artifact**:
   - Check the `.ansi` file into git under `docs/data/` as a persistent, durable design spec and visual regression reference.
4. **Concise Worker Dispatch (Low-Tier Worker)**:
   - The host passes a simple 2–3 line directive to the lower developer agent (e.g., `luna:med`, `terra:low`):
     *"Implement the TUI layout to match the visual spec at `docs/data/<file>.ansi`. Inspect with `cat docs/data/<file>.ansi`."*
   - Avoids lengthy prose, prompt bloat, and repeated review cycles.
5. **Core Rule Integration**:
   - Keep the rule core and token-efficient across `docs/TUIDesign.md`, `docs/CLIDesign.md`, and `docs/practices/AgenticLoop.md` without requiring deep discovery.

## 3. Sprint Milestones

- **M1 — TUIDesign & CLIDesign Updates**:
  - Add the `.ansi` mockup workflow section to `docs/TUIDesign.md` covering whole-app and sub-pane mockups, storage under `docs/data/`, `cat` inspection, and high-tier host / low-tier worker split.
  - Add cross-reference in `docs/CLIDesign.md`.
- **M2 — Core Rule in AgenticLoop & Source Sync**:
  - Add concise rule bullet in `docs/practices/AgenticLoop.md` (and `docs/practices/AgenticLoop.lite.md`).
  - Sync managed root docs (`harnez init -d .`).
  - Run verification (`make test` or `go test ./...`).
- **M3 — Verification & Sprint Teardown**:
  - Verify clean tree and close ticket #593.

## 4. Acceptance Criteria

- `docs/TUIDesign.md` includes the `.ansi` visual mockup workflow section (full-screen and sub-panes).
- `docs/CLIDesign.md` links to the TUI mockup practice.
- `docs/practices/AgenticLoop.md` has the concise core rule.
- `harnez init -d .` runs cleanly without unwanted drift.

