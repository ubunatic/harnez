# 709 — Create missing docs studies index section during indexing

**Status**: Open — reopened: second run fails in ubunatic.com and harnez.org: could not find end of studies table (expected a trailing blank line) when the studies table ends the file
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
`harnez index` fails when a repository's `docs/README.md` lacks the studies section anchor, and it also cannot render an index when `docs/studies/` is absent.

## 2. Technical Specification / Findings
`internal/index.UpdateDocsReadme` currently requires the anchor and `StudiesTable` requires the folder to exist. Reproduce with a temporary fixture before changing behavior. A missing section should be appended with the generated index; create the missing folder if needed.

## 3. Implementation & Verification Plan
Test missing-anchor and missing-folder fixtures, then run `harnez index -d .` in `ubunatic.com`, `harnez.org`, and `cati`. Commit each resulting external docs index separately.
