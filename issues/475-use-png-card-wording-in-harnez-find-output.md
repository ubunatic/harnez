# 475 — Use PNG card wording in harnez find output

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation

---

## 1. Problem & Motivation

`harnez find` currently reports rendered visual issue results with the message `🖼️ Rendered`. The project’s newer terminology calls these visuals “PNG cards,” so the output is inconsistent with current user-facing wording.

## 2. Goal

Update `harnez find` output to use the “PNG card” wording wherever it announces a generated visual card, with focused coverage or documentation updates as appropriate.

## 3. Implementation & Verification Plan

- Locate the output string and replace the legacy “Rendered” wording with clear “PNG card” terminology.
- Update affected tests or snapshots.
- Verify `harnez find` output and the relevant test suite.
