# 1137 — usage: non-blocking power peak file lock and tests for corrupt files and multi-process contention

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Performance
**Related**: 1121, PR #103

---

## 1. Problem & Motivation
Follow-up to issue 1121 (Power box implementation):
1. **Redraw blocking on peak lock**: `powerPeakStore.Observe` currently calls `saveLocked` synchronously within the frame draw path when the throttle interval has elapsed. `saveLocked` takes an exclusive file lock (`syscall.Flock(..., syscall.LOCK_EX)`). If another watcher holds the lock, or disk I/O stalls, `syscall.Flock` blocks the mutex `s.mu` and blocks the UI redraw loop.
2. **Missing corrupt-file resilience test**: While `readPowerPeakFile` returns `ok=false` on unmarshal failure, explicit regression test coverage is needed to verify that a corrupt, partial, or malformed JSON file does not crash the watcher and is safely overwritten/recovered.
3. **Missing multi-process test**: Existing concurrent tests run in-process goroutines (`go func()`). A test with multiple external OS processes updating peaks concurrently is needed to ensure `flock` and atomic rename behave correctly under real OS multi-process contention.

## 2. Technical Specification & Findings
- **Non-blocking lock / async write** (`internal/usage/power_peaks.go`):
  - In `saveLocked`, use a non-blocking lock (`syscall.LOCK_EX|syscall.LOCK_NB`) or decouple file flushing from the synchronous `Observe` draw call (e.g. background save goroutine / deferred write).
  - If the lock is held by another process, skip the immediate save and retry on the next interval without blocking the UI.
- **Corrupt-file handling**:
  - Test that reading a file with truncated JSON or random bytes handles gracefully, resets the store to empty, and overwrites the corrupt file cleanly on next flush.
- **Multi-process contention test**:
  - Add a test or helper verifying multiple OS subprocesses writing to the same `power-peaks.json` without file corruption or deadlock.

## 3. Implementation & Verification Plan
- **M1**: Refactor `saveLocked` to use non-blocking lock or asynchronous/non-blocking persist semantics so `Observe` never stalls the watch render loop.
- **M2**: Add unit tests in `internal/usage/power_peaks_test.go` for corrupt/truncated JSON recovery.
- **M3**: Add multi-process contention test verifying flock behavior across separate OS processes.
