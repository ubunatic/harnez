# 607 — Adopt Loom view for usage --compact --watch behind a flag (strangler, from loom 103)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: loom issue 103 (`examples/usage` in ../loom)

---

## 1. Problem & Motivation
Loom 103 rebuilt `usage --compact --watch` (All Usage and Load boxes) as a Loom example, with
a plain view and a Loom view that share one data model and can be switched by a flag and a key.
The next step of the strangler migration is to use this inside harnez itself.

## 2. Technical Specification / Findings
- The loom example is simplified: one quota window per agent, and no temperature, GPU/VRAM or
  Braille trend. Those need Loom-side support or a harnez-side widget.
- Keep the existing renderer as the default until the Loom view reaches parity.

## 3. Implementation & Verification Plan
Add a `--loom` (or similar) flag to `usage --compact --watch` that renders through Loom
widgets fed by harnez's real collectors, plus an interactive switch between the two views. Verify
with an ANSI snapshot comparison of both views and a user visual check.
