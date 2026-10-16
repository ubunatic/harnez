# 792 — Usage statusline observation goroutine outlives test TempDir (cleanup: directory not empty)

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation
TestStatuslineObservationLargeStoreCompletesWithinBudget, TestPassiveObservationReturnsQuicklyWhenDatabaseLocked and cmd/harnez TestAGYStatuslineUsesPayloadInsteadOfStoredQuota fail with "TempDir RemoveAll cleanup: directory not empty".

## 2. Technical Specification / Findings
`usage.ObserveStatuslineAt` returns immediately by design and does the SQLite write in a fire-and-forget goroutine. The test returns first, and that goroutine keeps creating/removing the WAL/shm/journal files inside t.TempDir while RemoveAll runs. Production code is correct (the fast return is the contract); nothing could join the goroutine.

## 3. Implementation & Verification Plan
Add a package-level WaitGroup plus `usage.WaitObservations()` that joins in-flight writes (each is bounded by its own budget). Tests register `t.Cleanup(WaitObservations)` right after TempDir (LIFO runs it before removal). Verified with -count=5 for all three tests.
