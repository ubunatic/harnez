# 397 — Support --doc-mode=vision and visual cheatsheet context attachment in subagent dispatch and harnez mode

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Multi-Agent Dispatch / Context Optimization
**Related**: [[393-end-to-end-visual-cheatsheet-subagent-dispatch-and-mechanical-lint-compliance-canary]], [[394-multimodal-doc-delivery-architecture-and-context-optimization-roadmap]], [[396-automated-visual-cheatsheet-builder-command-to-compile-markdown-docs-into-bounded-image-cards]]

---

## 1. Problem & Motivation

Issue 393 verified that an isolated subagent given **only** an attached visual cheatsheet image (`STYLE_GUIDE.png`) and zero text rules achieves 100% mechanical compliance with repository lint invariants while saving up to 8.5x tokens.

To make visual doc injection an operational capability across Harnez multi-agent workflows, the subagent dispatch mechanism (`harnez subagent`, `harnez agent`, `/harnez-agent`) and repository mode system (`harnez mode`) must natively support visual doc context attachment.

---

## 2. Proposed Architecture & Flags

### Subagent Dispatch:
```bash
# Dispatch a subagent with visual cheatsheet image context attached:
harnez subagent --doc-mode=vision --card=dev-3in1 --task="Implement deploy-check.sh"

# Dispatch with specific visual language card:
harnez agent run --doc-mode=vision --card=Bash_2col -- "Create backup rotation script"
```

### Workspace Doc Modes (`harnez mode`):
- `harnez mode full`: Standard full text markdown docs linked in `./docs/`.
- `harnez mode lite`: Compact text docs (`*.lite.md`) linked into project workspace.
- `harnez mode vision`: Visual cards rendered and symlinked under `./docs/vision/` for multimodal harnesses (Codex, AGY, Claude).

### Automated Context Injection:
- When `--doc-mode=vision` is selected, the subagent launcher stages the rendered card in the subagent's scratch directory as `STYLE_GUIDE.png` and constructs the multimodal prompt header, bypassing raw text injection.

---

## 3. Acceptance Criteria

- [ ] Support `--doc-mode=vision|lite|full` in subagent dispatch CLI.
- [ ] Integration with `harnez mode` command allowing repositories to switch between text, lite, and visual doc modes.
- [ ] Automated staging of visual cheatsheet assets during subagent workspace setup.
- [ ] End-to-end integration test verifying visual subagent dispatch and token accounting.
