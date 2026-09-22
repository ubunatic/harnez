# 502 — Strengthen AGENTS.md documentation and constant-uniqueness guidance

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `AGENTS.md`; proposal from the `lmcoder` project

---

## Problem & Motivation

Recent work introduced a new runtime/service mode without an Evergreen document, and low-tier agents left no documentation trail after changing a subsystem. The repository also lacks a clear convention or check preventing the same numeric service constant from being defined in multiple files.

## Goal

Make repository guidance reliably capture subsystem documentation and numeric-constant ownership: update `AGENTS.md` with a rule that new runtimes/services require a relevant Evergreen doc before issue closure, add a low-tier-agent reminder to grep `docs/` and record missing coverage, and decide on a maintainable convention or lint check for duplicate numeric constants.

## Implementation & Verification Plan

- Update the applicable `AGENTS.md` guidance and low-tier agent brief.
- Assess whether a focused Go analyzer or grep-based `make lint` check is appropriate; document the chosen rule and exceptions.
- Verify the guidance/check against the `lmcoder` lifecycle-mode gap and existing numeric constants without introducing false positives.
