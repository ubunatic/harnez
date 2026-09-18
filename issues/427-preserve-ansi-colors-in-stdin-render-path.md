# 427 — Preserve ANSI colors in stdin render path

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation

Piping ANSI-formatted output into the image renderer produces a garbage visual result. For example, `harnez usage --compact | harnez read -I` renders the compact usage output as a malformed card instead of preserving its terminal colors and readable layout.

The stdin render path needs to understand ANSI color/style escape sequences so that colored terminal output remains legible when converted into a visual context card. This is especially important for command output intended to be consumed through `harnez read -I`.

## 2. Technical Specification / Findings

- Preserve supported ANSI foreground/background colors and relevant text styles when stdin is rendered as an image.
- Strip or interpret escape sequences for layout purposes so they do not appear as visible garbage or corrupt width calculations.
- Keep plain, non-ANSI stdin behavior unchanged.
- Define behavior for unsupported or malformed escape sequences without allowing them to damage the rendered output.

## 3. Implementation & Verification Plan

1. **M1 — Reproduce and isolate**: Trace the stdin rendering path and identify where ANSI sequences are treated as text. Automated verification: a regression fixture covers representative `usage --compact` output.
2. **M2 — ANSI-aware rendering**: Add color/style parsing or normalization at the renderer boundary, preserving terminal appearance and display-width calculations. Automated verification: focused tests assert that ANSI escapes do not appear as visible text and that colored spans render with the expected styles.
3. **M3 — Regression coverage**: Cover plain text, common SGR colors/styles, resets, line wrapping, and malformed/unsupported sequences. Automated verification: stdin render tests pass without changing existing non-colored output.

Acceptance criteria:

- `harnez usage --compact | harnez read -I` produces a readable, correctly colored visual card.
- ANSI escape bytes are not rendered as garbage text.
- Display width and wrapping remain correct when colored text is present.
- Plain stdin and existing image-rendering behavior remain compatible.
