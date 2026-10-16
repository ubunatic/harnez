# 759 — Bench haiku:low, haiku:med, luna:med and sol:low on the same tasks

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: #755 (Haiku 5.5 in the model list), #565 (bench model matrix), `docs/studies/2026-10-08-claude-haiku-5-5-first-insights.md`

---

> `/goal`: Run `harnez bench run` on the same tasks for `claude:haiku:low`, `claude:haiku:med`, `codex:luna:med` and `codex:sol:low`, record results, and update the Haiku 5.5 study with our own numbers, or stop and report when blocked on a user decision or a denied permission. Deferred: the user will start this later.

## 1. Problem & Motivation

Published Haiku 5.5 benchmarks are vendor or launch-week figures, mostly at max effort, and they are not like-for-like with Luna and Sol. We need harnez's own numbers on our tasks at the effort levels we actually use.

## 2. Technical Specification / Findings

- Models: `claude:haiku:low`, `claude:haiku:med` (Haiku 5.5, `claude-haiku-5-5`), `codex:luna:med` (GPT-6 Luna), `codex:sol:low` (GPT-6.1 Sol).
- Runner: `harnez bench run --model <list>`; the matrix support is from #565.
- Cost signal: the spec cost values for these models are relative scores. Real cost per task should come from the run output, not the spec.
- Open: which tasks to use, and whether the read mode matrix (text, card) is needed. Record the choice in the ticket when the run is planned.

## 3. Implementation & Verification Plan

1. Pick the task set and read modes; record them here.
2. Run `harnez bench run` on the four models with the same tasks, using `HTO=0` for long runs.
3. Record pass rate, token use and cost per task in this ticket.
4. Update `docs/studies/2026-10-08-claude-haiku-5-5-first-insights.md` with the results, noting the difference from launch-week figures.
5. Close this ticket when the study is updated.
