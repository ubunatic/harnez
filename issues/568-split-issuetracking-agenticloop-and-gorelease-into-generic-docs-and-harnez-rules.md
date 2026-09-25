# 568 — Split IssueTracking, AgenticLoop and GoRelease into generic docs and harnez rules

**Status**: Open
**Priority**: P2
**Severity**: Low
**Category**: Docs / Agent Instructions
**Related**: [[567-move-harnez-only-rules-into-harnez-rules-with-a-tool-neutral-agents-md]]

---

## Problem

After 567, three copyable docs still mix generic practice with harnez commands:
`docs/practices/IssueTracking.md`, `docs/practices/AgenticLoop.md`, `docs/lang/GoRelease.md`.

## Goal

Split each into a tool-neutral doc in `docs/` and a harnez rule in `.harnez/rules/`
(e.g. `Issues.md`, `Loop.md`, `Release.md`), using 567's sorting rule. Depends on 567.
