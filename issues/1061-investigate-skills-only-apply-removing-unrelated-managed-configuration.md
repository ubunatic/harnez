# 1061 — Investigate skills-only apply removing unrelated managed configuration

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: 491, 1052

---

## Problem

During Pi background-task work, `harnez apply --components skills` reportedly listed managed Codex/Gemini hook configuration and two distill extensions as removed, although the intended operation was limited to skills. It is not yet established whether actual user configuration was lost, obsolete managed entries were correctly reconciled, or the report was misleading. The owner expects `harnez apply` to be safe.

## Goal

/goal Investigate and reproduce the reported behavior in an isolated environment; determine and document the intended component-selection and ownership semantics, fix any unsafe or misleading behavior, and verify unrelated configuration is preserved. Stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, clear the goal and write a failure report with the resume point.

Recheck live code, current configuration and recent history before starting. Compare planned and actual filesystem and configuration changes, including managed versus user-authored entries and idempotent re-apply. Do not run destructive experiments against the user's live configuration. Add regression tests for the verified failure or, if behavior is intended, document evidence and why it is safe.
