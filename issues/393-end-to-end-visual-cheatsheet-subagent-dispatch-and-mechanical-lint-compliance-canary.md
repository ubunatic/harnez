# 393 — End-to-end visual cheatsheet subagent dispatch and mechanical lint compliance canary

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Research & Canary / Context Optimization
**Related**: [[387-benchmark-token-efficiency-of-doc-screenshots-and-images-vs-raw-markdown-text-across-agent-harnesses]], [[392-multi-doc-bounded-developer-cheatsheet-card-3-in-1-packed-below-1568px-ceiling]]

---

## 1. Problem & Motivation

While Issue 387 verified that multimodal models can read text from screenshots with 100% OCR accuracy, the true test of visual documentation injection is **instruction following in a live coding task**.

This canary evaluates whether a freshly dispatched subagent, given *only* an attached visual cheatsheet image (e.g. `Bash_2col.png`) and zero text documentation, writes code that mechanically complies with every rule in that visual guide. Compliance is evaluated objectively using `harnez lint` / `internal/lint` (e.g. `if test` conditionals, forbidden `[ ]` brackets, no semicolons before `then`, `local` split declarations).

---

## 2. Acceptance Criteria

- [ ] Automated dispatch script spawning an isolated subagent with only visual doc context.
- [ ] Task execution against real shell script generation fixture.
- [ ] Mechanical validation of generated code via `harnez lint`.
- [ ] Token usage logging comparing visual vs raw text subagent dispatches.
