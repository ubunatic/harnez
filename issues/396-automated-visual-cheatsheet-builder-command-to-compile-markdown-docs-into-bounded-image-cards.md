# 396 — Automated visual cheatsheet builder command to compile markdown docs into bounded image cards

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Tooling & Build / Context Optimization
**Related**: [[387-benchmark-token-efficiency-of-doc-screenshots-and-images-vs-raw-markdown-text-across-agent-harnesses]], [[392-multi-doc-bounded-developer-cheatsheet-card-3-in-1-packed-below-1568px-ceiling]], [[394-multimodal-doc-delivery-architecture-and-context-optimization-roadmap]]

---

## 1. Problem & Motivation

The prototype image renderers in `scripts/canary-doc-vision/` (`render.py`, `bundle_render.py`) proved that 2-column cheatsheets and 3-in-1 bundled developer cards achieve 2.4x–8.5x token compression while maintaining 100% OCR accuracy.

However, rendering currently requires invoking ad-hoc Python scripts. Harnez needs a first-class CLI command (e.g. `harnez build-cards` or `harnez docs cards`) to compile repository documentation (`docs/lang/*.md`, `docs/practices/*.md`) into bounded, production-ready PNG cards as part of build and distribution pipelines.

---

## 2. Proposed CLI Interface & Design

```bash
# Compile all standard doc cheatsheets into docs/vision/ or ~/.harnez/cards/:
harnez docs cards build

# Compile specific bundled cards (e.g. developer 3-in-1 card: Bash + Make + Git):
harnez docs cards build --bundle=dev-3in1 --out=docs/vision/dev_3in1.png

# Validate existing cards against the 9.5px ViT resolution threshold and 1568px bounds:
harnez docs cards check docs/vision/*.png
```

### Key Technical Requirements:
1. **Font Resolution Invariant**: Enforce a strict minimum monospace font size of $9.5\text{px}$ (optimal: $10.5\text{px}$) to guarantee 100% OCR accuracy across all ViT architectures.
2. **Dimension Guardrail**: Enforce bounded canvas limits ($\le 1568\text{px}$ on max dimension) to avoid server-side downsampling cliffs in Claude and OpenAI.
3. **Multi-Doc Bundling Engine**: Support declarative configuration in `config.yaml` defining card bundles (e.g., `dev_cheatsheet_3in1` = `Bash.md` + `Make.md` + `Git.md`).
4. **Integration with `harnez apply` / `harnez init`**: Allow pre-rendered card distribution alongside text docs.

---

## 3. Acceptance Criteria

- [ ] CLI command `harnez docs cards [build|check]` implemented and integrated into Cobra command tree.
- [ ] Declarative bundle configuration support in `config.yaml` or spec files.
- [ ] Automatic layout engine wrapping markdown content into 2-column or 3-column micro-grids.
- [ ] CI validation check verifying that all generated card assets satisfy $\le 1568\text{px}$ bounds and $\ge 9.5\text{px}$ font ladder rules.
- [ ] Automated unit and CLI integration tests in `cmd/harnez/`.
