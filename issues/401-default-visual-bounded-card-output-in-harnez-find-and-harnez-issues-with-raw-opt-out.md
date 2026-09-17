# 401 — Default visual/bounded card output in harnez find and harnez issues with --raw opt-out

**Status**: Closed
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: CLI & Tooling / Context Optimization
**Related**: [[395-harnez-read-command-with-i-image-flag-for-rendering-and-injecting-whole-files-as-visual-context]], [[396-automated-visual-cheatsheet-builder-command-to-compile-markdown-docs-into-bounded-image-cards]], [[400-embed-retro-pixel-font-engine-into-internal-readcard-as-default-across-read-docs-cards-init-and-apply]]

---

## 1. Problem & Motivation

When agents execute issue tracker discovery (`harnez find issues ...`) or inspect specific tickets (`harnez issues show <id>`), dumping multi-hundred-line markdown files directly into the active prompt wastes thousands of context tokens and accelerates context window fatigue.

With the delivery of `harnez read` and the embedded retro pixel font engine (`internal/readcard`), Harnez can render dense, syntax-highlighted visual cards or bounded text summaries. Making bounded visual/summary cards the default in `harnez find issues` and `harnez issues` (while preserving `--raw` / `--text` / `--json` opt-outs for shell pipelines and text-only agents) will drastically cut agent context consumption during daily ticket discovery.

---

## 2. Proposed Architecture & Design

1. **Default Visual / Bounded Summary Output**:
   * For interactive terminal / agent sessions, `harnez find issues` and `harnez issues show` present bounded, high-density rendered cards or compact line-bounded summaries.
   * Multiple matching tickets are bundled into a multi-column overview card to avoid generating dozens of separate images.
2. **Opt-Out Mechanics**:
   * `--raw` / `--text`: Emit raw markdown or text directly to stdout.
   * `--json`: Emit structured JSON for script automation.
3. **Pipeline & Non-TTY Awareness**:
   * When stdout is redirected to a pipe or non-TTY environment, default to text/raw output automatically to avoid breaking shell pipelines.

---

## 3. Acceptance Criteria

- [x] Support `--raw`, `--text`, and `--json` flags on `harnez find issues` and `harnez issues`.
- [x] Connect `internal/readcard` rendering to `harnez find issues` and `harnez issues show`.
- [x] Implement multi-issue matrix/bundle rendering for search results.
- [x] Maintain pipe/non-TTY detection for script backwards compatibility.
- [x] Add unit and CLI tests in `cmd/harnez/find_test.go` and `cmd/harnez/issues_test.go`.
- [x] Update documentation and AGENTS.md conventions.
