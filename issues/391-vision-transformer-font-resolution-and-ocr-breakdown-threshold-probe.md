# 391 — Vision Transformer font resolution and OCR breakdown threshold probe

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Research & Canary / Context Optimization
**Related**: [[387-benchmark-token-efficiency-of-doc-screenshots-and-images-vs-raw-markdown-text-across-agent-harnesses]], [[392-multi-doc-bounded-developer-cheatsheet-card-3-in-1-packed-below-1568px-ceiling]]

---

## 1. Problem & Motivation

In Issue 387, we established that 10px–11px monospace typography on 2-column and 3-column cheatsheets achieves 100% OCR fidelity across Claude 3.7 and Gemini.

To determine the absolute minimum font size limit before character degradation begins (the "ViT Patch Threshold"), this canary generates a syntax stress card spanning 6px, 7px, 8px, 9px, 10px, 11px, and 12px font sizes with subtle code tokens (`!=`, `:=`, `==`, `&&`, `||`, `[ ]`, `{ }`, `$( )`, `;`, `:`, `!`, `~`) and probes multimodal models to identify the exact point of failure.

---

## 2. Acceptance Criteria

- [ ] Stress-test card generator script in `scripts/canary-doc-vision/` rendering font ladder (6px–12px).
- [ ] Empirical threshold table mapping accuracy percentage per font size on `claude` and `agy`.
- [ ] Documented minimum font size guidelines in `docs/studies/`.
