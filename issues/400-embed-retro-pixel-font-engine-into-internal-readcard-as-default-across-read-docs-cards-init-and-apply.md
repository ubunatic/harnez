# 400 — Embed retro pixel font engine into internal/readcard as default across read, docs cards, init, and apply

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: CLI & Tooling / Context Optimization
**Related**: [[395-harnez-read-command-with-i-image-flag-for-rendering-and-injecting-whole-files-as-visual-context]], [[396-automated-visual-cheatsheet-builder-command-to-compile-markdown-docs-into-bounded-image-cards]], [[397-support-doc-mode-vision-and-visual-cheatsheet-context-attachment-in-subagent-dispatch-and-harnez-mode]], [[398-trial-ultra-compact-pixel-and-bitmap-fonts-from-retro-games-for-extreme-vit-token-compression]]

---

## 1. Problem & Motivation

Our canary research (Issue 398 and study `docs/studies/2026-09-17-retro-pixel-fonts-and-micro-vit-compression-study.md`) proved that pure 1-bit binary retro pixel fonts ($5\times8$ Spleen/retro and $3\times5$ micro) eliminate anti-aliasing gray-bleed and achieve 100% OCR legibility on micro canvases, unlocking up to **10.5x token compression**.

To bring these efficiency gains to daily agent workflows, we need to:
1. Embed the retro pixel fonts (`Font5x8`, `Font3x5`, `Font4x6`, `Font6x12`) natively into `internal/readcard`.
2. Support `--font=pixel|retro|5x8|3x5|standard|8x16|7x13` across `harnez read`, `harnez docs cards build`, and `harnez init`.
3. Set `--font=pixel` ($5\times8$ / integer-scaled) as the default font across `read` and `docs cards`.
4. Update `harnez apply` to distribute skills and settings that leverage pixel font visual context.

---

## 2. Acceptance Criteria

- [ ] Embed $5\times8$ and $3\times5$ pixel bitmap tables in `internal/readcard/font.go` / `internal/readcard/font_glyphs.go`.
- [ ] Add `ParseFontName` in `internal/readcard` and make `pixel` ($5\times8$) the default for visual cards.
- [ ] Support `--font` flag in `harnez read` and `harnez docs cards build`.
- [ ] Update `harnez init` and `harnez apply` to support pixel font cards and skill definitions.
- [ ] Unit & CLI integration tests in `internal/readcard/` and `cmd/harnez/`.
- [ ] Rebuild binary (`make install`), run `make test-q1`, and execute `harnez apply`.
