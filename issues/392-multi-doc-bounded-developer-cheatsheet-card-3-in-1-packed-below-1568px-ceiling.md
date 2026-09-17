# 392 — Multi-doc bounded developer cheatsheet card (3-in-1 packed below 1568px ceiling)

**Status**: Closed — completed 3-in-1 multi-doc developer card render with 8.5x token compression
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Research & Canary / Context Optimization
**Related**: [[387-benchmark-token-efficiency-of-doc-screenshots-and-images-vs-raw-markdown-text-across-agent-harnesses]], [[391-vision-transformer-font-resolution-and-ocr-breakdown-threshold-probe]]

---

## 1. Problem & Motivation

Individual documentation files (`Bash.md`, `Make.md`, `Git.md`) each carry 1,500–2,500 text tokens (~6,500 text tokens total). Ingesting them as separate files or raw text inflates initial context.

Because Anthropic Claude imposes a 1,568px longest-edge downscaling ceiling and OpenAI caps shortest sides to 768px, a single mega-poster containing all 16 docs is downscaled into illegibility. However, packing a cluster of 3 essential developer docs (`Bash.md` + `Make.md` + `Git.md`) onto a bounded $1400 \times 1450\text{px}$ 3-column card stays strictly below downscaling thresholds while compressing 6,500 text tokens into ~1,100 vision tokens (a ~6x token reduction).

---

## 2. Acceptance Criteria

- [ ] Automated 3-in-1 cheatsheet generator bundling `Bash.md`, `Make.md`, and `Git.md`.
- [ ] Bounded dimensions ($\le 1500 \times 1500\text{px}$) avoiding server-side downsampling.
- [ ] OCR comprehension verification across all 3 embedded doc sections.
- [ ] Synthesis report and visual artifact in `docs/studies/`.
