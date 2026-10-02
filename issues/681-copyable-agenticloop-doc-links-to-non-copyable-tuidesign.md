# 681 — Copyable AgenticLoop doc links to non-copyable TUIDesign

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Documentation
**Related**: `docs/TUIDesign.md`

---

## 1. Problem & Motivation
The copyable `docs/practices/AgenticLoop.md` and its lite variant link to `TUIDesign.md`. `harnez init` installs the AgenticLoop guidance into consumer projects, but `TUIDesign.md` is a Harnez-only evergreen doc and is not copied. The resulting link in the consumer project points to a missing file. This was observed after running `harnez init` in `loom-games`.

## 2. Technical Specification / Findings
The full source link is in `docs/practices/AgenticLoop.md` under the TUI layout guidance; the lite copy contains the same link. The root `docs/AgenticLoop.md` copy is generated from the source. `docs/TUIDesign.md` exists in Harnez but is outside the copyable docs set.

## 3. Implementation & Verification Plan
Make the copyable guidance self-contained or point it only to documentation that `harnez init` also installs. Sync the generated root copy. Verify the link resolves in both full and lite output in a temporary consumer project after `harnez init`.

**Goal**: Make TUI layout guidance copied by `harnez init` free of links to files absent from consumer projects, or stop and report if blocked on a user decision or denied permission.
