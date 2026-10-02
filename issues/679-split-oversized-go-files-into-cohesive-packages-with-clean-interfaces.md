# 679 — Split oversized Go files into cohesive packages with clean interfaces

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Architecture
**Related**: [[160-watch-viewer-server-split-feasibility]], [[678-make-usage-view-modes-spec-driven-and-independent-of-watch]]

---

## 1. Problem & Motivation
Several Go source files have grown beyond 1,000 lines, making responsibilities and call boundaries difficult to follow. Split oversized files into cohesive packages with clean interfaces between callers and implementations.

## 2. Goal & Acceptance Criteria
Inventory Go source files over 1,000 lines, split them along cohesive responsibility boundaries, and expose narrow interfaces for cross-package calls. Preserve existing behavior and keep the affected tests passing.

## 3. Implementation & Verification Plan
Refactor incrementally, verifying each package boundary and affected behavior with the repository test suite.

**Goal**: Split oversized Go files into cohesive packages with clean interfaces and verify behavior, or stop and report if blocked on a user decision or denied permission.
