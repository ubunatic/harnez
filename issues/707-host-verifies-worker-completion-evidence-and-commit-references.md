# 707 — Host verifies worker completion evidence and commit references

**Status**: Closed — implemented in f6a8502c; verification rule committed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
Hosts can accept a worker's "done" report without checking that the claimed output exists. A worker closing tickets must also make its implementation commit traceable, preventing zero-change work from being presented as complete.

## 2. Technical Specification / Findings
Before accepting completion, the host checks `git show --stat` and the built output. A worker that closes tickets cites the commit that changed non-issue files. Keep this rule in the managed source `docs/practices/AgenticLoop.md`; sync its generated root copy.

## 3. Implementation & Verification Plan
Document the host verification and worker commit-citation requirements in the managed practice source. Verify the generated root copy matches after sync.
