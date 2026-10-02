# 689 — Replace default usage boxes with a plain table

**Status**: Done
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Refactor
**Related**: [687](687-add-an-interactive-usage-dashboard-mode.md), [usage table](../internal/usage/usagetable.go)

---

## 1. Problem & Motivation
The default `harnez usage [--watch]` view drew one large box per agent plus a History box. It was
hard to scan, and the user never used the history totals.

## 2. Decisions (2026-10-02, user)
- Keep `--compact` and `--minimal` unchanged.
- Without them, show a plain table: one row per quota window with Agent, Quota, Used, Resets,
  Tokens, Tok/min, Updated, Model and Account. Usage only, no load rows.
- Remove the Claude, AGY, Codex and History boxes, the title-bar history totals and `--dashboard`.
  Hotkeys 2-5 are now unused; 6/7/8 keep their numbers.
- All modes read the usage store, because direct collection alone missed Codex and showed stale
  Claude data.
