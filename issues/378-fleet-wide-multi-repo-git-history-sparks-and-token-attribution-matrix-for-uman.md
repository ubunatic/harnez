# 378 — Fleet-wide multi-repo git history sparks and token attribution matrix for uman

**Status**: Open  
**Priority**: P2 (Medium)  
**Severity**: Minor  
**Category**: Feature  

**Related**: [376](376-multi-track-git-history-evolution-sparks-across-code-tests-docs-skills-and-issues.md), [377](377-project-level-token-attribution-and-generative-cost-of-change-metrics.md), `docs/studies/2026-09-16-fleet-usage-and-token-history-analysis.md`  

---

## 1. Summary & Motivation

While `harnez` provides deep single-repository artifact evolution (`#376`) and project token attribution (`#377`), `uman` manages the developer's entire workspace across 20+ repositories simultaneously (`~/projects/*`).

Currently, `uman status` displays ahead/behind commits, dirty working trees, commit age, and website staleness.

This ticket tracks building a unified fleet-wide multi-track sparkline and token attribution matrix view for `uman` (e.g. `uman status --sparks` or `uman stats`), combining:
1. Multi-track artifact evolution (Code, Docs, Issues, Test ratio).
2. Attributed AI token spend across projects.

---

## 2. Proposed UX (`uman status --history` / `uman stats`)

```text
PROJECT       CODE SPARK     DOC SPARK      ISSUE SPARK    TEST RATIO   TOKENS ATTRIBUTED
harnez        [ ▂▃▄▅▆▇█]     [ ▂▃▅▆▇██]     [  ▂▄▆▇█▅]     0.66 [█]     1.42B tokens
lmcoder       [  ▂▃▄▅██]     [  ▂▃▃▄██]     [   ▂▃██ ]     0.82 [█]     680M tokens
cati          [    ▂▃██]     [    ▂▃██]     [     ██ ]     0.45 [▆]     120M tokens
psync         [ ▂▃▄▄▅██]     [ ▂▃▃▄▅██]     -              0.71 [▇]      45M tokens
fdapps        [   ▂▃▄██]     [    ▂▃██]     [    ▂██ ]     0.52 [▆]      30M tokens
smarthome     [    ▂▅██]     [     ▂██]     -              0.30 [▄]      15M tokens
```

---

## 3. Architecture & Integration Plan

1. **Export Engine**:
   - `internal/assess/` and `internal/usage/` in `harnez` expose clean Go APIs and CLI `--json` outputs for fast multi-track spark calculation and token attribution.
2. **`uman` Integration**:
   - `uman` calls into the shared assessment and usage libraries to render the multi-project sparkline table.
   - Embed into `uman tui` as an interactive historical view.

---

## 4. Acceptance Criteria

- [ ] `harnez` exports reusable Go packages / CLI JSON endpoints for multi-track sparklines and project token attribution.
- [ ] `uman status --history` (or `uman stats`) scans managed workspace projects and renders the multi-repo spark and token matrix.
- [ ] Column sorting supported (by token spend, by code volume, by ticket activity).
- [ ] Interactive support in `uman tui`.
- [ ] End-to-end integration verified across real workspace repositories in `~/projects/`.
