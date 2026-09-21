# 474 — Add manpage support to init --docs

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `harnez init --docs man` currently rejects `man`; `manpages` is an available documentation name.

---

## 1. Problem & Motivation

`harnez init --docs man` fails with `unknown doc(s) man`, even though projects may need the man-page guidance installed alongside their other language and workflow docs. This makes the intuitive `man` name unusable and leaves the requested documentation setup incomplete.

## 2. Goal

`harnez init --docs man` should successfully install the man-page documentation (or provide a documented, consistent alias to the existing `manpages` doc), with the behavior covered by tests and reflected in the CLI help or documentation.

## 3. Implementation & Verification Plan

- Trace the `init --docs` registry and determine whether `man` should alias `manpages` or become the canonical name.
- Add/update focused tests for the accepted name and installed content while preserving `manpages` compatibility.
- Verify the command and relevant test suite, then update user-facing help/docs as needed.
