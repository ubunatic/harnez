# 1285 — init: --quota-1 prints Quota-1 Guardrails added for AGENTS.md before migrating it to Quota.md

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: #701, #1077

---

## Problem

Running `harnez init --quota-1 --variant lite` in an existing project (`splitty`, `harnez 0.5.11+5547`) prints `Quota-1 Guardrails: added` for `AGENTS.md`, even though the rules land in `.harnez/rules/Quota.md` and `AGENTS.md` ends with no diff.

`initQuotaRules` (`internal/claude/init_quota.go:21-28`) calls `applySectionMD(agentsPath, quota1SectionName, cfg.AgentsMD.Rules.QuotaSection)` and prints `wrote AGENTS.md ...`, before the later rules phase migrates and strips that block out of `AGENTS.md` into `.harnez/rules/Quota.md`.

## Goal

/goal Stop `harnez init --quota-1` from writing `Quota-1 Guardrails` into `AGENTS.md` and reporting it as added there when Quota-1 rules belong in `.harnez/rules/Quota.md`, while preserving migration of legacy `AGENTS.md` Quota-1 blocks and Makefile `test-q1` reconciliation, verified with unit tests; or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and record a failure report with resume point.

Check live code and recent history first before implementing (`internal/claude/init_quota.go`, `internal/claude/init.go`, and `internal/claude/quota1_init_test.go`).
