# 588 — issues: status verbs reject plain Status: lines

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
Status-changing `harnez issues` verbs fail on tickets whose header uses the plain
`Status: Open` form, even though issue discovery and indexing recognize it.
This prevents lifecycle operations on valid existing tickets.

## 2. Technical Specification / Findings
The status rewrite path currently searches only for `**Status**:`. It must accept
both plain and bold metadata forms and preserve whichever form is present.

## 3. Implementation & Verification Plan
- Reuse the issue metadata parsing used by find/index where practical.
- Cover status rewrites for plain and bold headers, including preservation of the
  original marker.
- Run the single authorized verification: `make test-q1`.
